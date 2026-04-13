package tools

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type pptxPresentationXML struct {
	SlideIDs []pptxSlideIDXML `xml:"sldIdLst>sldId"`
}

type pptxSlideIDXML struct {
	ID    int    `xml:"id,attr"`
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

type pptxRelationshipsXML struct {
	XMLName       xml.Name              `xml:"Relationships"`
	XMLNS         string                `xml:"xmlns,attr,omitempty"`
	Relationships []pptxRelationshipXML `xml:"Relationship"`
}

type pptxRelationshipXML struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr,omitempty"`
	Target string `xml:"Target,attr"`
}

type pptxTemplateSlideChart struct {
	ChartIndex   int
	ChartXML     []byte
	ChartRelsXML []byte
	WorkbookXML  []byte
}

type pptxTemplateSlide struct {
	XML    []byte
	Rels   []byte
	Charts []pptxTemplateSlideChart
}

type pptxTemplateState struct {
	tempDir string
	slides  []pptxTemplateSlide
}

func (t *PPTXTool) executeTemplateMutation(ctx context.Context, args map[string]interface{}
// remapChartRelsWorkbook updates chart rels to point to the correct workbook
func remapChartRelsWorkbook(chartRels []byte, workbookIndex int) []byte {
	var rels pptxRelationshipsXML
	if err := xml.Unmarshal(chartRels, &rels); err != nil {
		return chartRels
	}
	
	for i := range rels.Relationships {
		if strings.Contains(rels.Relationships[i].Type, "package") && strings.Contains(rels.Relationships[i].Target, "embeddings") {
			// Update target to point to correct workbook
			rels.Relationships[i].Target = fmt.Sprintf("../embeddings/Microsoft_Excel_Worksheet%d.xlsx", workbookIndex)
		}
	}
	
	output, _ := xml.Marshal(rels)
	return output
}

, action string) (string, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return "", fmt.Errorf("path must be a non-empty string")
		}
	}
	absPath, relPath, _, err := t.scope.resolvePathWithContext(ctx, "pptx", path, false)
	if err != nil {
		return "", err
	}
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return "", err
	}

	summary, err := mutatePPTXTemplate(absPath, func(state *pptxTemplateState) error {
		switch action {
		case "duplicate_slide":
			return state.duplicateSlide(compatInt(args, "slide", "index"))
		case "delete_slide":
			return state.deleteSlide(compatInt(args, "slide", "index"))
		case "reorder_slides":
			rawOrder, ok := compatArgValue(args, "order")
			if !ok {
				return fmt.Errorf("order is required")
			}
			order, err := parsePPTXOrder(rawOrder, len(state.slides))
			if err != nil {
				return err
			}
			return state.reorderSlides(order)
		case "replace_text":
			replacements := parseReplacementMap(args, "replacements", "variables")
			if len(replacements) == 0 {
				return fmt.Errorf("replace_text requires replacements or variables")
			}
			return state.replaceText(replacements)
		default:
			return fmt.Errorf("unsupported pptx template action %q", action)
		}
	})
	if err != nil {
		return "", err
	}
	validation, err := t.validatePath(ctx, absPath)
	if err != nil {
		return "", err
	}
	info, _ := os.Stat(absPath)
	payload := nativeDocumentPayload{
		Action:       action,
		Path:         relPath,
		AbsolutePath: absPath,
		OriginalPath: path,
		Format:       "pptx",
		Engine:       "native_pptx_ooxml",
		EngineChain:  []string{"native_pptx_ooxml"},
		Degraded:     false,
		Validation:   validation,
		Size:         info.Size(),
		Summary:      summary,
		Success:      true,
	}
	return marshalNativeDocumentPayload(payload)
}

