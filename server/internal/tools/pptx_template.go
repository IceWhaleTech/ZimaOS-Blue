package tools

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
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

type pptxTemplateChartSeriesDefault struct {
	Type                 string
	Axis                 string
	Labels               string
	LabelPosition        string
	LabelFormat          string
	LabelSeparator       string
	ShowValue            string
	ShowCategory         string
	ShowSeriesName       string
	ShowPercent          string
	ShowLegendKey        string
	ShowBubbleSize       string
	PointShowLabels      []string
	PointShowValues      []string
	PointShowCategories  []string
	PointShowSeriesNames []string
	PointShowPercents    []string
	PointLabelPositions  []string
	PointLabelFormats    []string
	PointLabelSeparators []string
	Color                string
	LineWidth            float64
	Dash                 string
	Marker               string
	PointColors          []string
	PointExplosions      []int
}

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

func (t *PPTXTool) executeTemplateMutation(ctx context.Context, args map[string]interface{}, action string) (string, error) {
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
		case "update_chart_data":
			rawChart, ok := compatArgValue(args, "chart", "chart_data", "chartData")
			if !ok {
				return fmt.Errorf("update_chart_data requires chart data")
			}
			chartIndex := compatInt(args, "chart_index", "chartIndex")
			if chartIndex <= 0 {
				chartIndex = 1
			}
			return state.updateChartData(compatInt(args, "slide", "index"), chartIndex, rawChart)
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
		}
		charts, err := loadPPTXSlideCharts(tempDir, relsXML)
		if err != nil {
			return nil, fmt.Errorf("load slide charts: %w", err)
		}
		slides = append(slides, pptxTemplateSlide{
			XML:    slideXML,
			Rels:   relsXML,
			Charts: charts,
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

		// Slide rel targets are resolved relative to ppt/slides/
		chartPath := filepath.Join(tempDir, filepath.FromSlash(path.Clean(path.Join("ppt/slides", target))))
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
						// Chart rel targets are resolved relative to ppt/charts/
						workbookPath := filepath.Join(tempDir, filepath.FromSlash(path.Clean(path.Join("ppt/charts", chartRel.Target))))
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

func (s *pptxTemplateState) updateChartData(slideIndex, chartIndex int, chartRaw interface{}) error {
	if slideIndex <= 0 || slideIndex > len(s.slides) {
		return fmt.Errorf("slide must be between 1 and %d", len(s.slides))
	}
	slide := &s.slides[slideIndex-1]
	if len(slide.Charts) == 0 {
		return fmt.Errorf("slide %d does not contain a native chart", slideIndex)
	}
	if chartIndex <= 0 || chartIndex > len(slide.Charts) {
		return fmt.Errorf("chart_index must be between 1 and %d", len(slide.Charts))
	}

	target := &slide.Charts[chartIndex-1]
	chartInput, err := inferPPTXTemplateChartDefaults(chartRaw, target.ChartXML)
	if err != nil {
		return err
	}
	chartSpec, err := parseOfficeChart(chartInput)
	if err != nil {
		return err
	}
	chartPackage, err := officeBuildPPTXChartPackage(*chartSpec, target.ChartIndex)
	if err != nil {
		return err
	}
	target.ChartXML = []byte(chartPackage.ChartXML)
	target.ChartRelsXML = []byte(chartPackage.ChartRelsXML)
	if chartPackage.Workbook != nil {
		target.WorkbookXML = append([]byte(nil), chartPackage.Workbook.Data...)
	} else {
		target.WorkbookXML = nil
	}
	return nil
}

func inferPPTXTemplateChartDefaults(rawChart interface{}, existingChartXML []byte) (map[string]interface{}, error) {
	chartMap, ok := coerceCompatMap(rawChart)
	if !ok {
		return nil, fmt.Errorf("chart must be an object")
	}
	merged := make(map[string]interface{}, len(chartMap)+2)
	for key, value := range chartMap {
		merged[key] = value
	}
	if !pptxTemplateChartHasAnyKey(merged, "type", "chart_type", "kind") {
		if inferredType := inferPPTXTemplateChartType(existingChartXML); inferredType != "" {
			merged["type"] = inferredType
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "title", "name") {
		if inferredTitle := inferPPTXTemplateChartTitle(existingChartXML); inferredTitle != "" {
			merged["title"] = inferredTitle
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "vary_colors", "varyColors") {
		if inferredVaryColors, ok := inferPPTXTemplateChartVaryColors(existingChartXML); ok {
			merged["vary_colors"] = inferredVaryColors
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "start_angle", "startAngle", "first_slice_angle", "firstSliceAngle") {
		if inferredStartAngle, ok := inferPPTXTemplateChartStartAngle(existingChartXML); ok {
			merged["start_angle"] = inferredStartAngle
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "hole_size", "holeSize", "donut_hole_size", "donutHoleSize") {
		if inferredHoleSize, ok := inferPPTXTemplateChartHoleSize(existingChartXML); ok {
			merged["hole_size"] = inferredHoleSize
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "smooth", "smoothed", "smooth_lines", "smoothLines", "line_smoothing", "lineSmoothing") {
		if inferredSmooth, ok := inferPPTXTemplateChartSmooth(existingChartXML); ok {
			merged["smooth"] = inferredSmooth
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "gap_width", "gapWidth") {
		if inferredGapWidth, ok := inferPPTXTemplateChartGapWidth(existingChartXML); ok {
			merged["gap_width"] = inferredGapWidth
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "overlap") {
		if inferredOverlap, ok := inferPPTXTemplateChartOverlap(existingChartXML); ok {
			merged["overlap"] = inferredOverlap
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_type", "categoryAxisType", "x_axis_type", "xAxisType") {
		if inferredAxisType := inferPPTXTemplateChartAxisType(existingChartXML); inferredAxisType != "" {
			merged["x_axis_type"] = inferredAxisType
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_format", "categoryAxisFormat", "x_axis_format", "xAxisFormat") {
		if inferredAxisFormat := inferPPTXTemplateChartAxisFormat(existingChartXML); inferredAxisFormat != "" {
			merged["x_axis_format"] = inferredAxisFormat
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_format", "secondaryCategoryAxisFormat", "secondary_x_axis_format", "secondaryXAxisFormat", "x2_axis_format", "x2AxisFormat") {
		if inferredSecondaryAxisFormat := inferPPTXTemplateChartSecondaryAxisFormat(existingChartXML); inferredSecondaryAxisFormat != "" {
			merged["secondary_x_axis_format"] = inferredSecondaryAxisFormat
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_min", "categoryAxisMin", "x_axis_min", "xAxisMin") {
		if inferredCategoryAxisMin, ok := inferPPTXTemplateChartDateAxisNumber(existingChartXML, 0, "<c:min"); ok {
			merged["x_axis_min"] = inferredCategoryAxisMin
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_max", "categoryAxisMax", "x_axis_max", "xAxisMax") {
		if inferredCategoryAxisMax, ok := inferPPTXTemplateChartDateAxisNumber(existingChartXML, 0, "<c:max"); ok {
			merged["x_axis_max"] = inferredCategoryAxisMax
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_base_time_unit", "categoryAxisBaseTimeUnit", "x_axis_base_time_unit", "xAxisBaseTimeUnit") {
		if inferredBaseTimeUnit := inferPPTXTemplateChartDateAxisTimeUnit(existingChartXML, 0, "<c:baseTimeUnit"); inferredBaseTimeUnit != "" {
			merged["x_axis_base_time_unit"] = inferredBaseTimeUnit
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_major_unit", "categoryAxisMajorUnit", "x_axis_major_unit", "xAxisMajorUnit") {
		if inferredCategoryAxisMajorUnit, ok := inferPPTXTemplateChartDateAxisNumber(existingChartXML, 0, "<c:majorUnit"); ok {
			merged["x_axis_major_unit"] = inferredCategoryAxisMajorUnit
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_minor_unit", "categoryAxisMinorUnit", "x_axis_minor_unit", "xAxisMinorUnit") {
		if inferredCategoryAxisMinorUnit, ok := inferPPTXTemplateChartDateAxisNumber(existingChartXML, 0, "<c:minorUnit"); ok {
			merged["x_axis_minor_unit"] = inferredCategoryAxisMinorUnit
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_major_time_unit", "categoryAxisMajorTimeUnit", "x_axis_major_time_unit", "xAxisMajorTimeUnit") {
		if inferredCategoryAxisMajorTimeUnit := inferPPTXTemplateChartDateAxisTimeUnit(existingChartXML, 0, "<c:majorTimeUnit"); inferredCategoryAxisMajorTimeUnit != "" {
			merged["x_axis_major_time_unit"] = inferredCategoryAxisMajorTimeUnit
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_minor_time_unit", "categoryAxisMinorTimeUnit", "x_axis_minor_time_unit", "xAxisMinorTimeUnit") {
		if inferredCategoryAxisMinorTimeUnit := inferPPTXTemplateChartDateAxisTimeUnit(existingChartXML, 0, "<c:minorTimeUnit"); inferredCategoryAxisMinorTimeUnit != "" {
			merged["x_axis_minor_time_unit"] = inferredCategoryAxisMinorTimeUnit
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_label_position", "categoryAxisLabelPosition", "x_axis_label_position", "xAxisLabelPosition") {
		if inferredCategoryAxisLabelPosition := inferPPTXTemplateChartCategoryAxisLabelPosition(existingChartXML, 0); inferredCategoryAxisLabelPosition != "" {
			merged["x_axis_label_position"] = inferredCategoryAxisLabelPosition
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_label_position", "secondaryCategoryAxisLabelPosition", "secondary_x_axis_label_position", "secondaryXAxisLabelPosition", "x2_axis_label_position", "x2AxisLabelPosition") {
		if inferredSecondaryCategoryAxisLabelPosition := inferPPTXTemplateChartCategoryAxisLabelPosition(existingChartXML, 1); inferredSecondaryCategoryAxisLabelPosition != "" {
			merged["x2_axis_label_position"] = inferredSecondaryCategoryAxisLabelPosition
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_reverse_order", "categoryAxisReverseOrder", "x_axis_reverse_order", "xAxisReverseOrder") {
		if inferredCategoryAxisReverseOrder := inferPPTXTemplateChartCategoryAxisReverseOrder(existingChartXML, 0); inferredCategoryAxisReverseOrder != "" {
			merged["x_axis_reverse_order"] = inferredCategoryAxisReverseOrder
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_reverse_order", "secondaryCategoryAxisReverseOrder", "secondary_x_axis_reverse_order", "secondaryXAxisReverseOrder", "x2_axis_reverse_order", "x2AxisReverseOrder") {
		if inferredSecondaryCategoryAxisReverseOrder := inferPPTXTemplateChartCategoryAxisReverseOrder(existingChartXML, 1); inferredSecondaryCategoryAxisReverseOrder != "" {
			merged["x2_axis_reverse_order"] = inferredSecondaryCategoryAxisReverseOrder
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_crosses", "categoryAxisCrosses", "x_axis_crosses", "xAxisCrosses") {
		if inferredCategoryAxisCrosses := inferPPTXTemplateChartCategoryAxisCrosses(existingChartXML, 0); inferredCategoryAxisCrosses != "" {
			merged["x_axis_crosses"] = inferredCategoryAxisCrosses
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_crosses", "secondaryCategoryAxisCrosses", "secondary_x_axis_crosses", "secondaryXAxisCrosses", "x2_axis_crosses", "x2AxisCrosses") {
		if inferredSecondaryCategoryAxisCrosses := inferPPTXTemplateChartCategoryAxisCrosses(existingChartXML, 1); inferredSecondaryCategoryAxisCrosses != "" {
			merged["x2_axis_crosses"] = inferredSecondaryCategoryAxisCrosses
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_major_tick_mark", "categoryAxisMajorTickMark", "x_axis_major_tick_mark", "xAxisMajorTickMark") {
		if inferredCategoryAxisMajorTickMark := inferPPTXTemplateChartCategoryAxisTickMark(existingChartXML, 0, "<c:majorTickMark"); inferredCategoryAxisMajorTickMark != "" {
			merged["x_axis_major_tick_mark"] = inferredCategoryAxisMajorTickMark
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_minor_tick_mark", "categoryAxisMinorTickMark", "x_axis_minor_tick_mark", "xAxisMinorTickMark") {
		if inferredCategoryAxisMinorTickMark := inferPPTXTemplateChartCategoryAxisTickMark(existingChartXML, 0, "<c:minorTickMark"); inferredCategoryAxisMinorTickMark != "" {
			merged["x_axis_minor_tick_mark"] = inferredCategoryAxisMinorTickMark
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_major_tick_mark", "secondaryCategoryAxisMajorTickMark", "secondary_x_axis_major_tick_mark", "secondaryXAxisMajorTickMark", "x2_axis_major_tick_mark", "x2AxisMajorTickMark") {
		if inferredSecondaryCategoryAxisMajorTickMark := inferPPTXTemplateChartCategoryAxisTickMark(existingChartXML, 1, "<c:majorTickMark"); inferredSecondaryCategoryAxisMajorTickMark != "" {
			merged["x2_axis_major_tick_mark"] = inferredSecondaryCategoryAxisMajorTickMark
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_minor_tick_mark", "secondaryCategoryAxisMinorTickMark", "secondary_x_axis_minor_tick_mark", "secondaryXAxisMinorTickMark", "x2_axis_minor_tick_mark", "x2AxisMinorTickMark") {
		if inferredSecondaryCategoryAxisMinorTickMark := inferPPTXTemplateChartCategoryAxisTickMark(existingChartXML, 1, "<c:minorTickMark"); inferredSecondaryCategoryAxisMinorTickMark != "" {
			merged["x2_axis_minor_tick_mark"] = inferredSecondaryCategoryAxisMinorTickMark
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_label_alignment", "categoryAxisLabelAlignment", "x_axis_label_alignment", "xAxisLabelAlignment") {
		if inferredCategoryAxisLabelAlignment := inferPPTXTemplateChartCategoryAxisLabelAlignment(existingChartXML, 0); inferredCategoryAxisLabelAlignment != "" {
			merged["x_axis_label_alignment"] = inferredCategoryAxisLabelAlignment
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_label_alignment", "secondaryCategoryAxisLabelAlignment", "secondary_x_axis_label_alignment", "secondaryXAxisLabelAlignment", "x2_axis_label_alignment", "x2AxisLabelAlignment") {
		if inferredSecondaryCategoryAxisLabelAlignment := inferPPTXTemplateChartCategoryAxisLabelAlignment(existingChartXML, 1); inferredSecondaryCategoryAxisLabelAlignment != "" {
			merged["x2_axis_label_alignment"] = inferredSecondaryCategoryAxisLabelAlignment
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_label_offset", "categoryAxisLabelOffset", "x_axis_label_offset", "xAxisLabelOffset") {
		if inferredCategoryAxisLabelOffset, ok := inferPPTXTemplateChartCategoryAxisLabelOffset(existingChartXML, 0); ok {
			merged["x_axis_label_offset"] = inferredCategoryAxisLabelOffset
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_label_offset", "secondaryCategoryAxisLabelOffset", "secondary_x_axis_label_offset", "secondaryXAxisLabelOffset", "x2_axis_label_offset", "x2AxisLabelOffset") {
		if inferredSecondaryCategoryAxisLabelOffset, ok := inferPPTXTemplateChartCategoryAxisLabelOffset(existingChartXML, 1); ok {
			merged["x2_axis_label_offset"] = inferredSecondaryCategoryAxisLabelOffset
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_multi_level_labels", "categoryAxisMultiLevelLabels", "category_axis_multilevel_labels", "categoryAxisMultilevelLabels", "x_axis_multi_level_labels", "xAxisMultiLevelLabels", "x_axis_multilevel_labels", "xAxisMultilevelLabels") {
		if inferredCategoryAxisMultiLevelLabels, ok := inferPPTXTemplateChartCategoryAxisMultiLevelLabels(existingChartXML, 0); ok {
			merged["x_axis_multi_level_labels"] = inferredCategoryAxisMultiLevelLabels
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_multi_level_labels", "secondaryCategoryAxisMultiLevelLabels", "secondary_category_axis_multilevel_labels", "secondaryCategoryAxisMultilevelLabels", "secondary_x_axis_multi_level_labels", "secondaryXAxisMultiLevelLabels", "secondary_x_axis_multilevel_labels", "secondaryXAxisMultilevelLabels", "x2_axis_multi_level_labels", "x2AxisMultiLevelLabels", "x2_axis_multilevel_labels", "x2AxisMultilevelLabels") {
		if inferredSecondaryCategoryAxisMultiLevelLabels, ok := inferPPTXTemplateChartCategoryAxisMultiLevelLabels(existingChartXML, 1); ok {
			merged["x2_axis_multi_level_labels"] = inferredSecondaryCategoryAxisMultiLevelLabels
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_visible", "categoryAxisVisible", "show_category_axis", "showCategoryAxis", "x_axis_visible", "xAxisVisible") {
		if inferredCategoryAxisVisible, ok := inferPPTXTemplateChartCategoryAxisVisible(existingChartXML, 0); ok {
			merged["x_axis_visible"] = inferredCategoryAxisVisible
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_visible", "secondaryCategoryAxisVisible", "show_secondary_category_axis", "showSecondaryCategoryAxis", "secondary_x_axis_visible", "secondaryXAxisVisible", "x2_axis_visible", "x2AxisVisible") {
		if inferredSecondaryCategoryAxisVisible, ok := inferPPTXTemplateChartCategoryAxisVisible(existingChartXML, 1); ok {
			merged["x2_axis_visible"] = inferredSecondaryCategoryAxisVisible
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_auto", "categoryAxisAuto", "x_axis_auto", "xAxisAuto") {
		if inferredCategoryAxisAuto, ok := inferPPTXTemplateChartCategoryAxisAuto(existingChartXML, 0); ok {
			merged["x_axis_auto"] = inferredCategoryAxisAuto
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_auto", "secondaryCategoryAxisAuto", "secondary_x_axis_auto", "secondaryXAxisAuto", "x2_axis_auto", "x2AxisAuto") {
		if inferredSecondaryCategoryAxisAuto, ok := inferPPTXTemplateChartCategoryAxisAuto(existingChartXML, 1); ok {
			merged["x2_axis_auto"] = inferredSecondaryCategoryAxisAuto
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_format", "valueAxisFormat", "y_axis_format", "yAxisFormat") {
		if inferredValueAxisFormat := inferPPTXTemplateChartValueAxisFormat(existingChartXML, "primary"); inferredValueAxisFormat != "" {
			merged["y_axis_format"] = inferredValueAxisFormat
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_format", "secondaryValueAxisFormat", "secondary_y_axis_format", "secondaryYAxisFormat", "y2_axis_format", "y2AxisFormat") {
		if inferredSecondaryValueAxisFormat := inferPPTXTemplateChartValueAxisFormat(existingChartXML, "secondary"); inferredSecondaryValueAxisFormat != "" {
			merged["y2_axis_format"] = inferredSecondaryValueAxisFormat
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_min", "valueAxisMin", "y_axis_min", "yAxisMin") {
		if inferredValueAxisMin, ok := inferPPTXTemplateChartValueAxisNumber(existingChartXML, "primary", "<c:min"); ok {
			merged["y_axis_min"] = inferredValueAxisMin
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_max", "valueAxisMax", "y_axis_max", "yAxisMax") {
		if inferredValueAxisMax, ok := inferPPTXTemplateChartValueAxisNumber(existingChartXML, "primary", "<c:max"); ok {
			merged["y_axis_max"] = inferredValueAxisMax
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_min", "secondaryValueAxisMin", "secondary_y_axis_min", "secondaryYAxisMin", "y2_axis_min", "y2AxisMin") {
		if inferredSecondaryValueAxisMin, ok := inferPPTXTemplateChartValueAxisNumber(existingChartXML, "secondary", "<c:min"); ok {
			merged["y2_axis_min"] = inferredSecondaryValueAxisMin
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_max", "secondaryValueAxisMax", "secondary_y_axis_max", "secondaryYAxisMax", "y2_axis_max", "y2AxisMax") {
		if inferredSecondaryValueAxisMax, ok := inferPPTXTemplateChartValueAxisNumber(existingChartXML, "secondary", "<c:max"); ok {
			merged["y2_axis_max"] = inferredSecondaryValueAxisMax
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_major_unit", "valueAxisMajorUnit", "y_axis_major_unit", "yAxisMajorUnit") {
		if inferredValueAxisMajorUnit, ok := inferPPTXTemplateChartValueAxisNumber(existingChartXML, "primary", "<c:majorUnit"); ok {
			merged["y_axis_major_unit"] = inferredValueAxisMajorUnit
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_minor_unit", "valueAxisMinorUnit", "y_axis_minor_unit", "yAxisMinorUnit") {
		if inferredValueAxisMinorUnit, ok := inferPPTXTemplateChartValueAxisNumber(existingChartXML, "primary", "<c:minorUnit"); ok {
			merged["y_axis_minor_unit"] = inferredValueAxisMinorUnit
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_major_unit", "secondaryValueAxisMajorUnit", "secondary_y_axis_major_unit", "secondaryYAxisMajorUnit", "y2_axis_major_unit", "y2AxisMajorUnit") {
		if inferredSecondaryValueAxisMajorUnit, ok := inferPPTXTemplateChartValueAxisNumber(existingChartXML, "secondary", "<c:majorUnit"); ok {
			merged["y2_axis_major_unit"] = inferredSecondaryValueAxisMajorUnit
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_minor_unit", "secondaryValueAxisMinorUnit", "secondary_y_axis_minor_unit", "secondaryYAxisMinorUnit", "y2_axis_minor_unit", "y2AxisMinorUnit") {
		if inferredSecondaryValueAxisMinorUnit, ok := inferPPTXTemplateChartValueAxisNumber(existingChartXML, "secondary", "<c:minorUnit"); ok {
			merged["y2_axis_minor_unit"] = inferredSecondaryValueAxisMinorUnit
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_label_position", "valueAxisLabelPosition", "y_axis_label_position", "yAxisLabelPosition") {
		if inferredValueAxisLabelPosition := inferPPTXTemplateChartValueAxisLabelPosition(existingChartXML, "primary"); inferredValueAxisLabelPosition != "" {
			merged["y_axis_label_position"] = inferredValueAxisLabelPosition
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_label_position", "secondaryValueAxisLabelPosition", "secondary_y_axis_label_position", "secondaryYAxisLabelPosition", "y2_axis_label_position", "y2AxisLabelPosition") {
		if inferredSecondaryValueAxisLabelPosition := inferPPTXTemplateChartValueAxisLabelPosition(existingChartXML, "secondary"); inferredSecondaryValueAxisLabelPosition != "" {
			merged["y2_axis_label_position"] = inferredSecondaryValueAxisLabelPosition
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_reverse_order", "valueAxisReverseOrder", "y_axis_reverse_order", "yAxisReverseOrder") {
		if inferredValueAxisReverseOrder := inferPPTXTemplateChartValueAxisReverseOrder(existingChartXML, "primary"); inferredValueAxisReverseOrder != "" {
			merged["y_axis_reverse_order"] = inferredValueAxisReverseOrder
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_reverse_order", "secondaryValueAxisReverseOrder", "secondary_y_axis_reverse_order", "secondaryYAxisReverseOrder", "y2_axis_reverse_order", "y2AxisReverseOrder") {
		if inferredSecondaryValueAxisReverseOrder := inferPPTXTemplateChartValueAxisReverseOrder(existingChartXML, "secondary"); inferredSecondaryValueAxisReverseOrder != "" {
			merged["y2_axis_reverse_order"] = inferredSecondaryValueAxisReverseOrder
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_major_gridlines", "valueAxisMajorGridlines", "y_axis_major_gridlines", "yAxisMajorGridlines") {
		if inferredValueAxisMajorGridlines, ok := inferPPTXTemplateChartValueAxisHasTag(existingChartXML, "primary", "<c:majorGridlines"); ok {
			merged["y_axis_major_gridlines"] = inferredValueAxisMajorGridlines
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_minor_gridlines", "valueAxisMinorGridlines", "y_axis_minor_gridlines", "yAxisMinorGridlines") {
		if inferredValueAxisMinorGridlines, ok := inferPPTXTemplateChartValueAxisHasTag(existingChartXML, "primary", "<c:minorGridlines"); ok {
			merged["y_axis_minor_gridlines"] = inferredValueAxisMinorGridlines
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_major_gridlines", "secondaryValueAxisMajorGridlines", "secondary_y_axis_major_gridlines", "secondaryYAxisMajorGridlines", "y2_axis_major_gridlines", "y2AxisMajorGridlines") {
		if inferredSecondaryValueAxisMajorGridlines, ok := inferPPTXTemplateChartValueAxisHasTag(existingChartXML, "secondary", "<c:majorGridlines"); ok {
			merged["y2_axis_major_gridlines"] = inferredSecondaryValueAxisMajorGridlines
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_minor_gridlines", "secondaryValueAxisMinorGridlines", "secondary_y_axis_minor_gridlines", "secondaryYAxisMinorGridlines", "y2_axis_minor_gridlines", "y2AxisMinorGridlines") {
		if inferredSecondaryValueAxisMinorGridlines, ok := inferPPTXTemplateChartValueAxisHasTag(existingChartXML, "secondary", "<c:minorGridlines"); ok {
			merged["y2_axis_minor_gridlines"] = inferredSecondaryValueAxisMinorGridlines
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_crosses", "valueAxisCrosses", "y_axis_crosses", "yAxisCrosses") {
		if inferredValueAxisCrosses := inferPPTXTemplateChartValueAxisCrosses(existingChartXML, "primary"); inferredValueAxisCrosses != "" {
			merged["y_axis_crosses"] = inferredValueAxisCrosses
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_crosses", "secondaryValueAxisCrosses", "secondary_y_axis_crosses", "secondaryYAxisCrosses", "y2_axis_crosses", "y2AxisCrosses") {
		if inferredSecondaryValueAxisCrosses := inferPPTXTemplateChartValueAxisCrosses(existingChartXML, "secondary"); inferredSecondaryValueAxisCrosses != "" {
			merged["y2_axis_crosses"] = inferredSecondaryValueAxisCrosses
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_cross_between", "valueAxisCrossBetween", "y_axis_cross_between", "yAxisCrossBetween") {
		if inferredValueAxisCrossBetween := inferPPTXTemplateChartValueAxisCrossBetween(existingChartXML, "primary"); inferredValueAxisCrossBetween != "" {
			merged["y_axis_cross_between"] = inferredValueAxisCrossBetween
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_cross_between", "secondaryValueAxisCrossBetween", "secondary_y_axis_cross_between", "secondaryYAxisCrossBetween", "y2_axis_cross_between", "y2AxisCrossBetween") {
		if inferredSecondaryValueAxisCrossBetween := inferPPTXTemplateChartValueAxisCrossBetween(existingChartXML, "secondary"); inferredSecondaryValueAxisCrossBetween != "" {
			merged["y2_axis_cross_between"] = inferredSecondaryValueAxisCrossBetween
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_major_tick_mark", "valueAxisMajorTickMark", "y_axis_major_tick_mark", "yAxisMajorTickMark") {
		if inferredValueAxisMajorTickMark := inferPPTXTemplateChartValueAxisTickMark(existingChartXML, "primary", "<c:majorTickMark"); inferredValueAxisMajorTickMark != "" {
			merged["y_axis_major_tick_mark"] = inferredValueAxisMajorTickMark
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_minor_tick_mark", "valueAxisMinorTickMark", "y_axis_minor_tick_mark", "yAxisMinorTickMark") {
		if inferredValueAxisMinorTickMark := inferPPTXTemplateChartValueAxisTickMark(existingChartXML, "primary", "<c:minorTickMark"); inferredValueAxisMinorTickMark != "" {
			merged["y_axis_minor_tick_mark"] = inferredValueAxisMinorTickMark
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_major_tick_mark", "secondaryValueAxisMajorTickMark", "secondary_y_axis_major_tick_mark", "secondaryYAxisMajorTickMark", "y2_axis_major_tick_mark", "y2AxisMajorTickMark") {
		if inferredSecondaryValueAxisMajorTickMark := inferPPTXTemplateChartValueAxisTickMark(existingChartXML, "secondary", "<c:majorTickMark"); inferredSecondaryValueAxisMajorTickMark != "" {
			merged["y2_axis_major_tick_mark"] = inferredSecondaryValueAxisMajorTickMark
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_minor_tick_mark", "secondaryValueAxisMinorTickMark", "secondary_y_axis_minor_tick_mark", "secondaryYAxisMinorTickMark", "y2_axis_minor_tick_mark", "y2AxisMinorTickMark") {
		if inferredSecondaryValueAxisMinorTickMark := inferPPTXTemplateChartValueAxisTickMark(existingChartXML, "secondary", "<c:minorTickMark"); inferredSecondaryValueAxisMinorTickMark != "" {
			merged["y2_axis_minor_tick_mark"] = inferredSecondaryValueAxisMinorTickMark
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "show_legend", "showLegend") {
		if inferredShowLegend, _, ok := inferPPTXTemplateChartLegend(existingChartXML); ok {
			merged["show_legend"] = inferredShowLegend
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "legend_position", "legendPosition", "legend_pos", "legendPos") {
		if _, inferredLegendPosition, ok := inferPPTXTemplateChartLegend(existingChartXML); ok && inferredLegendPosition != "" {
			merged["legend_position"] = inferredLegendPosition
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "labels", "data_labels", "show_labels", "showLabels") {
		if inferredLabels, ok := inferPPTXTemplateChartLabels(existingChartXML); ok {
			merged["labels"] = inferredLabels
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "label_position", "labelPosition", "labels_position", "data_label_position", "dataLabelPosition") {
		if inferredLabelPosition := inferPPTXTemplateChartLabelPosition(existingChartXML); inferredLabelPosition != "" {
			merged["label_position"] = inferredLabelPosition
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "label_format", "labelFormat", "labels_format", "data_label_format", "dataLabelFormat") {
		if inferredLabelFormat := inferPPTXTemplateChartLabelFormat(existingChartXML); inferredLabelFormat != "" {
			merged["label_format"] = inferredLabelFormat
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "label_separator", "labelSeparator", "data_label_separator", "dataLabelSeparator") {
		if inferredLabelSeparator := inferPPTXTemplateChartLabelSeparator(existingChartXML); inferredLabelSeparator != "" {
			merged["label_separator"] = inferredLabelSeparator
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "show_leader_lines", "showLeaderLines", "label_leader_lines", "labelLeaderLines") {
		if inferredShowLeaderLines, ok := inferPPTXTemplateChartLabelToggle(existingChartXML, "<c:showLeaderLines"); ok {
			merged["show_leader_lines"] = inferredShowLeaderLines
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "show_value", "showValue", "label_value", "labelValue") {
		if inferredShowValue, ok := inferPPTXTemplateChartLabelToggle(existingChartXML, "<c:showVal"); ok {
			merged["show_value"] = inferredShowValue
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "show_category", "showCategory", "label_category", "labelCategory") {
		if inferredShowCategory, ok := inferPPTXTemplateChartLabelToggle(existingChartXML, "<c:showCatName"); ok {
			merged["show_category"] = inferredShowCategory
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "show_series_name", "showSeriesName", "label_series_name", "labelSeriesName") {
		if inferredShowSeriesName, ok := inferPPTXTemplateChartLabelToggle(existingChartXML, "<c:showSerName"); ok {
			merged["show_series_name"] = inferredShowSeriesName
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "show_percent", "showPercent", "label_percent", "labelPercent") {
		if inferredShowPercent, ok := inferPPTXTemplateChartLabelToggle(existingChartXML, "<c:showPercent"); ok {
			merged["show_percent"] = inferredShowPercent
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "show_legend_key", "showLegendKey", "label_legend_key", "labelLegendKey") {
		if inferredShowLegendKey, ok := inferPPTXTemplateChartLabelToggle(existingChartXML, "<c:showLegendKey"); ok {
			merged["show_legend_key"] = inferredShowLegendKey
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "show_bubble_size", "showBubbleSize", "label_bubble_size", "labelBubbleSize") {
		if inferredShowBubbleSize, ok := inferPPTXTemplateChartLabelToggle(existingChartXML, "<c:showBubbleSize"); ok {
			merged["show_bubble_size"] = inferredShowBubbleSize
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "category_axis_title", "categoryAxisTitle", "x_axis_title", "xAxisTitle") {
		if inferredPrimaryCategoryTitle := inferPPTXTemplateChartCategoryAxisTitle(existingChartXML, 0); inferredPrimaryCategoryTitle != "" {
			merged["x_axis_title"] = inferredPrimaryCategoryTitle
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_category_axis_title", "secondaryCategoryAxisTitle", "secondary_x_axis_title", "secondaryXAxisTitle", "x2_axis_title", "x2AxisTitle") {
		if inferredSecondaryCategoryTitle := inferPPTXTemplateChartCategoryAxisTitle(existingChartXML, 1); inferredSecondaryCategoryTitle != "" {
			merged["x2_axis_title"] = inferredSecondaryCategoryTitle
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "value_axis_title", "valueAxisTitle", "y_axis_title", "yAxisTitle") {
		if inferredPrimaryValueTitle := inferPPTXTemplateChartValueAxisTitle(existingChartXML, "primary"); inferredPrimaryValueTitle != "" {
			merged["y_axis_title"] = inferredPrimaryValueTitle
		}
	}
	if !pptxTemplateChartHasAnyKey(merged, "secondary_value_axis_title", "secondaryValueAxisTitle", "secondary_y_axis_title", "secondaryYAxisTitle", "y2_axis_title", "y2AxisTitle") {
		if inferredSecondaryValueTitle := inferPPTXTemplateChartValueAxisTitle(existingChartXML, "secondary"); inferredSecondaryValueTitle != "" {
			merged["y2_axis_title"] = inferredSecondaryValueTitle
		}
	}
	inferPPTXTemplateSeriesStyleDefaults(merged, existingChartXML)
	if officeNormalizeChartType(anyToStringForLLM(firstMapValue(merged, "type", "chart_type", "kind"))) == "combo" {
		inferPPTXTemplateComboSeriesDefaults(merged, existingChartXML)
	}
	return merged, nil
}

func pptxTemplateChartHasAnyKey(chart map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if _, ok := chart[key]; ok {
			return true
		}
	}
	return false
}

func inferPPTXTemplateChartType(chartXML []byte) string {
	xmlText := string(chartXML)
	hasBar := strings.Contains(xmlText, "<c:barChart")
	hasLine := strings.Contains(xmlText, "<c:lineChart")
	switch {
	case hasBar && hasLine:
		return "combo"
	case strings.Contains(xmlText, "<c:doughnutChart"):
		return "donut"
	case strings.Contains(xmlText, "<c:pieChart"):
		return "pie"
	case hasLine:
		return "line"
	case hasBar:
		barBlock := pptxTemplateFirstChartBlock(xmlText, "<c:barChart", "</c:barChart>")
		barDir := pptxTemplateTagAttributeValue(barBlock, "<c:barDir", "val")
		grouping := pptxTemplateTagAttributeValue(barBlock, "<c:grouping", "val")
		switch grouping {
		case "percentStacked":
			if barDir == "bar" {
				return "percent_stacked_bar"
			}
			return "percent_stacked_column"
		case "stacked":
			if barDir == "bar" {
				return "stacked_bar"
			}
			return "stacked_column"
		default:
			return "bar"
		}
	default:
		return ""
	}
}

func inferPPTXTemplateChartAxisType(chartXML []byte) string {
	xmlText := string(chartXML)
	if !strings.Contains(xmlText, "<c:dateAx") {
		return ""
	}
	format := pptxTemplateDateAxisFormatCode(xmlText)
	lower := strings.ToLower(strings.TrimSpace(format))
	if strings.Contains(lower, "h") || strings.Contains(lower, ":") {
		return "datetime"
	}
	return "date"
}

func inferPPTXTemplateChartTitle(chartXML []byte) string {
	xmlText := string(chartXML)
	start := strings.Index(xmlText, "<c:chart>")
	if start < 0 {
		return ""
	}
	end := strings.Index(xmlText[start:], "<c:plotArea>")
	if end < 0 {
		return ""
	}
	return pptxTemplateTitleText(xmlText[start : start+end])
}

func inferPPTXTemplateChartVaryColors(chartXML []byte) (bool, bool) {
	value, ok := pptxTemplateUniformAttributeAcrossBlocks(pptxTemplateChartAppearanceBlocks(string(chartXML)), "<c:varyColors", "val")
	if !ok {
		return false, false
	}
	switch value {
	case "0":
		return false, true
	case "1":
		return true, true
	default:
		return false, false
	}
}

func inferPPTXTemplateChartStartAngle(chartXML []byte) (int, bool) {
	return pptxTemplateChartAppearanceInt(string(chartXML), "<c:firstSliceAng")
}

func inferPPTXTemplateChartHoleSize(chartXML []byte) (int, bool) {
	return pptxTemplateChartAppearanceInt(string(chartXML), "<c:holeSize")
}

func inferPPTXTemplateChartSmooth(chartXML []byte) (bool, bool) {
	lineBlocks := pptxTemplateChartBlocks(string(chartXML), "<c:lineChart>", "</c:lineChart>")
	if len(lineBlocks) == 0 {
		return false, false
	}
	smoothValue := ""
	for _, block := range lineBlocks {
		seriesBlocks := pptxTemplateChartBlocks(block, "<c:ser>", "</c:ser>")
		if len(seriesBlocks) == 0 {
			return false, false
		}
		for _, seriesBlock := range seriesBlocks {
			current := strings.TrimSpace(pptxTemplateTagAttributeValue(seriesBlock, "<c:smooth", "val"))
			if current == "" {
				return false, false
			}
			if smoothValue == "" {
				smoothValue = current
				continue
			}
			if current != smoothValue {
				return false, false
			}
		}
	}
	switch smoothValue {
	case "0":
		return false, true
	case "1":
		return true, true
	default:
		return false, false
	}
}

func inferPPTXTemplateChartGapWidth(chartXML []byte) (int, bool) {
	return pptxTemplateChartUniformIntAcrossBlocks(pptxTemplateChartBlocks(string(chartXML), "<c:barChart>", "</c:barChart>"), "<c:gapWidth")
}

func inferPPTXTemplateChartOverlap(chartXML []byte) (int, bool) {
	return pptxTemplateChartUniformIntAcrossBlocks(pptxTemplateChartBlocks(string(chartXML), "<c:barChart>", "</c:barChart>"), "<c:overlap")
}

func inferPPTXTemplateChartAxisFormat(chartXML []byte) string {
	return pptxTemplateDateAxisFormatCode(string(chartXML))
}

func inferPPTXTemplateChartSecondaryAxisFormat(chartXML []byte) string {
	return pptxTemplateDateAxisFormatCodeAt(string(chartXML), 1)
}

func inferPPTXTemplateChartValueAxisFormat(chartXML []byte, axisKind string) string {
	return pptxTemplateValueAxisFormatCode(string(chartXML), axisKind)
}

func inferPPTXTemplateChartValueAxisNumber(chartXML []byte, axisKind, tagStart string) (float64, bool) {
	return pptxTemplateValueAxisNumber(string(chartXML), axisKind, tagStart)
}

func inferPPTXTemplateChartValueAxisLabelPosition(chartXML []byte, axisKind string) string {
	return pptxTemplateValueAxisAttribute(string(chartXML), axisKind, "<c:tickLblPos", "val")
}

func inferPPTXTemplateChartValueAxisReverseOrder(chartXML []byte, axisKind string) string {
	return pptxTemplateValueAxisAttribute(string(chartXML), axisKind, "<c:orientation", "val")
}

func inferPPTXTemplateChartValueAxisCrosses(chartXML []byte, axisKind string) string {
	return pptxTemplateValueAxisAttribute(string(chartXML), axisKind, "<c:crosses", "val")
}

func inferPPTXTemplateChartValueAxisCrossBetween(chartXML []byte, axisKind string) string {
	return pptxTemplateValueAxisAttribute(string(chartXML), axisKind, "<c:crossBetween", "val")
}

func inferPPTXTemplateChartValueAxisHasTag(chartXML []byte, axisKind, tagStart string) (bool, bool) {
	block := pptxTemplateValueAxisBlock(string(chartXML), axisKind)
	if block == "" {
		return false, false
	}
	return strings.Contains(block, tagStart), true
}

func inferPPTXTemplateChartValueAxisTickMark(chartXML []byte, axisKind, tagStart string) string {
	return pptxTemplateValueAxisAttribute(string(chartXML), axisKind, tagStart, "val")
}

func inferPPTXTemplateChartDateAxisNumber(chartXML []byte, axisIndex int, tagStart string) (float64, bool) {
	return pptxTemplateDateAxisNumber(string(chartXML), axisIndex, tagStart)
}

func inferPPTXTemplateChartDateAxisTimeUnit(chartXML []byte, axisIndex int, tagStart string) string {
	return pptxTemplateDateAxisTimeUnit(string(chartXML), axisIndex, tagStart)
}

func inferPPTXTemplateChartCategoryAxisLabelPosition(chartXML []byte, axisIndex int) string {
	return pptxTemplateCategoryAxisAttribute(string(chartXML), axisIndex, "<c:tickLblPos", "val")
}

func inferPPTXTemplateChartCategoryAxisReverseOrder(chartXML []byte, axisIndex int) string {
	return pptxTemplateCategoryAxisAttribute(string(chartXML), axisIndex, "<c:orientation", "val")
}

func inferPPTXTemplateChartCategoryAxisCrosses(chartXML []byte, axisIndex int) string {
	return pptxTemplateCategoryAxisAttribute(string(chartXML), axisIndex, "<c:crosses", "val")
}

func inferPPTXTemplateChartCategoryAxisTickMark(chartXML []byte, axisIndex int, tagStart string) string {
	return pptxTemplateCategoryAxisAttribute(string(chartXML), axisIndex, tagStart, "val")
}

func inferPPTXTemplateChartCategoryAxisLabelAlignment(chartXML []byte, axisIndex int) string {
	return pptxTemplateCategoryAxisAttribute(string(chartXML), axisIndex, "<c:lblAlgn", "val")
}

func inferPPTXTemplateChartCategoryAxisLabelOffset(chartXML []byte, axisIndex int) (int, bool) {
	value := pptxTemplateCategoryAxisAttribute(string(chartXML), axisIndex, "<c:lblOffset", "val")
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func inferPPTXTemplateChartCategoryAxisMultiLevelLabels(chartXML []byte, axisIndex int) (bool, bool) {
	value := pptxTemplateCategoryAxisAttribute(string(chartXML), axisIndex, "<c:noMultiLvlLbl", "val")
	switch value {
	case "0":
		return true, true
	case "1":
		return false, true
	default:
		return false, false
	}
}

func inferPPTXTemplateChartCategoryAxisVisible(chartXML []byte, axisIndex int) (bool, bool) {
	value := pptxTemplateCategoryAxisAttribute(string(chartXML), axisIndex, "<c:delete", "val")
	switch value {
	case "0":
		return true, true
	case "1":
		return false, true
	default:
		return false, false
	}
}

func inferPPTXTemplateChartCategoryAxisAuto(chartXML []byte, axisIndex int) (bool, bool) {
	value := pptxTemplateCategoryAxisAttribute(string(chartXML), axisIndex, "<c:auto", "val")
	switch value {
	case "0":
		return false, true
	case "1":
		return true, true
	default:
		return false, false
	}
}

func inferPPTXTemplateChartLegend(chartXML []byte) (bool, string, bool) {
	xmlText := string(chartXML)
	legendBlock := pptxTemplateFirstChartBlock(xmlText, "<c:legend>", "</c:legend>")
	if legendBlock != "" {
		return true, strings.TrimSpace(pptxTemplateTagAttributeValue(legendBlock, "<c:legendPos", "val")), true
	}
	if pptxTemplateChartWouldShowLegendByDefault(xmlText) {
		return false, "", true
	}
	return false, "", false
}

func inferPPTXTemplateChartLabels(chartXML []byte) (bool, bool) {
	if !pptxTemplateUniformDataLabelsHasChartDefaults(string(chartXML)) {
		return false, false
	}
	return true, true
}

func inferPPTXTemplateChartLabelPosition(chartXML []byte) string {
	return pptxTemplateChartDataLabelsDefaultAttribute(string(chartXML), "<c:dLblPos", "val")
}

func inferPPTXTemplateChartLabelFormat(chartXML []byte) string {
	return pptxTemplateChartDataLabelsDefaultAttribute(string(chartXML), "<c:numFmt", "formatCode")
}

func inferPPTXTemplateChartLabelSeparator(chartXML []byte) string {
	block := pptxTemplateUniformDataLabelsDefaultsBlock(string(chartXML))
	if block == "" {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(pptxTemplateTagValue(block, "<c:separator>", "</c:separator>")))
}

func inferPPTXTemplateChartLabelToggle(chartXML []byte, tagStart string) (bool, bool) {
	value := pptxTemplateChartDataLabelsDefaultAttribute(string(chartXML), tagStart, "val")
	switch value {
	case "0":
		return false, true
	case "1":
		return true, true
	default:
		return false, false
	}
}

func inferPPTXTemplateChartCategoryAxisTitle(chartXML []byte, axisIndex int) string {
	xmlText := string(chartXML)
	var axisBlocks []string
	if strings.Contains(xmlText, "<c:dateAx") {
		axisBlocks = pptxTemplateChartBlocks(xmlText, "<c:dateAx>", "</c:dateAx>")
	} else {
		axisBlocks = pptxTemplateChartBlocks(xmlText, "<c:catAx>", "</c:catAx>")
	}
	if axisIndex < 0 || axisIndex >= len(axisBlocks) {
		return ""
	}
	return pptxTemplateTitleText(axisBlocks[axisIndex])
}

func inferPPTXTemplateChartValueAxisTitle(chartXML []byte, axisKind string) string {
	xmlText := string(chartXML)
	for _, block := range pptxTemplateChartBlocks(xmlText, "<c:valAx>", "</c:valAx>") {
		currentAxisKind := "primary"
		if pptxTemplateTagAttributeValue(block, "<c:axPos", "val") == "r" {
			currentAxisKind = "secondary"
		}
		if currentAxisKind != axisKind {
			continue
		}
		return pptxTemplateTitleText(block)
	}
	return ""
}

func inferPPTXTemplateSeriesStyleDefaults(chart map[string]interface{}, chartXML []byte) {
	seriesItems, ok := chart["series"].([]interface{})
	if !ok || len(seriesItems) == 0 {
		return
	}
	defaults := inferPPTXTemplateExistingSeriesStyleDefaults(chartXML)
	if len(defaults) == 0 {
		return
	}
	for idx, item := range seriesItems {
		seriesMap, ok := coerceCompatMap(item)
		if !ok {
			continue
		}
		name := strings.TrimSpace(anyToStringForLLM(firstMapValue(seriesMap, "name", "label")))
		if name == "" {
			name = fmt.Sprintf("Series %d", idx+1)
		}
		inferred, ok := defaults[name]
		if !ok {
			continue
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "color", "series_color", "seriesColor") && inferred.Color != "" {
			seriesMap["color"] = inferred.Color
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "line_width", "lineWidth", "stroke_width", "strokeWidth") && inferred.LineWidth > 0 {
			seriesMap["line_width"] = inferred.LineWidth
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "dash", "dash_style", "dashStyle") && inferred.Dash != "" {
			seriesMap["dash"] = inferred.Dash
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "marker", "marker_style", "markerStyle") && inferred.Marker != "" {
			seriesMap["marker"] = inferred.Marker
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_colors", "pointColors", "slice_colors", "sliceColors") && len(inferred.PointColors) > 0 {
			seriesMap["point_colors"] = append([]string(nil), inferred.PointColors...)
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_explosions", "pointExplosions", "slice_explosions", "sliceExplosions") && len(inferred.PointExplosions) > 0 {
			seriesMap["point_explosions"] = append([]int(nil), inferred.PointExplosions...)
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_show_labels", "pointShowLabels", "slice_show_labels", "sliceShowLabels") && len(inferred.PointShowLabels) > 0 {
			seriesMap["point_show_labels"] = append([]string(nil), inferred.PointShowLabels...)
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_show_values", "pointShowValues", "slice_show_values", "sliceShowValues") && len(inferred.PointShowValues) > 0 {
			seriesMap["point_show_values"] = append([]string(nil), inferred.PointShowValues...)
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_show_categories", "pointShowCategories", "slice_show_categories", "sliceShowCategories") && len(inferred.PointShowCategories) > 0 {
			seriesMap["point_show_categories"] = append([]string(nil), inferred.PointShowCategories...)
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_show_series_names", "pointShowSeriesNames", "point_show_series_name", "pointShowSeriesName", "slice_show_series_names", "sliceShowSeriesNames", "slice_show_series_name", "sliceShowSeriesName") && len(inferred.PointShowSeriesNames) > 0 {
			seriesMap["point_show_series_names"] = append([]string(nil), inferred.PointShowSeriesNames...)
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_show_percents", "pointShowPercents", "point_show_percent", "pointShowPercent", "slice_show_percents", "sliceShowPercents", "slice_show_percent", "sliceShowPercent") && len(inferred.PointShowPercents) > 0 {
			seriesMap["point_show_percents"] = append([]string(nil), inferred.PointShowPercents...)
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_label_positions", "pointLabelPositions", "slice_label_positions", "sliceLabelPositions") && len(inferred.PointLabelPositions) > 0 {
			seriesMap["point_label_positions"] = append([]string(nil), inferred.PointLabelPositions...)
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_label_formats", "pointLabelFormats", "slice_label_formats", "sliceLabelFormats") && len(inferred.PointLabelFormats) > 0 {
			seriesMap["point_label_formats"] = append([]string(nil), inferred.PointLabelFormats...)
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "point_label_separators", "pointLabelSeparators", "slice_label_separators", "sliceLabelSeparators") && len(inferred.PointLabelSeparators) > 0 {
			seriesMap["point_label_separators"] = append([]string(nil), inferred.PointLabelSeparators...)
		}
		seriesItems[idx] = seriesMap
	}
	chart["series"] = seriesItems
}

func inferPPTXTemplateExistingSeriesStyleDefaults(chartXML []byte) map[string]pptxTemplateChartSeriesDefault {
	defaults := make(map[string]pptxTemplateChartSeriesDefault)
	seriesNames := make([]string, 0, 2)
	xmlText := string(chartXML)
	for idx, seriesBlock := range pptxTemplateChartBlocks(xmlText, "<c:ser>", "</c:ser>") {
		name := pptxTemplateSeriesName(seriesBlock)
		if name == "" {
			name = fmt.Sprintf("Series %d", idx+1)
		}
		defaults[name] = pptxTemplateSeriesStyleDefaults(seriesBlock)
		seriesNames = append(seriesNames, name)
	}
	if len(seriesNames) > 0 {
		if pointLabelDefaults := pptxTemplateCircularSeriesPointLabelDefaults(xmlText); pointLabelDefaults.hasPointLabelDefaults() {
			inferred := defaults[seriesNames[0]]
			inferred.PointShowLabels = pointLabelDefaults.PointShowLabels
			inferred.PointShowValues = pointLabelDefaults.PointShowValues
			inferred.PointShowCategories = pointLabelDefaults.PointShowCategories
			inferred.PointShowSeriesNames = pointLabelDefaults.PointShowSeriesNames
			inferred.PointShowPercents = pointLabelDefaults.PointShowPercents
			inferred.PointLabelPositions = pointLabelDefaults.PointLabelPositions
			inferred.PointLabelFormats = pointLabelDefaults.PointLabelFormats
			inferred.PointLabelSeparators = pointLabelDefaults.PointLabelSeparators
			defaults[seriesNames[0]] = inferred
		}
	}
	return defaults
}

func pptxTemplateSeriesStyleDefaults(seriesBlock string) pptxTemplateChartSeriesDefault {
	defaults := pptxTemplateChartSeriesDefault{}
	spPrBlock := pptxTemplateSeriesStyleBlock(seriesBlock)
	if spPrBlock != "" {
		defaults.Color = strings.TrimSpace(pptxTemplateTagAttributeValue(spPrBlock, "<a:srgbClr", "val"))
		if lineWidth, ok := pptxTemplateSeriesLineWidth(spPrBlock); ok {
			defaults.LineWidth = lineWidth
		}
		defaults.Dash = pptxTemplateSeriesDashValue(spPrBlock)
	}
	markerBlock := pptxTemplateFirstChartBlock(seriesBlock, "<c:marker>", "</c:marker>")
	if markerBlock != "" {
		defaults.Marker = strings.ToLower(strings.TrimSpace(pptxTemplateTagAttributeValue(markerBlock, "<c:symbol", "val")))
	}
	defaults.PointColors = pptxTemplateSeriesPointColors(seriesBlock)
	defaults.PointExplosions = pptxTemplateSeriesPointExplosions(seriesBlock)
	return defaults
}

func (d pptxTemplateChartSeriesDefault) hasPointLabelDefaults() bool {
	return len(d.PointShowLabels) > 0 ||
		len(d.PointShowValues) > 0 ||
		len(d.PointShowCategories) > 0 ||
		len(d.PointShowSeriesNames) > 0 ||
		len(d.PointShowPercents) > 0 ||
		len(d.PointLabelPositions) > 0 ||
		len(d.PointLabelFormats) > 0 ||
		len(d.PointLabelSeparators) > 0
}

func pptxTemplateCircularSeriesPointLabelDefaults(chartXML string) pptxTemplateChartSeriesDefault {
	switch inferPPTXTemplateChartType([]byte(chartXML)) {
	case "pie", "donut":
	default:
		return pptxTemplateChartSeriesDefault{}
	}
	chartBlock := pptxTemplateChartAppearanceBlock(chartXML)
	if chartBlock == "" {
		return pptxTemplateChartSeriesDefault{}
	}
	dLblsBlock := pptxTemplateFirstChartBlock(chartBlock, "<c:dLbls>", "</c:dLbls>")
	if dLblsBlock == "" {
		return pptxTemplateChartSeriesDefault{}
	}
	return pptxTemplateChartSeriesDefault{
		PointShowLabels:      pptxTemplateDataLabelPointValues(dLblsBlock, pptxTemplateDataLabelDeleteSetting),
		PointShowValues:      pptxTemplateDataLabelPointValues(dLblsBlock, func(block string) string { return pptxTemplateChartLabelSetting(block, "<c:showVal") }),
		PointShowCategories:  pptxTemplateDataLabelPointValues(dLblsBlock, func(block string) string { return pptxTemplateChartLabelSetting(block, "<c:showCatName") }),
		PointShowSeriesNames: pptxTemplateDataLabelPointValues(dLblsBlock, func(block string) string { return pptxTemplateChartLabelSetting(block, "<c:showSerName") }),
		PointShowPercents:    pptxTemplateDataLabelPointValues(dLblsBlock, func(block string) string { return pptxTemplateChartLabelSetting(block, "<c:showPercent") }),
		PointLabelPositions: pptxTemplateDataLabelPointValues(dLblsBlock, func(block string) string {
			return officeNormalizeChartLabelPosition(strings.TrimSpace(pptxTemplateTagAttributeValue(block, "<c:dLblPos", "val")))
		}),
		PointLabelFormats: pptxTemplateDataLabelPointValues(dLblsBlock, pptxTemplateDataLabelFormatValue),
		PointLabelSeparators: pptxTemplateDataLabelPointValues(dLblsBlock, func(block string) string {
			return officeNormalizeChartLabelSeparator(html.UnescapeString(pptxTemplateTagValue(block, "<c:separator>", "</c:separator>")))
		}),
	}
}

func pptxTemplateDataLabelPointValues(dLblsBlock string, valueFunc func(string) string) []string {
	pointBlocks := pptxTemplateChartBlocks(dLblsBlock, "<c:dLbl>", "</c:dLbl>")
	if len(pointBlocks) == 0 {
		return nil
	}
	valuesByIndex := make(map[int]string, len(pointBlocks))
	lastNonEmpty := -1
	for _, block := range pointBlocks {
		idx, ok := pptxTemplateDataPointIndex(block)
		if !ok {
			continue
		}
		value := valueFunc(block)
		if value == "" {
			continue
		}
		valuesByIndex[idx] = value
		if idx > lastNonEmpty {
			lastNonEmpty = idx
		}
	}
	if lastNonEmpty < 0 {
		return nil
	}
	values := make([]string, lastNonEmpty+1)
	for idx, value := range valuesByIndex {
		if idx >= 0 && idx < len(values) {
			values[idx] = value
		}
	}
	return values
}

func pptxTemplateDataLabelDeleteSetting(block string) string {
	switch strings.TrimSpace(pptxTemplateTagAttributeValue(block, "<c:delete", "val")) {
	case "0":
		return "show"
	case "1":
		return "hide"
	default:
		return ""
	}
}

func pptxTemplateDataLabelFormatValue(block string) string {
	formatCode := strings.TrimSpace(pptxTemplateTagAttributeValue(block, "<c:numFmt", "formatCode"))
	if formatCode == "" {
		return ""
	}
	sourceLinked := strings.TrimSpace(pptxTemplateTagAttributeValue(block, "<c:numFmt", "sourceLinked"))
	if strings.EqualFold(formatCode, "General") && sourceLinked != "0" {
		return ""
	}
	return officeNormalizeChartLabelFormat(formatCode)
}

func pptxTemplateSeriesStyleBlock(seriesBlock string) string {
	searchLimit := len(seriesBlock)
	for _, marker := range []string{
		"<c:invertIfNegative",
		"<c:marker>",
		"<c:dPt>",
		"<c:cat>",
		"<c:val>",
		"<c:xVal>",
		"<c:yVal>",
		"<c:bubbleSize>",
		"<c:bubble3D>",
		"<c:smooth",
	} {
		if idx := strings.Index(seriesBlock, marker); idx >= 0 && idx < searchLimit {
			searchLimit = idx
		}
	}
	if searchLimit <= 0 {
		return ""
	}
	return pptxTemplateFirstChartBlock(seriesBlock[:searchLimit], "<c:spPr>", "</c:spPr>")
}

func pptxTemplateSeriesLineWidth(xmlText string) (float64, bool) {
	value := strings.TrimSpace(pptxTemplateTagAttributeValue(xmlText, "<a:ln", "w"))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return float64(parsed) / 12700, true
}

func pptxTemplateSeriesDashValue(xmlText string) string {
	switch strings.ToLower(strings.TrimSpace(pptxTemplateTagAttributeValue(xmlText, "<a:prstDash", "val"))) {
	case "solid":
		return "solid"
	case "dash":
		return "dash"
	case "sysdot":
		return "dot"
	case "dashdot":
		return "dash_dot"
	default:
		return ""
	}
}

func pptxTemplateSeriesPointColors(seriesBlock string) []string {
	pointBlocks := pptxTemplateChartBlocks(seriesBlock, "<c:dPt>", "</c:dPt>")
	if len(pointBlocks) == 0 {
		return nil
	}
	colorsByIndex := make(map[int]string, len(pointBlocks))
	lastNonEmpty := -1
	for _, block := range pointBlocks {
		idx, ok := pptxTemplateDataPointIndex(block)
		if !ok {
			continue
		}
		spPrBlock := pptxTemplateFirstChartBlock(block, "<c:spPr>", "</c:spPr>")
		if spPrBlock == "" {
			continue
		}
		color := strings.TrimSpace(pptxTemplateTagAttributeValue(spPrBlock, "<a:srgbClr", "val"))
		if color == "" {
			continue
		}
		colorsByIndex[idx] = color
		if idx > lastNonEmpty {
			lastNonEmpty = idx
		}
	}
	if lastNonEmpty < 0 {
		return nil
	}
	colors := make([]string, lastNonEmpty+1)
	for idx, color := range colorsByIndex {
		if idx >= 0 && idx < len(colors) {
			colors[idx] = color
		}
	}
	return colors
}

func pptxTemplateSeriesPointExplosions(seriesBlock string) []int {
	pointBlocks := pptxTemplateChartBlocks(seriesBlock, "<c:dPt>", "</c:dPt>")
	if len(pointBlocks) == 0 {
		return nil
	}
	explosionsByIndex := make(map[int]int, len(pointBlocks))
	lastNonZero := -1
	for _, block := range pointBlocks {
		idx, ok := pptxTemplateDataPointIndex(block)
		if !ok {
			continue
		}
		value := strings.TrimSpace(pptxTemplateTagAttributeValue(block, "<c:explosion", "val"))
		if value == "" {
			continue
		}
		explosion, err := strconv.Atoi(value)
		if err != nil || explosion <= 0 {
			continue
		}
		explosionsByIndex[idx] = explosion
		if idx > lastNonZero {
			lastNonZero = idx
		}
	}
	if lastNonZero < 0 {
		return nil
	}
	explosions := make([]int, lastNonZero+1)
	for idx, explosion := range explosionsByIndex {
		if idx >= 0 && idx < len(explosions) {
			explosions[idx] = explosion
		}
	}
	return explosions
}

func pptxTemplateDataPointIndex(block string) (int, bool) {
	value := strings.TrimSpace(pptxTemplateTagAttributeValue(block, "<c:idx", "val"))
	if value == "" {
		return 0, false
	}
	idx, err := strconv.Atoi(value)
	if err != nil || idx < 0 {
		return 0, false
	}
	return idx, true
}

func inferPPTXTemplateComboSeriesDefaults(chart map[string]interface{}, chartXML []byte) {
	seriesItems, ok := chart["series"].([]interface{})
	if !ok || len(seriesItems) == 0 {
		return
	}
	defaults := inferPPTXTemplateExistingComboSeriesDefaults(chartXML)
	if len(defaults) == 0 {
		return
	}
	for idx, item := range seriesItems {
		seriesMap, ok := coerceCompatMap(item)
		if !ok {
			continue
		}
		name := strings.TrimSpace(anyToStringForLLM(firstMapValue(seriesMap, "name", "label")))
		if name == "" {
			name = fmt.Sprintf("Series %d", idx+1)
		}
		inferred, ok := defaults[name]
		if !ok {
			continue
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "type", "chart_type", "render_as", "renderAs") && inferred.Type != "" {
			seriesMap["type"] = inferred.Type
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "axis", "y_axis", "yAxis", "value_axis", "valueAxis") && inferred.Axis != "" {
			seriesMap["axis"] = inferred.Axis
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "labels", "data_labels", "show_labels", "showLabels") && inferred.Labels != "" {
			seriesMap["labels"] = inferred.Labels
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "label_position", "labelPosition", "labels_position", "data_label_position", "dataLabelPosition") && inferred.LabelPosition != "" {
			seriesMap["label_position"] = inferred.LabelPosition
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "label_format", "labelFormat", "labels_format", "data_label_format", "dataLabelFormat") && inferred.LabelFormat != "" {
			seriesMap["label_format"] = inferred.LabelFormat
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "label_separator", "labelSeparator", "data_label_separator", "dataLabelSeparator") && inferred.LabelSeparator != "" {
			seriesMap["label_separator"] = inferred.LabelSeparator
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "show_value", "showValue", "label_value", "labelValue") && inferred.ShowValue != "" {
			seriesMap["show_value"] = inferred.ShowValue
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "show_category", "showCategory", "label_category", "labelCategory") && inferred.ShowCategory != "" {
			seriesMap["show_category"] = inferred.ShowCategory
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "show_series_name", "showSeriesName", "label_series_name", "labelSeriesName") && inferred.ShowSeriesName != "" {
			seriesMap["show_series_name"] = inferred.ShowSeriesName
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "show_percent", "showPercent", "label_percent", "labelPercent") && inferred.ShowPercent != "" {
			seriesMap["show_percent"] = inferred.ShowPercent
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "show_legend_key", "showLegendKey", "label_legend_key", "labelLegendKey") && inferred.ShowLegendKey != "" {
			seriesMap["show_legend_key"] = inferred.ShowLegendKey
		}
		if !pptxTemplateChartHasAnyKey(seriesMap, "show_bubble_size", "showBubbleSize", "label_bubble_size", "labelBubbleSize") && inferred.ShowBubbleSize != "" {
			seriesMap["show_bubble_size"] = inferred.ShowBubbleSize
		}
		seriesItems[idx] = seriesMap
	}
	chart["series"] = seriesItems
}

func inferPPTXTemplateExistingComboSeriesDefaults(chartXML []byte) map[string]pptxTemplateChartSeriesDefault {
	xmlText := string(chartXML)
	valueAxisKinds := pptxTemplateValueAxisKinds(xmlText)
	defaults := make(map[string]pptxTemplateChartSeriesDefault)
	for _, group := range []struct {
		blockType string
		axisType  string
	}{
		{blockType: "barChart", axisType: "bar"},
		{blockType: "lineChart", axisType: "line"},
	} {
		for _, block := range pptxTemplateChartBlocks(xmlText, "<c:"+group.blockType+">", "</c:"+group.blockType+">") {
			axis := pptxTemplateChartGroupAxis(block, valueAxisKinds)
			seriesBlocks := pptxTemplateChartBlocks(block, "<c:ser>", "</c:ser>")
			labelDefaults := pptxTemplateChartSeriesDefault{}
			if len(seriesBlocks) == 1 {
				labelDefaults = pptxTemplateComboSeriesLabelDefaults(block)
			}
			for _, seriesBlock := range seriesBlocks {
				name := pptxTemplateSeriesName(seriesBlock)
				if name == "" {
					continue
				}
				defaults[name] = pptxTemplateChartSeriesDefault{
					Type:           group.axisType,
					Axis:           axis,
					Labels:         labelDefaults.Labels,
					LabelPosition:  labelDefaults.LabelPosition,
					LabelFormat:    labelDefaults.LabelFormat,
					LabelSeparator: labelDefaults.LabelSeparator,
					ShowValue:      labelDefaults.ShowValue,
					ShowCategory:   labelDefaults.ShowCategory,
					ShowSeriesName: labelDefaults.ShowSeriesName,
					ShowPercent:    labelDefaults.ShowPercent,
					ShowLegendKey:  labelDefaults.ShowLegendKey,
					ShowBubbleSize: labelDefaults.ShowBubbleSize,
				}
			}
		}
	}
	return defaults
}

func pptxTemplateComboSeriesLabelDefaults(groupBlock string) pptxTemplateChartSeriesDefault {
	dLblsBlock := pptxTemplateFirstChartBlock(groupBlock, "<c:dLbls>", "</c:dLbls>")
	if dLblsBlock == "" {
		return pptxTemplateChartSeriesDefault{}
	}
	return pptxTemplateChartSeriesDefault{
		Labels:         "show",
		LabelPosition:  strings.TrimSpace(pptxTemplateTagAttributeValue(dLblsBlock, "<c:dLblPos", "val")),
		LabelFormat:    strings.TrimSpace(pptxTemplateTagAttributeValue(dLblsBlock, "<c:numFmt", "formatCode")),
		LabelSeparator: strings.TrimSpace(html.UnescapeString(pptxTemplateTagValue(dLblsBlock, "<c:separator>", "</c:separator>"))),
		ShowValue:      pptxTemplateChartLabelSetting(dLblsBlock, "<c:showVal"),
		ShowCategory:   pptxTemplateChartLabelSetting(dLblsBlock, "<c:showCatName"),
		ShowSeriesName: pptxTemplateChartLabelSetting(dLblsBlock, "<c:showSerName"),
		ShowPercent:    pptxTemplateChartLabelSetting(dLblsBlock, "<c:showPercent"),
		ShowLegendKey:  pptxTemplateChartLabelSetting(dLblsBlock, "<c:showLegendKey"),
		ShowBubbleSize: pptxTemplateChartLabelSetting(dLblsBlock, "<c:showBubbleSize"),
	}
}

func pptxTemplateChartLabelSetting(xmlText, tagStart string) string {
	switch strings.TrimSpace(pptxTemplateTagAttributeValue(xmlText, tagStart, "val")) {
	case "0":
		return "hide"
	case "1":
		return "show"
	default:
		return ""
	}
}

func pptxTemplateDateAxisFormatCode(xmlText string) string {
	return pptxTemplateDateAxisFormatCodeAt(xmlText, 0)
}

func pptxTemplateDateAxisFormatCodeAt(xmlText string, axisIndex int) string {
	block := pptxTemplateDateAxisBlockAt(xmlText, axisIndex)
	if block == "" {
		return ""
	}
	return strings.TrimSpace(pptxTemplateTagAttributeValue(block, "<c:numFmt", "formatCode"))
}

func pptxTemplateDateAxisBlockAt(xmlText string, axisIndex int) string {
	if axisIndex < 0 {
		return ""
	}
	searchFrom := 0
	for current := 0; current <= axisIndex; current++ {
		start := strings.Index(xmlText[searchFrom:], "<c:dateAx")
		if start < 0 {
			return ""
		}
		start += searchFrom
		end := strings.Index(xmlText[start:], "</c:dateAx>")
		if end < 0 {
			return ""
		}
		block := xmlText[start : start+end]
		if current == axisIndex {
			return block
		}
		searchFrom = start + end + len("</c:dateAx>")
	}
	return ""
}

func pptxTemplateCategoryAxisBlockAt(xmlText string, axisIndex int) string {
	if strings.Contains(xmlText, "<c:dateAx") {
		return pptxTemplateDateAxisBlockAt(xmlText, axisIndex)
	}
	blocks := pptxTemplateChartBlocks(xmlText, "<c:catAx>", "</c:catAx>")
	if axisIndex < 0 || axisIndex >= len(blocks) {
		return ""
	}
	return blocks[axisIndex]
}

func pptxTemplateCategoryAxisAttribute(xmlText string, axisIndex int, tagStart, attribute string) string {
	block := pptxTemplateCategoryAxisBlockAt(xmlText, axisIndex)
	if block == "" {
		return ""
	}
	return strings.TrimSpace(pptxTemplateTagAttributeValue(block, tagStart, attribute))
}

func pptxTemplateFirstChartBlock(xmlText, startTag, endTag string) string {
	start := strings.Index(xmlText, startTag)
	if start < 0 {
		return ""
	}
	end := strings.Index(xmlText[start:], endTag)
	if end < 0 {
		return xmlText[start:]
	}
	return xmlText[start : start+end+len(endTag)]
}

func pptxTemplateChartBlocks(xmlText, startTag, endTag string) []string {
	blocks := make([]string, 0, 2)
	searchFrom := 0
	for searchFrom < len(xmlText) {
		start := strings.Index(xmlText[searchFrom:], startTag)
		if start < 0 {
			break
		}
		start += searchFrom
		end := strings.Index(xmlText[start:], endTag)
		if end < 0 {
			break
		}
		end += start + len(endTag)
		blocks = append(blocks, xmlText[start:end])
		searchFrom = end
	}
	return blocks
}

func pptxTemplateChartGroupAxis(chartBlock string, valueAxisKinds map[string]string) string {
	axisIDs := pptxTemplateAxisIDs(chartBlock)
	if len(axisIDs) >= 2 {
		if axis := valueAxisKinds[axisIDs[1]]; axis != "" {
			return axis
		}
	}
	return "primary"
}

func pptxTemplateValueAxisKinds(xmlText string) map[string]string {
	kinds := make(map[string]string)
	for _, block := range pptxTemplateChartBlocks(xmlText, "<c:valAx>", "</c:valAx>") {
		axisIDs := pptxTemplateAxisIDs(block)
		if len(axisIDs) == 0 {
			continue
		}
		axisKind := "primary"
		if pptxTemplateTagAttributeValue(block, "<c:axPos", "val") == "r" {
			axisKind = "secondary"
		}
		kinds[axisIDs[0]] = axisKind
	}
	return kinds
}

func pptxTemplateValueAxisFormatCode(xmlText, axisKind string) string {
	block := pptxTemplateValueAxisBlock(xmlText, axisKind)
	if block == "" {
		return ""
	}
	formatCode := strings.TrimSpace(pptxTemplateTagAttributeValue(block, "<c:numFmt", "formatCode"))
	if formatCode == "" {
		return ""
	}
	sourceLinked := strings.TrimSpace(pptxTemplateTagAttributeValue(block, "<c:numFmt", "sourceLinked"))
	if strings.EqualFold(formatCode, "General") && sourceLinked != "0" {
		return ""
	}
	return formatCode
}

func pptxTemplateValueAxisNumber(xmlText, axisKind, tagStart string) (float64, bool) {
	block := pptxTemplateValueAxisBlock(xmlText, axisKind)
	if block == "" {
		return 0, false
	}
	value := strings.TrimSpace(pptxTemplateTagAttributeValue(block, tagStart, "val"))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func pptxTemplateValueAxisAttribute(xmlText, axisKind, tagStart, attribute string) string {
	block := pptxTemplateValueAxisBlock(xmlText, axisKind)
	if block == "" {
		return ""
	}
	return strings.TrimSpace(pptxTemplateTagAttributeValue(block, tagStart, attribute))
}

func pptxTemplateValueAxisBlock(xmlText, axisKind string) string {
	for _, block := range pptxTemplateChartBlocks(xmlText, "<c:valAx>", "</c:valAx>") {
		currentAxisKind := "primary"
		if pptxTemplateTagAttributeValue(block, "<c:axPos", "val") == "r" {
			currentAxisKind = "secondary"
		}
		if currentAxisKind == axisKind {
			return block
		}
	}
	return ""
}

func pptxTemplateDateAxisNumber(xmlText string, axisIndex int, tagStart string) (float64, bool) {
	block := pptxTemplateDateAxisBlockAt(xmlText, axisIndex)
	if block == "" {
		return 0, false
	}
	value := strings.TrimSpace(pptxTemplateTagAttributeValue(block, tagStart, "val"))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func pptxTemplateDateAxisTimeUnit(xmlText string, axisIndex int, tagStart string) string {
	block := pptxTemplateDateAxisBlockAt(xmlText, axisIndex)
	if block == "" {
		return ""
	}
	return strings.TrimSpace(pptxTemplateTagAttributeValue(block, tagStart, "val"))
}

func pptxTemplateChartWouldShowLegendByDefault(xmlText string) bool {
	chartType := inferPPTXTemplateChartType([]byte(xmlText))
	seriesCount := strings.Count(xmlText, "<c:ser>")
	categoryCount := pptxTemplateChartCategoryPointCount(xmlText)
	switch chartType {
	case "pie", "donut":
		return categoryCount > 1
	case "line":
		return seriesCount > 1
	case "":
		return false
	default:
		return seriesCount > 0
	}
}

func pptxTemplateChartCategoryPointCount(xmlText string) int {
	categoryBlock := pptxTemplateFirstChartBlock(xmlText, "<c:cat>", "</c:cat>")
	if categoryBlock == "" {
		return 0
	}
	return strings.Count(categoryBlock, "<c:pt ")
}

func pptxTemplateChartAppearanceAttribute(xmlText, tagStart, attribute string) string {
	block := pptxTemplateChartAppearanceBlock(xmlText)
	if block == "" {
		return ""
	}
	return strings.TrimSpace(pptxTemplateTagAttributeValue(block, tagStart, attribute))
}

func pptxTemplateChartAppearanceInt(xmlText, tagStart string) (int, bool) {
	value := pptxTemplateChartAppearanceAttribute(xmlText, tagStart, "val")
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func pptxTemplateChartUniformIntAcrossBlocks(blocks []string, tagStart string) (int, bool) {
	value, ok := pptxTemplateUniformAttributeAcrossBlocks(blocks, tagStart, "val")
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func pptxTemplateChartAppearanceBlock(xmlText string) string {
	switch inferPPTXTemplateChartType([]byte(xmlText)) {
	case "pie":
		return pptxTemplateFirstChartBlock(xmlText, "<c:pieChart>", "</c:pieChart>")
	case "donut":
		return pptxTemplateFirstChartBlock(xmlText, "<c:doughnutChart>", "</c:doughnutChart>")
	case "line":
		return pptxTemplateFirstChartBlock(xmlText, "<c:lineChart>", "</c:lineChart>")
	case "bar", "stacked_bar", "stacked_column", "percent_stacked_bar", "percent_stacked_column":
		return pptxTemplateFirstChartBlock(xmlText, "<c:barChart>", "</c:barChart>")
	default:
		return ""
	}
}

func pptxTemplateChartAppearanceBlocks(xmlText string) []string {
	switch inferPPTXTemplateChartType([]byte(xmlText)) {
	case "combo":
		blocks := make([]string, 0, 4)
		blocks = append(blocks, pptxTemplateChartBlocks(xmlText, "<c:barChart>", "</c:barChart>")...)
		blocks = append(blocks, pptxTemplateChartBlocks(xmlText, "<c:lineChart>", "</c:lineChart>")...)
		return blocks
	case "pie", "donut", "line", "bar", "stacked_bar", "stacked_column", "percent_stacked_bar", "percent_stacked_column":
		if block := pptxTemplateChartAppearanceBlock(xmlText); block != "" {
			return []string{block}
		}
	}
	return nil
}

func pptxTemplateUniformAttributeAcrossBlocks(blocks []string, tagStart, attribute string) (string, bool) {
	value := ""
	found := false
	for _, block := range blocks {
		current := strings.TrimSpace(pptxTemplateTagAttributeValue(block, tagStart, attribute))
		if current == "" {
			return "", false
		}
		if !found {
			value = current
			found = true
			continue
		}
		if current != value {
			return "", false
		}
	}
	if !found {
		return "", false
	}
	return value, true
}

func pptxTemplateChartDataLabelsDefaultAttribute(xmlText, tagStart, attribute string) string {
	block := pptxTemplateUniformDataLabelsDefaultsBlock(xmlText)
	if block == "" {
		return ""
	}
	return strings.TrimSpace(pptxTemplateTagAttributeValue(block, tagStart, attribute))
}

func pptxTemplateUniformDataLabelsDefaultsBlock(xmlText string) string {
	block := pptxTemplateUniformDataLabelsBlock(xmlText)
	if block == "" {
		return ""
	}
	for _, pointBlock := range pptxTemplateChartBlocks(block, "<c:dLbl>", "</c:dLbl>") {
		block = strings.Replace(block, pointBlock, "", 1)
	}
	return block
}

func pptxTemplateUniformDataLabelsHasChartDefaults(xmlText string) bool {
	block := pptxTemplateUniformDataLabelsDefaultsBlock(xmlText)
	if block == "" {
		return false
	}
	start := strings.Index(block, ">")
	end := strings.LastIndex(block, "</c:dLbls>")
	if start < 0 || end <= start {
		return false
	}
	return strings.TrimSpace(block[start+1:end]) != ""
}

func pptxTemplateUniformDataLabelsBlock(xmlText string) string {
	if inferPPTXTemplateChartType([]byte(xmlText)) == "combo" {
		return ""
	}
	blocks := pptxTemplateChartBlocks(xmlText, "<c:dLbls>", "</c:dLbls>")
	if len(blocks) == 0 {
		return ""
	}
	first := blocks[0]
	for _, block := range blocks[1:] {
		if block != first {
			return ""
		}
	}
	return first
}

func pptxTemplateAxisIDs(xmlText string) []string {
	ids := make([]string, 0, 2)
	searchFrom := 0
	for searchFrom < len(xmlText) {
		start := strings.Index(xmlText[searchFrom:], "<c:axId")
		if start < 0 {
			break
		}
		start += searchFrom
		value := strings.TrimSpace(pptxTemplateTagAttributeValue(xmlText[start:], "<c:axId", "val"))
		if value != "" {
			ids = append(ids, value)
		}
		searchFrom = start + len("<c:axId")
	}
	return ids
}

func pptxTemplateSeriesName(seriesBlock string) string {
	txBlock := pptxTemplateFirstChartBlock(seriesBlock, "<c:tx>", "</c:tx>")
	if txBlock == "" {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(pptxTemplateTagValue(txBlock, "<c:v>", "</c:v>")))
}

func pptxTemplateTitleText(xmlText string) string {
	titleBlock := pptxTemplateFirstChartBlock(xmlText, "<c:title>", "</c:title>")
	if titleBlock == "" {
		return ""
	}
	values := pptxTemplateTagValues(titleBlock, "<a:t>", "</a:t>")
	if len(values) == 0 {
		return ""
	}
	var text strings.Builder
	for _, value := range values {
		text.WriteString(html.UnescapeString(value))
	}
	return strings.TrimSpace(text.String())
}

func pptxTemplateTagAttributeValue(xmlText, tagStart, attribute string) string {
	start := strings.Index(xmlText, tagStart)
	if start < 0 {
		return ""
	}
	end := strings.Index(xmlText[start:], ">")
	if end < 0 {
		return ""
	}
	tag := xmlText[start : start+end]
	pattern := attribute + `="`
	valueStart := strings.Index(tag, pattern)
	if valueStart < 0 {
		return ""
	}
	valueStart += len(pattern)
	valueEnd := strings.Index(tag[valueStart:], `"`)
	if valueEnd < 0 {
		return ""
	}
	return tag[valueStart : valueStart+valueEnd]
}

func pptxTemplateTagValue(xmlText, startTag, endTag string) string {
	start := strings.Index(xmlText, startTag)
	if start < 0 {
		return ""
	}
	start += len(startTag)
	end := strings.Index(xmlText[start:], endTag)
	if end < 0 {
		return ""
	}
	return xmlText[start : start+end]
}

func pptxTemplateTagValues(xmlText, startTag, endTag string) []string {
	values := make([]string, 0, 2)
	searchFrom := 0
	for searchFrom < len(xmlText) {
		start := strings.Index(xmlText[searchFrom:], startTag)
		if start < 0 {
			break
		}
		start += searchFrom + len(startTag)
		end := strings.Index(xmlText[start:], endTag)
		if end < 0 {
			break
		}
		values = append(values, xmlText[start:start+end])
		searchFrom = start + end + len(endTag)
	}
	return values
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
	seenChartIndices := make(map[int]int)

	for idx, slide := range s.slides {
		slideName := fmt.Sprintf("slide%d.xml", idx+1)
		slidePath := filepath.Join(slidesDir, slideName)
		if err := os.WriteFile(slidePath, slide.XML, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", slideName, err)
		}

		// Process slide charts - remap chart indices and write chart files
		updatedRels := slide.Rels
		for _, chart := range slide.Charts {
			newChartIndex := chart.ChartIndex
			if seenChartIndices[chart.ChartIndex] > 0 {
				maxChartIndex++
				newChartIndex = maxChartIndex
			}
			seenChartIndices[chart.ChartIndex]++

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
			} else if err := os.Remove(chartRelsPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove chart rels %s: %w", chartName, err)
			}

			// Update slide rels to point to new chart index
			if newChartIndex != chart.ChartIndex {
				oldPattern := fmt.Sprintf("chart%d.xml", chart.ChartIndex)
				newPattern := fmt.Sprintf("chart%d.xml", newChartIndex)
				updatedRels = []byte(strings.ReplaceAll(string(updatedRels), oldPattern, newPattern))
			}
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
	if err := cleanupPPTXOrphanChartArtifacts(s.tempDir, len(s.slides)); err != nil {
		return err
	}
	if err := cleanupPPTXOrphanMedia(s.tempDir, len(s.slides)); err != nil {
		return err
	}
	contentTypesXML, err := buildPPTXTemplateContentTypesXML(s.tempDir, len(s.slides))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.tempDir, "[Content_Types].xml"), []byte(contentTypesXML), 0o644); err != nil {
		return fmt.Errorf("write content types: %w", err)
	}
	return nil
}

func buildPPTXTemplateContentTypesXML(tempDir string, slideCount int) (string, error) {
	chartNames, err := pptxTemplateChartPartNames(tempDir)
	if err != nil {
		return "", err
	}
	hasEmbeddedWorkbook, err := pptxTemplateHasEmbeddedWorkbook(tempDir)
	if err != nil {
		return "", err
	}

	var overrides strings.Builder
	for idx := 1; idx <= slideCount; idx++ {
		overrides.WriteString(fmt.Sprintf(`<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, idx))
	}
	for _, chartName := range chartNames {
		overrides.WriteString(`<Override PartName="/ppt/charts/` + officeXMLText(chartName) + `" ContentType="application/vnd.openxmlformats-officedocument.drawingml.chart+xml"/>`)
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		func() string {
			if hasEmbeddedWorkbook {
				return `<Default Extension="xlsx" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"/>`
			}
			return ``
		}() +
		`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>` +
		`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
		`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>` +
		`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>` +
		`<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>` +
		`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
		overrides.String() +
		`</Types>`, nil
}

func pptxTemplateChartPartNames(tempDir string) ([]string, error) {
	chartsDir := filepath.Join(tempDir, "ppt", "charts")
	entries, err := os.ReadDir(chartsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read charts dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !strings.HasPrefix(name, "chart") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func pptxTemplateHasEmbeddedWorkbook(tempDir string) (bool, error) {
	embeddingsDir := filepath.Join(tempDir, "ppt", "embeddings")
	entries, err := os.ReadDir(embeddingsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read embeddings dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(strings.TrimSpace(entry.Name())), ".xlsx") {
			return true, nil
		}
	}
	return false, nil
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

func cleanupPPTXOrphanChartArtifacts(tempDir string, slideCount int) error {
	usedCharts := make(map[string]struct{})
	for idx := 1; idx <= slideCount; idx++ {
		relsPath := filepath.Join(tempDir, "ppt", "slides", "_rels", fmt.Sprintf("slide%d.xml.rels", idx))
		rels, err := readPPTXRelationshipsFile(relsPath)
		if err != nil {
			continue
		}
		for _, rel := range rels.Relationships {
			target := strings.TrimSpace(rel.Target)
			if target == "" {
				continue
			}
			resolved := path.Clean(path.Join("ppt/slides", target))
			if strings.HasPrefix(resolved, "ppt/charts/") && strings.HasSuffix(strings.ToLower(resolved), ".xml") {
				usedCharts[resolved] = struct{}{}
			}
		}
	}

	usedEmbeddings := make(map[string]struct{})
	for chartPath := range usedCharts {
		relsPath := filepath.Join(tempDir, filepath.FromSlash(path.Join("ppt/charts/_rels", path.Base(chartPath)+".rels")))
		rels, err := readPPTXRelationshipsFile(relsPath)
		if err != nil {
			continue
		}
		for _, rel := range rels.Relationships {
			target := strings.TrimSpace(rel.Target)
			if target == "" {
				continue
			}
			resolved := path.Clean(path.Join("ppt/charts", target))
			if strings.HasPrefix(resolved, "ppt/embeddings/") && strings.HasSuffix(strings.ToLower(resolved), ".xlsx") {
				usedEmbeddings[resolved] = struct{}{}
			}
		}
	}

	chartsDir := filepath.Join(tempDir, "ppt", "charts")
	chartEntries, err := os.ReadDir(chartsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read charts dir: %w", err)
	}
	for _, entry := range chartEntries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !strings.HasPrefix(name, "chart") || !strings.HasSuffix(strings.ToLower(name), ".xml") {
			continue
		}
		relPath := path.Clean(path.Join("ppt/charts", name))
		if _, ok := usedCharts[relPath]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(chartsDir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove orphan chart %s: %w", name, err)
		}
		relsPath := filepath.Join(tempDir, "ppt", "charts", "_rels", name+".rels")
		if err := os.Remove(relsPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove orphan chart rels %s: %w", name, err)
		}
	}

	embeddingsDir := filepath.Join(tempDir, "ppt", "embeddings")
	embeddingEntries, err := os.ReadDir(embeddingsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read embeddings dir: %w", err)
	}
	for _, entry := range embeddingEntries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !strings.HasSuffix(strings.ToLower(name), ".xlsx") {
			continue
		}
		relPath := path.Clean(path.Join("ppt/embeddings", name))
		if _, ok := usedEmbeddings[relPath]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(embeddingsDir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove orphan embedding %s: %w", name, err)
		}
	}

	return nil
}

func readPPTXRelationshipsFile(path string) (pptxRelationshipsXML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pptxRelationshipsXML{}, err
	}
	var rels pptxRelationshipsXML
	if err := xml.Unmarshal(data, &rels); err != nil {
		return pptxRelationshipsXML{}, err
	}
	return rels, nil
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