func mutatePPTXTemplate(path string, mutator func(*pptxTemplateState) error) (string, error) {
	entries, err := readZipArchive(path)
	if err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp("", "zimaos-blue-pptx-*")
	if err != nil {
		return "", fmt.Errorf("create pptx temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	for _, entry := range entries {
		target := filepath.Join(tempDir, filepath.FromSlash(entry.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", fmt.Errorf("create temp entry dir: %w", err)
		}
		if err := os.WriteFile(target, entry.Data, 0o644); err != nil {
			return "", fmt.Errorf("write temp entry %s: %w", entry.Name, err)
		}
	}

	state, err := loadPPTXTemplateState(tempDir)
	if err != nil {
		return "", err
	}
	if err := mutator(state); err != nil {
		return "", err
	}
	if err := state.save(); err != nil {
		return "", err
	}

	tmpOutput := path + ".tmp"
	if err := writeZipFromDir(tmpOutput, tempDir); err != nil {
		return "", err
	}
	if err := os.Rename(tmpOutput, path); err != nil {
		return "", fmt.Errorf("replace pptx archive: %w", err)
	}
	return fmt.Sprintf("Updated presentation template with %d slides", len(state.slides)), nil
}

func loadPPTXTemplateState(tempDir string) (*pptxTemplateState, error) {
	presentationXML, err := os.ReadFile(filepath.Join(tempDir, "ppt", "presentation.xml"))
	if err != nil {
		return nil, fmt.Errorf("read presentation.xml: %w", err)
	}
	presentationRelsXML, err := os.ReadFile(filepath.Join(tempDir, "ppt", "_rels", "presentation.xml.rels"))
	if err != nil {
		return nil, fmt.Errorf("read presentation.xml.rels: %w", err)
	}

	slideRelIDs, err := parsePPTXSlideRelIDs(presentationXML)
	if err != nil {
		return nil, fmt.Errorf("decode presentation.xml: %w", err)
	}
	var rels pptxRelationshipsXML
	if err := xml.Unmarshal(presentationRelsXML, &rels); err != nil {
		return nil, fmt.Errorf("decode presentation.xml.rels: %w", err)
	}

	relTargets := make(map[string]string, len(rels.Relationships))
	for _, rel := range rels.Relationships {
		relTargets[rel.ID] = strings.TrimSpace(rel.Target)
	}

	slides := make([]pptxTemplateSlide, 0, len(slideRelIDs))
	for _, relID := range slideRelIDs {
		target := relTargets[relID]
		if target == "" {
			continue
		}
		slidePath := filepath.Join(tempDir, "ppt", filepath.FromSlash(target))
		slideXML, err := os.ReadFile(slidePath)
		if err != nil {
			return nil, fmt.Errorf("read slide %s: %w", target, err)
		}
		relsPath := filepath.Join(tempDir, "ppt", "slides", "_rels", filepath.Base(target)+".rels")
		relsXML := []byte(officePPTXSlideRelsXML(0))
		if data, err := os.ReadFile(relsPath); err == nil {
			relsXML = data
		charts, err := loadPPTXSlideCharts(tempDir, relsXML)
		if err != nil {
			return nil, fmt.Errorf("load slide charts: %w", err)
		}
		slides = append(slides, pptxTemplateSlide{
			XML:    slideXML,
			Rels:   relsXML,
			Charts: charts,
		})
		})
	}

	return &pptxTemplateState{tempDir: tempDir, slides: slides}, nil
}

// loadPPTXSlideCharts extracts chart information from slide relationships
func loadPPTXSlideCharts(tempDir string, slideRels []byte) ([]pptxTemplateSlideChart, error) {
	var rels pptxRelationshipsXML
	if err := xml.Unmarshal(slideRels, &rels); err != nil {
		return nil, err
	}
	
	var charts []pptxTemplateSlideChart
	for _, rel := range rels.Relationships {
		// Check if this is a chart relationship
		if !strings.Contains(rel.Type, "chart") {
			continue
		}
		
		target := strings.TrimSpace(rel.Target)
		if target == "" {
			continue
		}
		
		// Extract chart index from target like "../charts/chart1.xml"
		var chartIndex int
		if _, err := fmt.Sscanf(filepath.Base(target), "chart%d.xml", &chartIndex); err != nil {
			continue
		}
		
		// Load chart XML
		chartPath := filepath.Join(tempDir, "ppt", filepath.FromSlash(target))
		chartXML, err := os.ReadFile(chartPath)
		if err != nil {
			return nil, fmt.Errorf("read chart %s: %w", target, err)
		}
		
		// Load chart rels
		chartRelsPath := filepath.Join(tempDir, "ppt", "charts", "_rels", filepath.Base(target)+".rels")
		var chartRelsXML []byte
		if data, err := os.ReadFile(chartRelsPath); err == nil {
			chartRelsXML = data
		}
		
		// Load embedded workbook if any
		var workbookXML []byte
		if len(chartRelsXML) > 0 {
			var chartRels pptxRelationshipsXML
			if err := xml.Unmarshal(chartRelsXML, &chartRels); err == nil {
				for _, chartRel := range chartRels.Relationships {
					if strings.Contains(chartRel.Type, "package") && strings.Contains(chartRel.Target, "embeddings") {
						workbookPath := filepath.Join(tempDir, "ppt", filepath.FromSlash(chartRel.Target))
						if data, err := os.ReadFile(workbookPath); err == nil {
							workbookXML = data
						}
						break
					}
				}
			}
		}
		
		charts = append(charts, pptxTemplateSlideChart{
			ChartIndex:   chartIndex,
			ChartXML:     chartXML,
			ChartRelsXML: chartRelsXML,
			WorkbookXML:  workbookXML,
		})
	}
	
	return charts, nil
}

func parsePPTXSlideRelIDs(data []byte) ([]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	relIDs := make([]string, 0, 8)
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "sldId" {
			continue
		}
		for _, attr := range start.Attr {
			if strings.HasPrefix(strings.TrimSpace(attr.Value), "rId") {
				relIDs = append(relIDs, strings.TrimSpace(attr.Value))
				break
			}
		}
	}
	return relIDs, nil
}

func (s *pptxTemplateState) duplicateSlide(index int) error {
	if index <= 0 || index > len(s.slides) {
		return fmt.Errorf("slide must be between 1 and %d", len(s.slides))
	}
	source := s.slides[index-1]
	
	// Deep copy charts
	duplicatedCharts := make([]pptxTemplateSlideChart, len(source.Charts))
	for i, chart := range source.Charts {
		duplicatedCharts[i] = pptxTemplateSlideChart{
			ChartIndex:   chart.ChartIndex,
			ChartXML:     append([]byte(nil), chart.ChartXML...),
			ChartRelsXML: append([]byte(nil), chart.ChartRelsXML...),
			WorkbookXML:  append([]byte(nil), chart.WorkbookXML...),
		}
	}
	
	duplicate := pptxTemplateSlide{
		XML:    append([]byte(nil), source.XML...),
		Rels:   append([]byte(nil), source.Rels...),
		Charts: duplicatedCharts,
	}
	s.slides = append(s.slides[:index], append([]pptxTemplateSlide{duplicate}, s.slides[index:]...)...)
	return nil
}

func (s *pptxTemplateState) deleteSlide(index int) error {
	if index <= 0 || index > len(s.slides) {
		return fmt.Errorf("slide must be between 1 and %d", len(s.slides))
	}
	s.slides = append(s.slides[:index-1], s.slides[index:]...)
	return nil
}

func parsePPTXOrder(raw interface{}, count int) ([]int, error) {
	items, ok := raw.([]interface{})
	if !ok || len(items) != count {
		return nil, fmt.Errorf("order must list exactly %d slides", count)
	}
	order := make([]int, 0, len(items))
	seen := make(map[int]struct{}, len(items))
	for _, item := range items {
		value := 0
		switch typed := item.(type) {
		case float64:
			value = int(typed)
		case int:
			value = typed
		default:
			return nil, fmt.Errorf("order values must be numeric")
		}
		if value <= 0 || value > count {
			return nil, fmt.Errorf("order value %d out of range", value)
		}
		if _, exists := seen[value]; exists {
			return nil, fmt.Errorf("order value %d duplicated", value)
		}
		seen[value] = struct{}{}
		order = append(order, value)
	}
	return order, nil
}

func (s *pptxTemplateState) reorderSlides(order []int) error {
	reordered := make([]pptxTemplateSlide, 0, len(order))
	for _, index := range order {
		reordered = append(reordered, s.slides[index-1])
	}
	s.slides = reordered
	return nil
}

func (s *pptxTemplateState) replaceText(replacements map[string]string) error {
	for idx := range s.slides {
		updated, err := replaceTextNodesInPPTXSlideXML(s.slides[idx].XML, replacements)
		if err != nil {
			return err
		}
		s.slides[idx].XML = updated
	}
	return nil
}

func replaceTextNodesInPPTXSlideXML(data []byte, replacements map[string]string) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	inTextNode := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode slide xml: %w", err)
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if typed.Name.Local == "t" {
				inTextNode++
			}
			if err := encoder.EncodeToken(typed); err != nil {
				return nil, fmt.Errorf("encode slide xml start token: %w", err)
			}
		case xml.EndElement:
			if typed.Name.Local == "t" && inTextNode > 0 {
				inTextNode--
			}
			if err := encoder.EncodeToken(typed); err != nil {
				return nil, fmt.Errorf("encode slide xml end token: %w", err)
			}
		case xml.CharData:
			if inTextNode > 0 {
				text := string(typed)
				for from, to := range replacements {
					text = strings.ReplaceAll(text, from, to)
				}
				typed = xml.CharData([]byte(text))
			}
			if err := encoder.EncodeToken(typed); err != nil {
				return nil, fmt.Errorf("encode slide xml char data: %w", err)
			}
		default:
			if err := encoder.EncodeToken(token); err != nil {
				return nil, fmt.Errorf("encode slide xml token: %w", err)
			}
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, fmt.Errorf("flush slide xml: %w", err)
	}
	return buf.Bytes(), nil
}

func (s *pptxTemplateState) save() error {
	slidesDir := filepath.Join(s.tempDir, "ppt", "slides")
	relsDir := filepath.Join(slidesDir, "_rels")
	if err := os.RemoveAll(slidesDir); err != nil {
		return fmt.Errorf("remove old slides dir: %w", err)
	}
	if err := os.MkdirAll(relsDir, 0o755); err != nil {
		return fmt.Errorf("create slides rels dir: %w", err)
	}

	titles := make([]officePPTXSlide, 0, len(s.slides))
	// First, collect all charts from all slides to determine next available chart index
	maxChartIndex := 0
	for _, slide := range s.slides {
		for _, chart := range slide.Charts {
			if chart.ChartIndex > maxChartIndex {
				maxChartIndex = chart.ChartIndex
			}
		}
	}

	for idx, slide := range s.slides {
		slideName := fmt.Sprintf("slide%d.xml", idx+1)
		slidePath := filepath.Join(slidesDir, slideName)
		if err := os.WriteFile(slidePath, slide.XML, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", slideName, err)
		}

		// Process slide charts - remap chart indices and write chart files
		updatedRels := slide.Rels
		for _, chart := range slide.Charts {
			maxChartIndex++
			newChartIndex := maxChartIndex

			// Write chart XML
			chartName := fmt.Sprintf("chart%d.xml", newChartIndex)
			chartDir := filepath.Join(s.tempDir, "ppt", "charts")
			chartPath := filepath.Join(chartDir, chartName)
			if err := os.MkdirAll(chartDir, 0o755); err != nil {
				return fmt.Errorf("create charts dir: %w", err)
			}
			if err := os.WriteFile(chartPath, chart.ChartXML, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", chartName, err)
			}

			// Write chart rels
			chartRelsDir := filepath.Join(chartDir, "_rels")
			if err := os.MkdirAll(chartRelsDir, 0o755); err != nil {
				return fmt.Errorf("create chart rels dir: %w", err)
			}
			chartRelsPath := filepath.Join(chartRelsDir, chartName+".rels")
			if len(chart.ChartRelsXML) > 0 {
				// Remap workbook reference in chart rels
				remappedRels := remapChartRelsWorkbook(chart.ChartRelsXML, newChartIndex)
				if err := os.WriteFile(chartRelsPath, remappedRels, 0o644); err != nil {
					return fmt.Errorf("write chart rels %s: %w", chartName, err)
				}

				// Write embedded workbook if present
				if len(chart.WorkbookXML) > 0 {
					workbookName := fmt.Sprintf("Microsoft_Excel_Worksheet%d.xlsx", newChartIndex)
					embeddingsDir := filepath.Join(s.tempDir, "ppt", "embeddings")
					if err := os.MkdirAll(embeddingsDir, 0o755); err != nil {
						return fmt.Errorf("create embeddings dir: %w", err)
					}
					workbookPath := filepath.Join(embeddingsDir, workbookName)
					if err := os.WriteFile(workbookPath, chart.WorkbookXML, 0o644); err != nil {
						return fmt.Errorf("write workbook %s: %w", workbookName, err)
					}
				}
			}

			// Update slide rels to point to new chart index
			oldPattern := fmt.Sprintf("chart%d.xml", chart.ChartIndex)
			newPattern := fmt.Sprintf("chart%d.xml", newChartIndex)
			updatedRels = []byte(strings.ReplaceAll(string(updatedRels), oldPattern, newPattern))
		}

		relsPath := filepath.Join(relsDir, slideName+".rels")
		if err := os.WriteFile(relsPath, updatedRels, 0o644); err != nil {
			return fmt.Errorf("write slide rels %s: %w", slideName, err)
		}
		titles = append(titles, officePPTXSlide{Title: pptxSlideTitle(slide.XML)})
	}

	if err := os.WriteFile(filepath.Join(s.tempDir, "ppt", "presentation.xml"), []byte(officePPTXPresentationXML(len(s.slides))), 0o644); err != nil {
		return fmt.Errorf("write presentation.xml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.tempDir, "ppt", "_rels", "presentation.xml.rels"), []byte(officePPTXPresentationRelsXML(len(s.slides))), 0o644); err != nil {
		return fmt.Errorf("write presentation.xml.rels: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.tempDir, "docProps", "app.xml"), []byte(officePPTXAppPropsXML(titles)), 0o644); err != nil {
		return fmt.Errorf("write app.xml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.tempDir, "[Content_Types].xml"), []byte(officePPTXContentTypesXML(len(s.slides), 0, 0)), 0o644); err != nil {
		return fmt.Errorf("write content types: %w", err)
	}
	if err := cleanupPPTXOrphanMedia(s.tempDir, len(s.slides)); err != nil {
		return err
	}
	return nil
}

func pptxSlideTitle(data []byte) string {
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	captureText := false
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if typed.Name.Local == "t" {
				captureText = true
			}
		case xml.EndElement:
			if typed.Name.Local == "t" {
				captureText = false
			}
		case xml.CharData:
			if captureText {
				text := strings.TrimSpace(string(typed))
				if text != "" {
					return text
				}
			}
		}
	}
	return "Slide"
}

func cleanupPPTXOrphanMedia(tempDir string, slideCount int) error {
	mediaDir := filepath.Join(tempDir, "ppt", "media")
	info, err := os.Stat(mediaDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	used := make(map[string]struct{})
	for idx := 1; idx <= slideCount; idx++ {
		relsPath := filepath.Join(tempDir, "ppt", "slides", "_rels", fmt.Sprintf("slide%d.xml.rels", idx))
		data, err := os.ReadFile(relsPath)
		if err != nil {
			continue
		}
		var rels pptxRelationshipsXML
		if err := xml.Unmarshal(data, &rels); err != nil {
			continue
		}
		for _, rel := range rels.Relationships {
			target := strings.TrimSpace(rel.Target)
			if !strings.Contains(target, "media/") {
				continue
			}
			used[path.Clean(path.Join("ppt/slides", target))] = struct{}{}
		}
	}
	entries, err := os.ReadDir(mediaDir)
	if err != nil {
		return fmt.Errorf("read media dir: %w", err)
	}
	for _, entry := range entries {
		relPath := path.Clean(path.Join("ppt/media", entry.Name()))
		if _, ok := used[relPath]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(mediaDir, entry.Name())); err != nil {
			return fmt.Errorf("remove orphan media %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func writeZipFromDir(outputPath, root string) error {
	files := make([]string, 0)
	if err := filepath.Walk(root, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		files = append(files, current)
		return nil
	}); err != nil {
		return fmt.Errorf("walk temp pptx dir: %w", err)
	}
	sort.Strings(files)

	entries := make([]zipArchiveEntry, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read temp pptx file %s: %w", file, err)
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return fmt.Errorf("resolve zip relative path: %w", err)
		}
		entries = append(entries, zipArchiveEntry{
			Name:   filepath.ToSlash(rel),
			Data:   data,
			Method: 8,
		})
	}
	return writeZipArchive(outputPath, entries)
}
