package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type OfficeTool struct {
	scope *fsToolScope
}

type officeWorkbookSpec struct {
	Title    string
	Subtitle string
	Theme    officeTheme
	Stats    []officeStat
	Notes    []string
	Sheets   []officeSheetSpec
}

type officeDocSpec struct {
	Title      string
	Subtitle   string
	Summary    string
	Theme      officeTheme
	StyleHint  string
	Sections   []officeDocSection
	Paragraphs []string
	Notes      []string
}

type officeStat struct {
	Label string
	Value string
	Tone  string
}

type officeSheetSpec struct {
	Name    string
	Columns []officeColumnSpec
	Rows    [][]interface{}
	Freeze  string
	Filter  bool
}

type officeColumnSpec struct {
	Header string
	Key    string
	Width  float64
	Kind   string
}

type officeDocSection struct {
	Heading    string
	Paragraphs []string
	Bullets    []string
	Table      *officeTableSpec
	Chart      *officeChartSpec
}

type officeTableSpec struct {
	Headers          []string
	Rows             [][]string
	ColumnWidths     []float64
	ColumnAlignments []string
}

type officeChartSpec struct {
	Type                                  string
	Title                                 string
	CategoryAxisTitle                     string
	SecondaryCategoryAxisTitle            string
	CategoryAxisType                      string
	CategoryAxisLabelPosition             string
	SecondaryCategoryAxisLabelPosition    string
	CategoryAxisReverseOrder              string
	SecondaryCategoryAxisReverseOrder     string
	CategoryAxisCrosses                   string
	SecondaryCategoryAxisCrosses          string
	CategoryAxisMajorTickMark             string
	CategoryAxisMinorTickMark             string
	SecondaryCategoryAxisMajorTickMark    string
	SecondaryCategoryAxisMinorTickMark    string
	CategoryAxisLabelAlignment            string
	SecondaryCategoryAxisLabelAlignment   string
	CategoryAxisLabelOffset               *int
	SecondaryCategoryAxisLabelOffset      *int
	CategoryAxisMultiLevelLabels          string
	SecondaryCategoryAxisMultiLevelLabels string
	CategoryAxisVisible                   string
	SecondaryCategoryAxisVisible          string
	CategoryAxisAuto                      string
	SecondaryCategoryAxisAuto             string
	CategoryAxisFormat                    string
	SecondaryCategoryAxisFormat           string
	CategoryAxisMin                       *float64
	CategoryAxisMax                       *float64
	CategoryAxisBaseTimeUnit              string
	CategoryAxisMajorUnit                 *float64
	CategoryAxisMinorUnit                 *float64
	CategoryAxisMajorTimeUnit             string
	CategoryAxisMinorTimeUnit             string
	ValueAxisTitle                        string
	SecondaryValueAxisTitle               string
	ValueAxisFormat                       string
	SecondaryValueAxisFormat              string
	ValueAxisMin                          *float64
	ValueAxisMax                          *float64
	SecondaryValueAxisMin                 *float64
	SecondaryValueAxisMax                 *float64
	ValueAxisMajorUnit                    *float64
	ValueAxisMinorUnit                    *float64
	SecondaryValueAxisMajorUnit           *float64
	SecondaryValueAxisMinorUnit           *float64
	ValueAxisMajorGridlines               string
	ValueAxisMinorGridlines               string
	SecondaryValueAxisMajorGridlines      string
	SecondaryValueAxisMinorGridlines      string
	ValueAxisCrosses                      string
	SecondaryValueAxisCrosses             string
	ValueAxisCrossBetween                 string
	SecondaryValueAxisCrossBetween        string
	ValueAxisReverseOrder                 string
	SecondaryValueAxisReverseOrder        string
	ValueAxisLabelPosition                string
	SecondaryValueAxisLabelPosition       string
	ValueAxisMajorTickMark                string
	ValueAxisMinorTickMark                string
	SecondaryValueAxisMajorTickMark       string
	SecondaryValueAxisMinorTickMark       string
	ShowLegend                            string
	LegendPosition                        string
	VaryColors                            string
	StartAngle                            int
	HoleSize                              int
	Smooth                                string
	GapWidth                              *int
	Overlap                               *int
	Labels                                string
	LabelPosition                         string
	LabelFormat                           string
	LabelSeparator                        string
	ShowLeaderLines                       string
	ShowValue                             string
	ShowCategory                          string
	ShowSeriesName                        string
	ShowPercent                           string
	ShowLegendKey                         string
	ShowBubbleSize                        string
	Categories                            []string
	Series                                []officeChartSeries
}

type officeChartSeries struct {
	Name                 string
	WorkbookIndex        int
	Type                 string
	Axis                 string
	Labels               string
	LabelPosition        string
	LabelFormat          string
	LabelSeparator       string
	ShowLeaderLines      string
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
	PointColors          []string
	PointExplosions      []int
	Smooth               string
	Color                string
	LineWidth            float64
	Dash                 string
	Marker               string
	Values               []float64
}

type officeBuildInfo struct {
	Format         string
	SheetCount     int
	RowCount       int
	SectionCount   int
	ParagraphCount int
}

func NewOfficeTool(allowedPaths []string, approvals *ApprovalManager, dirStore *DirAllowlistStore) *OfficeTool {
	scope := newFSToolScope(allowedPaths)
	scope = scope.withApprovalFlow(approvals, dirStore)
	return &OfficeTool{scope: scope}
}

func RegisterOfficeTool(registry *Registry, allowedPaths []string, approvals *ApprovalManager, dirStore *DirAllowlistStore) {
	if registry == nil {
		return
	}
	registry.Register(NewOfficeTool(allowedPaths, approvals, dirStore))
}

func (t *OfficeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "office",
		Description: "Create polished .xlsx spreadsheets and .docx reports with native built-in themes. Prefer this for styled Excel/Word artifacts instead of raw file_write when the target path ends in .xlsx or .docx. Use theme values such as analysis, ui_review, executive, or clean, and optionally provide style_hint for tone guidance.",
		Icon:        "file-text",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Destination workspace path ending in .xlsx or .docx.",
				},
				"format": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"xlsx", "docx"},
					"description": "Optional explicit format. Usually inferred from path.",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Optional title shown in the workbook overview sheet or Word cover heading.",
				},
				"subtitle": map[string]interface{}{
					"type":        "string",
					"description": "Optional subtitle or deck/report descriptor.",
				},
				"theme": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"analysis", "ui_review", "executive", "clean"},
					"description": "Built-in visual theme. Default is inferred from style_hint, otherwise analysis.",
				},
				"style_hint": map[string]interface{}{
					"type":        "string",
					"description": "Optional tone and styling guidance such as 审计报告、UI 评估、稳重商务、简洁中性. The tool maps this onto the closest built-in theme and typography/layout density.",
				},
				"summary": map[string]interface{}{
					"description": "Optional high-level summary. For docx this can be a string or object with text/body. For xlsx this can be an object with stats/kpis and notes.",
				},
				"headers": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Convenience form for a single-sheet spreadsheet header row.",
				},
				"rows": map[string]interface{}{
					"type":        "array",
					"description": "Convenience form for a single-sheet spreadsheet. Each row can be an array or an object.",
				},
				"columns": map[string]interface{}{
					"type":        "array",
					"description": "Optional spreadsheet column definitions. Each item can be a string header or an object with header/key/width/kind.",
				},
				"sheets": map[string]interface{}{
					"type":        "array",
					"description": "Workbook sheets for xlsx output. Each sheet can define name, columns/headers, rows, freeze, and filter.",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "For docx output, optional plain text or simple Markdown-like content. Supports # headings, bullets, and pipe tables.",
				},
				"sections": map[string]interface{}{
					"type":        "array",
					"description": "For docx output, optional structured sections. Each section can include heading, paragraphs/body/text, bullets/items, and table. The shared parser also accepts pptx-specific chart blocks when used through the pptx tool.",
				},
				"notes": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Optional short notes shown in the overview sheet or document footer blocks.",
				},
				"create_dirs": map[string]interface{}{
					"type":        "boolean",
					"description": "Create parent directories when needed. Default true.",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (t *OfficeTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path := strings.TrimSpace(firstCompatPathString(args))
	if path == "" {
		var err error
		path, err = fsAsString(args, "path")
		if err != nil || path == "" {
			return nil, errors.New("path must be a non-empty string")
		}
	}
	format, err := inferOfficeFormat(path, firstCompatString(args, "format"))
	if err != nil {
		return nil, err
	}

	styleHint := firstCompatString(args, "style_hint", "styleHint", "style", "visual_style", "visualStyle")
	theme := resolveOfficeTheme(firstCompatString(args, "theme"), styleHint)
	title := strings.TrimSpace(firstCompatString(args, "title"))
	if title == "" && format == "xlsx" {
		title = officeDefaultTitle(path)
	}
	subtitle := strings.TrimSpace(firstCompatString(args, "subtitle"))

	createDirs, err := fsAsBool(args, "create_dirs", true)
	if err != nil {
		return nil, err
	}

	var (
		data []byte
		info officeBuildInfo
	)
	switch format {
	case "xlsx":
		spec, parseErr := parseOfficeWorkbookSpec(args, title, subtitle, theme)
		if parseErr != nil {
			return nil, parseErr
		}
		data, info, err = buildOfficeXLSX(spec)
	case "docx":
		spec, parseErr := parseOfficeDocSpec(args, title, subtitle, theme, styleHint)
		if parseErr != nil {
			return nil, parseErr
		}
		data, info, err = buildOfficeDOCX(spec)
	default:
		err = fmt.Errorf("unsupported office format: %s", format)
	}
	if err != nil {
		return nil, err
	}

	originalPath := path
	absPath, relPath, _, err := t.scope.resolvePathWithContext(ctx, "office", path, false)
	if err != nil {
		return nil, err
	}
	if err := enforceWritePathGuard(ctx, absPath); err != nil {
		return nil, err
	}
	if createDirs {
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write office file: %w", err)
	}

	response := map[string]interface{}{
		"path":          relPath,
		"absolute_path": absPath,
		"original_path": originalPath,
		"format":        format,
		"theme":         theme.Name,
		"success":       true,
		"size":          len(data),
		"message":       fmt.Sprintf("Created styled %s artifact at %s", format, relPath),
	}
	if info.SheetCount > 0 {
		response["sheet_count"] = info.SheetCount
		response["row_count"] = info.RowCount
	}
	if info.SectionCount > 0 {
		response["section_count"] = info.SectionCount
		response["paragraph_count"] = info.ParagraphCount
	}
	jsonResult, _ := json.Marshal(response)
	return string(jsonResult), nil
}

func inferOfficeFormat(path, rawFormat string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(rawFormat))
	if format == "" {
		format = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(filepath.Ext(path))), ".")
	}
	switch format {
	case "xlsx", "docx":
		return format, nil
	default:
		return "", errors.New("office format must be xlsx or docx, or the path must end in .xlsx/.docx")
	}
}

func officeDefaultTitle(path string) string {
	base := strings.TrimSpace(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if base == "" {
		return "Office Artifact"
	}
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")
	return strings.TrimSpace(base)
}

func parseOfficeWorkbookSpec(args map[string]interface{}, title, subtitle string, theme officeTheme) (officeWorkbookSpec, error) {
	spec := officeWorkbookSpec{
		Title:    title,
		Subtitle: subtitle,
		Theme:    theme,
		Notes:    officeStringSliceArg(args, "notes"),
	}

	stats, notes := parseOfficeSummaryStats(args)
	spec.Stats = append(spec.Stats, stats...)
	if len(spec.Notes) == 0 && len(notes) > 0 {
		spec.Notes = append(spec.Notes, notes...)
	}

	if rawSheets, ok := compatArgValue(args, "sheets"); ok {
		sheets, err := parseOfficeSheetArray(rawSheets)
		if err != nil {
			return spec, err
		}
		spec.Sheets = append(spec.Sheets, sheets...)
	}
	if len(spec.Sheets) == 0 {
		sheet, err := parseOfficeSingleSheet(args)
		if err != nil {
			return spec, err
		}
		spec.Sheets = append(spec.Sheets, sheet)
	}

	totalRows := 0
	for _, sheet := range spec.Sheets {
		totalRows += len(sheet.Rows)
	}
	if len(spec.Stats) == 0 {
		spec.Stats = []officeStat{
			{Label: "Sheets", Value: fmt.Sprintf("%d", len(spec.Sheets)), Tone: "primary"},
			{Label: "Rows", Value: fmt.Sprintf("%d", totalRows), Tone: "success"},
			{Label: "Theme", Value: theme.Name, Tone: "muted"},
		}
	}
	return spec, nil
}

func parseOfficeDocSpec(args map[string]interface{}, title, subtitle string, theme officeTheme, styleHint string) (officeDocSpec, error) {
	spec := officeDocSpec{
		Title:     title,
		Subtitle:  subtitle,
		Theme:     theme,
		StyleHint: strings.TrimSpace(styleHint),
		Notes:     officeStringSliceArg(args, "notes"),
	}

	spec.Summary = parseOfficeSummaryText(args)
	if rawSections, ok := compatArgValue(args, "sections"); ok {
		sections, err := parseOfficeDocSections(rawSections)
		if err != nil {
			return spec, err
		}
		spec.Sections = append(spec.Sections, sections...)
	}

	if len(spec.Sections) == 0 {
		content := strings.TrimSpace(firstCompatString(args, "content", "markdown", "body", "text"))
		if content != "" {
			contentSpec := parseMarkdownishOfficeDoc(content)
			if spec.Title == "" {
				spec.Title = contentSpec.Title
			}
			if spec.Subtitle == "" {
				spec.Subtitle = contentSpec.Subtitle
			}
			if spec.Summary == "" {
				spec.Summary = contentSpec.Summary
			}
			spec.Sections = append(spec.Sections, contentSpec.Sections...)
			spec.Paragraphs = append(spec.Paragraphs, contentSpec.Paragraphs...)
		}
	}
	if topParagraphs := officeStringSliceArg(args, "paragraphs"); len(topParagraphs) > 0 {
		spec.Paragraphs = append(spec.Paragraphs, topParagraphs...)
	}
	if len(spec.Sections) == 0 && len(spec.Paragraphs) == 0 && strings.TrimSpace(spec.Summary) == "" {
		return spec, errors.New("docx content requires sections, content/markdown, paragraphs, or summary")
	}
	return spec, nil
}

func parseOfficeSummaryStats(args map[string]interface{}) ([]officeStat, []string) {
	raw, ok := compatArgValue(args, "summary")
	if !ok || raw == nil {
		return nil, nil
	}
	switch typed := raw.(type) {
	case map[string]interface{}:
		statsRaw, _ := typed["stats"]
		if statsRaw == nil {
			statsRaw, _ = typed["kpis"]
		}
		stats := parseOfficeStats(statsRaw)
		notes := officeStringSliceAny(typed["notes"])
		return stats, notes
	default:
		return nil, nil
	}
}

func parseOfficeSummaryText(args map[string]interface{}) string {
	raw, ok := compatArgValue(args, "summary")
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		for _, key := range []string{"text", "body", "content", "summary"} {
			if value := strings.TrimSpace(anyToStringForLLM(typed[key])); value != "" {
				return value
			}
		}
	}
	return ""
}

func parseOfficeStats(raw interface{}) []officeStat {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	stats := make([]officeStat, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		label := strings.TrimSpace(anyToStringForLLM(m["label"]))
		value := strings.TrimSpace(anyToStringForLLM(m["value"]))
		if label == "" || value == "" {
			continue
		}
		stats = append(stats, officeStat{
			Label: label,
			Value: value,
			Tone:  strings.TrimSpace(anyToStringForLLM(m["tone"])),
		})
	}
	return stats
}

func parseOfficeSingleSheet(args map[string]interface{}) (officeSheetSpec, error) {
	name := strings.TrimSpace(firstCompatString(args, "sheet_name", "sheetName", "name"))
	if name == "" {
		name = "Sheet1"
	}
	columns, rows, err := parseOfficeGrid(args)
	if err != nil {
		return officeSheetSpec{}, err
	}
	return officeSheetSpec{
		Name:    name,
		Columns: columns,
		Rows:    rows,
		Freeze:  strings.TrimSpace(firstCompatString(args, "freeze")),
		Filter:  compatBoolArgDefault(args, "filter", true),
	}, nil
}

func parseOfficeSheetArray(raw interface{}) ([]officeSheetSpec, error) {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil, errors.New("sheets must be a non-empty array")
	}
	sheets := make([]officeSheetSpec, 0, len(items))
	for idx, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("sheet %d must be an object", idx+1)
		}
		name := strings.TrimSpace(anyToStringForLLM(m["name"]))
		if name == "" {
			name = fmt.Sprintf("Sheet%d", idx+1)
		}
		columns, rows, err := parseOfficeGrid(m)
		if err != nil {
			return nil, fmt.Errorf("sheet %s: %w", name, err)
		}
		sheet := officeSheetSpec{
			Name:    name,
			Columns: columns,
			Rows:    rows,
			Freeze:  strings.TrimSpace(anyToStringForLLM(m["freeze"])),
			Filter:  compatBoolValueDefault(m["filter"], true),
		}
		sheets = append(sheets, sheet)
	}
	return sheets, nil
}

func parseOfficeGrid(values map[string]interface{}) ([]officeColumnSpec, [][]interface{}, error) {
	var columns []officeColumnSpec
	if raw, ok := compatArgValue(values, "columns"); ok {
		parsed, err := parseOfficeColumns(raw)
		if err != nil {
			return nil, nil, err
		}
		columns = parsed
	} else {
		headers := officeStringSliceArg(values, "headers")
		for _, header := range headers {
			columns = append(columns, officeColumnSpec{Header: header, Key: header})
		}
	}

	rawRows, ok := compatArgValue(values, "rows")
	if !ok {
		if rawTable, ok := compatArgValue(values, "table"); ok {
			tableMap, _ := rawTable.(map[string]interface{})
			if len(tableMap) > 0 {
				if len(columns) == 0 {
					parsed, err := parseOfficeColumns(tableMap["columns"])
					if err == nil && len(parsed) > 0 {
						columns = parsed
					}
					if len(columns) == 0 {
						headers := officeStringSliceAny(tableMap["headers"])
						for _, header := range headers {
							columns = append(columns, officeColumnSpec{Header: header, Key: header})
						}
					}
				}
				rawRows = tableMap["rows"]
				ok = rawRows != nil
			}
		}
	}
	if !ok || rawRows == nil {
		return nil, nil, errors.New("rows are required for xlsx output")
	}
	return normalizeOfficeRows(columns, rawRows)
}

func parseOfficeColumns(raw interface{}) ([]officeColumnSpec, error) {
	items, ok := raw.([]interface{})
	if !ok {
		return nil, nil
	}
	columns := make([]officeColumnSpec, 0, len(items))
	for idx, item := range items {
		switch typed := item.(type) {
		case string:
			header := strings.TrimSpace(typed)
			if header == "" {
				continue
			}
			columns = append(columns, officeColumnSpec{Header: header, Key: header})
		case map[string]interface{}:
			header := strings.TrimSpace(anyToStringForLLM(typed["header"]))
			key := strings.TrimSpace(anyToStringForLLM(typed["key"]))
			if header == "" {
				header = strings.TrimSpace(anyToStringForLLM(typed["label"]))
			}
			if header == "" {
				header = key
			}
			if header == "" {
				return nil, fmt.Errorf("column %d requires header or key", idx+1)
			}
			if key == "" {
				key = header
			}
			columns = append(columns, officeColumnSpec{
				Header: header,
				Key:    key,
				Width:  officeNumberValue(typed["width"]),
				Kind:   strings.TrimSpace(anyToStringForLLM(typed["kind"])),
			})
		default:
			return nil, fmt.Errorf("column %d must be a string or object", idx+1)
		}
	}
	return columns, nil
}

func normalizeOfficeRows(columns []officeColumnSpec, raw interface{}) ([]officeColumnSpec, [][]interface{}, error) {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return columns, nil, errors.New("rows must be a non-empty array")
	}
	rows := make([][]interface{}, 0, len(items))

	switch items[0].(type) {
	case []interface{}:
		maxCols := 0
		for _, item := range items {
			row, ok := item.([]interface{})
			if !ok {
				return columns, nil, errors.New("rows must all use the same shape")
			}
			copied := make([]interface{}, len(row))
			copy(copied, row)
			rows = append(rows, copied)
			if len(copied) > maxCols {
				maxCols = len(copied)
			}
		}
		if len(columns) == 0 {
			for i := 0; i < maxCols; i++ {
				columns = append(columns, officeColumnSpec{
					Header: fmt.Sprintf("Column %d", i+1),
					Key:    fmt.Sprintf("col_%d", i+1),
				})
			}
		}
	case map[string]interface{}:
		keys := make([]string, 0, len(columns))
		if len(columns) > 0 {
			for _, column := range columns {
				keys = append(keys, column.Key)
			}
		}
		if len(keys) == 0 {
			first, _ := items[0].(map[string]interface{})
			for key := range first {
				keys = append(keys, key)
			}
			sortStringsCaseInsensitive(keys)
			for _, key := range keys {
				columns = append(columns, officeColumnSpec{Header: key, Key: key})
			}
		}
		for _, item := range items {
			rowMap, ok := item.(map[string]interface{})
			if !ok {
				return columns, nil, errors.New("rows must all use the same shape")
			}
			row := make([]interface{}, len(keys))
			for idx, key := range keys {
				row[idx] = rowMap[key]
			}
			rows = append(rows, row)
		}
	default:
		return columns, nil, errors.New("rows must contain arrays or objects")
	}

	if len(columns) == 0 {
		return nil, nil, errors.New("could not infer spreadsheet columns")
	}
	for i := range columns {
		if strings.TrimSpace(columns[i].Kind) == "" {
			columns[i].Kind = inferOfficeColumnKind(columns[i], rows, i)
		}
	}
	return columns, rows, nil
}

func parseOfficeDocSections(raw interface{}) ([]officeDocSection, error) {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil, errors.New("sections must be a non-empty array")
	}
	sections := make([]officeDocSection, 0, len(items))
	for idx, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("section %d must be an object", idx+1)
		}
		section := officeDocSection{
			Heading: strings.TrimSpace(anyToStringForLLM(m["heading"])),
			Bullets: officeStringSliceAny(firstMapValue(m, "bullets", "items", "list")),
		}
		if section.Heading == "" {
			section.Heading = strings.TrimSpace(anyToStringForLLM(m["title"]))
		}
		section.Paragraphs = append(section.Paragraphs, officeStringSliceAny(m["paragraphs"])...)
		if len(section.Paragraphs) == 0 {
			body := strings.TrimSpace(anyToStringForLLM(firstMapValue(m, "body", "text", "content")))
			if body != "" {
				section.Paragraphs = append(section.Paragraphs, splitOfficeParagraphs(body)...)
			}
		}
		if rawTable, ok := m["table"]; ok {
			table, err := parseOfficeTable(rawTable)
			if err != nil {
				return nil, fmt.Errorf("section %d: %w", idx+1, err)
			}
			section.Table = table
		}
		if rawChart, ok := m["chart"]; ok {
			chart, err := parseOfficeChart(rawChart)
			if err != nil {
				return nil, fmt.Errorf("section %d: %w", idx+1, err)
			}
			section.Chart = chart
		}
		sections = append(sections, section)
	}
	return sections, nil
}

func parseOfficeTable(raw interface{}) (*officeTableSpec, error) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, errors.New("table must be an object")
	}
	headers := officeStringSliceAny(firstMapValue(m, "headers"))
	columnKeys := []string(nil)
	columnWidths := []float64(nil)
	columnAlignments := []string(nil)
	columnFormats := []string(nil)
	if rawColumns, ok := compatArgValue(m, "columns"); ok && rawColumns != nil {
		parsedHeaders, parsedKeys, parsedWidths, parsedAlignments, parsedFormats, err := parseOfficeTableColumns(rawColumns)
		if err != nil {
			return nil, err
		}
		if len(parsedHeaders) > 0 {
			headers = parsedHeaders
		}
		columnKeys = parsedKeys
		columnWidths = parsedWidths
		columnAlignments = parsedAlignments
		columnFormats = parsedFormats
	}
	rowsRaw, ok := m["rows"].([]interface{})
	if !ok {
		return nil, errors.New("table rows must be an array")
	}
	rows := make([][]string, 0, len(rowsRaw))
	for _, item := range rowsRaw {
		switch typed := item.(type) {
		case []interface{}:
			row := make([]string, len(typed))
			for idx, value := range typed {
				row[idx] = officeFormatTableDisplayValue(value, officeTableColumnFormat(columnFormats, idx))
			}
			rows = append(rows, row)
		case map[string]interface{}:
			if len(headers) == 0 {
				return nil, errors.New("table rows with object values require headers or columns")
			}
			if len(columnKeys) == 0 {
				columnKeys = append([]string(nil), headers...)
			}
			row := make([]string, len(columnKeys))
			for idx, key := range columnKeys {
				row[idx] = officeFormatTableDisplayValue(typed[key], officeTableColumnFormat(columnFormats, idx))
			}
			rows = append(rows, row)
		default:
			return nil, errors.New("table rows must contain arrays or objects")
		}
	}
	if len(headers) == 0 && len(rows) > 0 {
		headers = make([]string, len(rows[0]))
		for i := range headers {
			headers[i] = fmt.Sprintf("Column %d", i+1)
		}
	}
	return &officeTableSpec{Headers: headers, Rows: rows, ColumnWidths: columnWidths, ColumnAlignments: columnAlignments}, nil
}

func parseOfficeTableColumns(raw interface{}) ([]string, []string, []float64, []string, []string, error) {
	items, ok := raw.([]interface{})
	if !ok {
		return nil, nil, nil, nil, nil, errors.New("table columns must be an array")
	}
	headers := make([]string, 0, len(items))
	keys := make([]string, 0, len(items))
	widths := make([]float64, 0, len(items))
	alignments := make([]string, 0, len(items))
	formats := make([]string, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case map[string]interface{}:
			header := strings.TrimSpace(anyToStringForLLM(firstMapValue(typed, "header", "title", "name")))
			key := strings.TrimSpace(anyToStringForLLM(firstMapValue(typed, "key", "field")))
			rawFormat := anyToStringForLLM(firstMapValue(typed, "format", "kind", "type"))
			if header == "" {
				header = key
			}
			if key == "" {
				key = header
			}
			if header == "" {
				return nil, nil, nil, nil, nil, errors.New("table columns must define header or key")
			}
			headers = append(headers, header)
			keys = append(keys, key)
			widths = append(widths, officePositiveFloatValue(firstMapValue(typed, "width", "ratio", "weight")))
			alignments = append(alignments, officeNormalizeTableAlignment(
				anyToStringForLLM(firstMapValue(typed, "align", "alignment", "text_align", "textAlignment")),
				rawFormat,
			))
			formats = append(formats, officeNormalizeTableDisplayFormat(rawFormat))
		default:
			value := strings.TrimSpace(anyToStringForLLM(item))
			if value == "" {
				return nil, nil, nil, nil, nil, errors.New("table columns must not be empty")
			}
			headers = append(headers, value)
			keys = append(keys, value)
			widths = append(widths, 0)
			alignments = append(alignments, "")
			formats = append(formats, "")
		}
	}
	return headers, keys, widths, alignments, formats, nil
}

func officePositiveFloatValue(raw interface{}) float64 {
	switch typed := raw.(type) {
	case float64:
		if typed > 0 {
			return typed
		}
	case float32:
		if typed > 0 {
			return float64(typed)
		}
	case int:
		if typed > 0 {
			return float64(typed)
		}
	case int64:
		if typed > 0 {
			return float64(typed)
		}
	case json.Number:
		if value, err := typed.Float64(); err == nil && value > 0 {
			return value
		}
	}
	return 0
}

func officeNormalizeTableAlignment(rawAlign, rawKind string) string {
	align := strings.ToLower(strings.TrimSpace(rawAlign))
	switch align {
	case "left", "l", "start":
		return "left"
	case "center", "centre", "middle", "c", "ctr":
		return "center"
	case "right", "r", "end":
		return "right"
	}

	kind := strings.ToLower(strings.TrimSpace(rawKind))
	switch kind {
	case "integer", "int", "number", "numeric", "decimal", "currency", "percent":
		return "right"
	default:
		return ""
	}
}

func officeNormalizeTableDisplayFormat(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "integer", "int":
		return "integer"
	case "decimal", "number", "numeric":
		return "decimal"
	case "currency", "money":
		return "currency"
	case "percent", "percentage":
		return "percent"
	case "date":
		return "date"
	case "time", "datetime", "date-time", "date_time", "timestamp":
		return "datetime"
	default:
		return ""
	}
}

func officeTableColumnFormat(formats []string, columnIndex int) string {
	if columnIndex < 0 || columnIndex >= len(formats) {
		return ""
	}
	return formats[columnIndex]
}

func officeFormatTableDisplayValue(raw interface{}, format string) string {
	text := strings.TrimSpace(anyToStringForLLM(raw))
	if text == "" {
		return ""
	}

	switch officeNormalizeTableDisplayFormat(format) {
	case "date":
		if ts, ok := officeParseTableTime(raw); ok {
			return ts.UTC().Format("2006-01-02")
		}
		return text
	case "datetime":
		if ts, ok := officeParseTableTime(raw); ok {
			return ts.UTC().Format("2006-01-02 15:04")
		}
		return text
	}

	value, ok := officeChartNumber(raw)
	if !ok {
		return text
	}

	switch officeNormalizeTableDisplayFormat(format) {
	case "integer":
		return strconv.FormatFloat(math.Round(value), 'f', 0, 64)
	case "decimal":
		return strconv.FormatFloat(value, 'f', 2, 64)
	case "currency":
		if value < 0 {
			return "-$" + strconv.FormatFloat(math.Abs(value), 'f', 2, 64)
		}
		return "$" + strconv.FormatFloat(value, 'f', 2, 64)
	case "percent":
		return strconv.FormatFloat(value*100, 'f', 2, 64) + "%"
	default:
		return text
	}
}

func officeParseTableTime(raw interface{}) (time.Time, bool) {
	switch typed := raw.(type) {
	case time.Time:
		return typed, true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return time.Time{}, false
		}
		layouts := []string{
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
			"2006-01-02",
		}
		for _, layout := range layouts {
			ts, err := time.Parse(layout, text)
			if err == nil {
				return ts, true
			}
		}
	}
	return time.Time{}, false
}

func parseOfficeChart(raw interface{}) (*officeChartSpec, error) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, errors.New("chart must be an object")
	}

	chartType := officeNormalizeChartType(anyToStringForLLM(firstMapValue(m, "type", "chart_type", "kind")))
	if chartType == "" {
		return nil, errors.New("chart type must be one of: bar, column, stacked_bar, stacked_column, percent_stacked_bar, percent_stacked_column, line, pie, donut, combo")
	}

	categories := officeStringSliceAny(firstMapValue(m, "categories", "labels"))
	seriesRaw, ok := m["series"].([]interface{})
	if !ok || len(seriesRaw) == 0 {
		return nil, errors.New("chart series must be a non-empty array")
	}
	series := make([]officeChartSeries, 0, len(seriesRaw))
	maxLen := 0
	comboBars := 0
	comboLines := 0
	for idx, item := range seriesRaw {
		seriesItem, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("chart series %d must be an object", idx+1)
		}
		valuesRaw, ok := seriesItem["values"].([]interface{})
		if !ok || len(valuesRaw) == 0 {
			return nil, fmt.Errorf("chart series %d values must be a non-empty array", idx+1)
		}
		values := make([]float64, 0, len(valuesRaw))
		for valueIndex, rawValue := range valuesRaw {
			value, ok := officeChartNumber(rawValue)
			if !ok {
				return nil, fmt.Errorf("chart series %d value %d must be numeric", idx+1, valueIndex+1)
			}
			values = append(values, value)
		}
		if len(values) > maxLen {
			maxLen = len(values)
		}
		seriesType := officeNormalizeChartSeriesType(chartType, anyToStringForLLM(firstMapValue(seriesItem, "type", "chart_type", "render_as", "renderAs")), idx)
		seriesAxis := officeNormalizeChartSeriesAxis(chartType, seriesType, anyToStringForLLM(firstMapValue(seriesItem, "axis", "y_axis", "yAxis", "value_axis", "valueAxis")))
		if chartType == "combo" {
			switch seriesType {
			case "bar":
				comboBars++
			case "line":
				comboLines++
			default:
				return nil, fmt.Errorf("chart series %d type must be bar or line for combo charts", idx+1)
			}
		}
		series = append(series, officeChartSeries{
			Name:                 firstNonEmptyOfficeString(strings.TrimSpace(anyToStringForLLM(firstMapValue(seriesItem, "name", "label"))), fmt.Sprintf("Series %d", idx+1)),
			Type:                 seriesType,
			Axis:                 seriesAxis,
			Labels:               officeNormalizeChartLabels(firstMapValue(seriesItem, "labels", "data_labels", "show_labels", "showLabels")),
			LabelPosition:        officeNormalizeChartLabelPosition(anyToStringForLLM(firstMapValue(seriesItem, "label_position", "labelPosition", "labels_position", "data_label_position", "dataLabelPosition"))),
			LabelFormat:          officeNormalizeChartLabelFormat(anyToStringForLLM(firstMapValue(seriesItem, "label_format", "labelFormat", "labels_format", "data_label_format", "dataLabelFormat"))),
			LabelSeparator:       officeNormalizeChartLabelSeparator(anyToStringForLLM(firstMapValue(seriesItem, "label_separator", "labelSeparator", "data_label_separator", "dataLabelSeparator"))),
			ShowLeaderLines:      officeNormalizeChartLabels(firstMapValue(seriesItem, "show_leader_lines", "showLeaderLines", "label_leader_lines", "labelLeaderLines")),
			ShowValue:            officeNormalizeChartLabels(firstMapValue(seriesItem, "show_value", "showValue", "label_value", "labelValue")),
			ShowCategory:         officeNormalizeChartLabels(firstMapValue(seriesItem, "show_category", "showCategory", "label_category", "labelCategory")),
			ShowSeriesName:       officeNormalizeChartLabels(firstMapValue(seriesItem, "show_series_name", "showSeriesName", "label_series_name", "labelSeriesName")),
			ShowPercent:          officeNormalizeChartLabels(firstMapValue(seriesItem, "show_percent", "showPercent", "label_percent", "labelPercent")),
			ShowLegendKey:        officeNormalizeChartLabels(firstMapValue(seriesItem, "show_legend_key", "showLegendKey", "label_legend_key", "labelLegendKey")),
			ShowBubbleSize:       officeNormalizeChartLabels(firstMapValue(seriesItem, "show_bubble_size", "showBubbleSize", "label_bubble_size", "labelBubbleSize")),
			PointShowLabels:      officeNormalizeChartPointLabelVisibility(firstMapValue(seriesItem, "point_show_labels", "pointShowLabels", "slice_show_labels", "sliceShowLabels")),
			PointShowValues:      officeNormalizeChartPointLabelVisibility(firstMapValue(seriesItem, "point_show_values", "pointShowValues", "slice_show_values", "sliceShowValues")),
			PointShowCategories:  officeNormalizeChartPointLabelVisibility(firstMapValue(seriesItem, "point_show_categories", "pointShowCategories", "slice_show_categories", "sliceShowCategories")),
			PointShowSeriesNames: officeNormalizeChartPointLabelVisibility(firstMapValue(seriesItem, "point_show_series_names", "pointShowSeriesNames", "point_show_series_name", "pointShowSeriesName", "slice_show_series_names", "sliceShowSeriesNames", "slice_show_series_name", "sliceShowSeriesName")),
			PointShowPercents:    officeNormalizeChartPointLabelVisibility(firstMapValue(seriesItem, "point_show_percents", "pointShowPercents", "point_show_percent", "pointShowPercent", "slice_show_percents", "sliceShowPercents", "slice_show_percent", "sliceShowPercent")),
			PointLabelPositions:  officeNormalizeChartPointLabelPositions(firstMapValue(seriesItem, "point_label_positions", "pointLabelPositions", "slice_label_positions", "sliceLabelPositions")),
			PointLabelFormats:    officeNormalizeChartPointLabelFormats(firstMapValue(seriesItem, "point_label_formats", "pointLabelFormats", "slice_label_formats", "sliceLabelFormats")),
			PointLabelSeparators: officeNormalizeChartPointLabelSeparators(firstMapValue(seriesItem, "point_label_separators", "pointLabelSeparators", "slice_label_separators", "sliceLabelSeparators")),
			PointColors:          officeNormalizeChartPointColors(firstMapValue(seriesItem, "point_colors", "pointColors", "slice_colors", "sliceColors")),
			PointExplosions:      officeNormalizeChartPointExplosions(firstMapValue(seriesItem, "point_explosions", "pointExplosions", "slice_explosions", "sliceExplosions")),
			Smooth:               officeNormalizeChartLabels(firstMapValue(seriesItem, "smooth", "smoothed", "smooth_lines", "smoothLines")),
			Color:                officeNormalizeChartSeriesColor(anyToStringForLLM(firstMapValue(seriesItem, "color", "colour", "hex"))),
			LineWidth:            officeNormalizeChartSeriesLineWidth(firstMapValue(seriesItem, "line_width", "lineWidth", "stroke_width", "strokeWidth")),
			Dash:                 officeNormalizeChartSeriesDash(anyToStringForLLM(firstMapValue(seriesItem, "dash", "dash_style", "dashStyle"))),
			Marker:               officeNormalizeChartSeriesMarker(anyToStringForLLM(firstMapValue(seriesItem, "marker", "marker_style", "markerStyle"))),
			Values:               values,
		})
	}
	if chartType == "combo" && (comboBars == 0 || comboLines == 0) {
		return nil, errors.New("combo chart requires at least one bar series and one line series")
	}

	if len(categories) == 0 {
		categories = make([]string, maxLen)
		for idx := range categories {
			categories[idx] = fmt.Sprintf("Category %d", idx+1)
		}
	}
	for len(categories) < maxLen {
		categories = append(categories, fmt.Sprintf("Category %d", len(categories)+1))
	}
	if len(categories) == 0 {
		return nil, errors.New("chart categories are required")
	}
	categoryAxisType := officeNormalizeChartCategoryAxisType(anyToStringForLLM(firstMapValue(m, "category_axis_type", "categoryAxisType", "x_axis_type", "xAxisType")))
	if categoryAxisType == "date" || categoryAxisType == "datetime" {
		withTime := categoryAxisType == "datetime"
		switch chartType {
		case "pie", "donut":
			return nil, fmt.Errorf("%s category axis is not yet supported for %s charts", categoryAxisType, chartType)
		}
		for idx, category := range categories {
			if _, ok := officeExcelDateSerial(category, withTime); !ok {
				return nil, fmt.Errorf("chart category %d must be an ISO-like %s when x_axis_type is %s", idx+1, map[bool]string{true: "date-time", false: "date"}[withTime], categoryAxisType)
			}
		}
	}
	categoryAxisMinRaw := firstMapValue(m, "category_axis_min", "categoryAxisMin", "x_axis_min", "xAxisMin")
	categoryAxisMaxRaw := firstMapValue(m, "category_axis_max", "categoryAxisMax", "x_axis_max", "xAxisMax")
	var categoryAxisMin *float64
	var categoryAxisMax *float64
	if categoryAxisType == "date" || categoryAxisType == "datetime" {
		withTime := categoryAxisType == "datetime"
		var err error
		categoryAxisMin, err = officeParseChartDateAxisBound(categoryAxisMinRaw, "x_axis_min", withTime)
		if err != nil {
			return nil, err
		}
		categoryAxisMax, err = officeParseChartDateAxisBound(categoryAxisMaxRaw, "x_axis_max", withTime)
		if err != nil {
			return nil, err
		}
		if categoryAxisMin != nil && categoryAxisMax != nil && *categoryAxisMin > *categoryAxisMax {
			return nil, fmt.Errorf("x_axis_min must be less than or equal to x_axis_max when x_axis_type is %s", categoryAxisType)
		}
	}
	for idx := range series {
		for len(series[idx].Values) < len(categories) {
			series[idx].Values = append(series[idx].Values, 0)
		}
	}

	return &officeChartSpec{
		Type:                                  chartType,
		Title:                                 strings.TrimSpace(anyToStringForLLM(firstMapValue(m, "title", "name"))),
		CategoryAxisTitle:                     strings.TrimSpace(anyToStringForLLM(firstMapValue(m, "category_axis_title", "categoryAxisTitle", "x_axis_title", "xAxisTitle"))),
		SecondaryCategoryAxisTitle:            strings.TrimSpace(anyToStringForLLM(firstMapValue(m, "secondary_category_axis_title", "secondaryCategoryAxisTitle", "secondary_x_axis_title", "secondaryXAxisTitle", "x2_axis_title", "x2AxisTitle"))),
		CategoryAxisType:                      categoryAxisType,
		CategoryAxisLabelPosition:             officeNormalizeChartAxisLabelPosition(anyToStringForLLM(firstMapValue(m, "category_axis_label_position", "categoryAxisLabelPosition", "x_axis_label_position", "xAxisLabelPosition"))),
		SecondaryCategoryAxisLabelPosition:    officeNormalizeChartAxisLabelPosition(anyToStringForLLM(firstMapValue(m, "secondary_category_axis_label_position", "secondaryCategoryAxisLabelPosition", "secondary_x_axis_label_position", "secondaryXAxisLabelPosition", "x2_axis_label_position", "x2AxisLabelPosition"))),
		CategoryAxisReverseOrder:              officeNormalizeChartAxisReverseOrder(firstMapValue(m, "category_axis_reverse_order", "categoryAxisReverseOrder", "x_axis_reverse_order", "xAxisReverseOrder")),
		SecondaryCategoryAxisReverseOrder:     officeNormalizeChartAxisReverseOrder(firstMapValue(m, "secondary_category_axis_reverse_order", "secondaryCategoryAxisReverseOrder", "secondary_x_axis_reverse_order", "secondaryXAxisReverseOrder", "x2_axis_reverse_order", "x2AxisReverseOrder")),
		CategoryAxisCrosses:                   officeNormalizeChartAxisCrosses(anyToStringForLLM(firstMapValue(m, "category_axis_crosses", "categoryAxisCrosses", "x_axis_crosses", "xAxisCrosses"))),
		SecondaryCategoryAxisCrosses:          officeNormalizeChartAxisCrosses(anyToStringForLLM(firstMapValue(m, "secondary_category_axis_crosses", "secondaryCategoryAxisCrosses", "secondary_x_axis_crosses", "secondaryXAxisCrosses", "x2_axis_crosses", "x2AxisCrosses"))),
		CategoryAxisMajorTickMark:             officeNormalizeChartTickMark(anyToStringForLLM(firstMapValue(m, "category_axis_major_tick_mark", "categoryAxisMajorTickMark", "x_axis_major_tick_mark", "xAxisMajorTickMark"))),
		CategoryAxisMinorTickMark:             officeNormalizeChartTickMark(anyToStringForLLM(firstMapValue(m, "category_axis_minor_tick_mark", "categoryAxisMinorTickMark", "x_axis_minor_tick_mark", "xAxisMinorTickMark"))),
		SecondaryCategoryAxisMajorTickMark:    officeNormalizeChartTickMark(anyToStringForLLM(firstMapValue(m, "secondary_category_axis_major_tick_mark", "secondaryCategoryAxisMajorTickMark", "secondary_x_axis_major_tick_mark", "secondaryXAxisMajorTickMark", "x2_axis_major_tick_mark", "x2AxisMajorTickMark"))),
		SecondaryCategoryAxisMinorTickMark:    officeNormalizeChartTickMark(anyToStringForLLM(firstMapValue(m, "secondary_category_axis_minor_tick_mark", "secondaryCategoryAxisMinorTickMark", "secondary_x_axis_minor_tick_mark", "secondaryXAxisMinorTickMark", "x2_axis_minor_tick_mark", "x2AxisMinorTickMark"))),
		CategoryAxisLabelAlignment:            officeNormalizeChartAxisLabelAlignment(anyToStringForLLM(firstMapValue(m, "category_axis_label_alignment", "categoryAxisLabelAlignment", "x_axis_label_alignment", "xAxisLabelAlignment"))),
		SecondaryCategoryAxisLabelAlignment:   officeNormalizeChartAxisLabelAlignment(anyToStringForLLM(firstMapValue(m, "secondary_category_axis_label_alignment", "secondaryCategoryAxisLabelAlignment", "secondary_x_axis_label_alignment", "secondaryXAxisLabelAlignment", "x2_axis_label_alignment", "x2AxisLabelAlignment"))),
		CategoryAxisLabelOffset:               officeNormalizeChartAxisLabelOffset(firstMapValue(m, "category_axis_label_offset", "categoryAxisLabelOffset", "x_axis_label_offset", "xAxisLabelOffset")),
		SecondaryCategoryAxisLabelOffset:      officeNormalizeChartAxisLabelOffset(firstMapValue(m, "secondary_category_axis_label_offset", "secondaryCategoryAxisLabelOffset", "secondary_x_axis_label_offset", "secondaryXAxisLabelOffset", "x2_axis_label_offset", "x2AxisLabelOffset")),
		CategoryAxisMultiLevelLabels:          officeNormalizeChartLabels(firstMapValue(m, "category_axis_multi_level_labels", "categoryAxisMultiLevelLabels", "category_axis_multilevel_labels", "categoryAxisMultilevelLabels", "x_axis_multi_level_labels", "xAxisMultiLevelLabels", "x_axis_multilevel_labels", "xAxisMultilevelLabels")),
		SecondaryCategoryAxisMultiLevelLabels: officeNormalizeChartLabels(firstMapValue(m, "secondary_category_axis_multi_level_labels", "secondaryCategoryAxisMultiLevelLabels", "secondary_category_axis_multilevel_labels", "secondaryCategoryAxisMultilevelLabels", "secondary_x_axis_multi_level_labels", "secondaryXAxisMultiLevelLabels", "secondary_x_axis_multilevel_labels", "secondaryXAxisMultilevelLabels", "x2_axis_multi_level_labels", "x2AxisMultiLevelLabels", "x2_axis_multilevel_labels", "x2AxisMultilevelLabels")),
		CategoryAxisVisible:                   officeNormalizeChartLabels(firstMapValue(m, "category_axis_visible", "categoryAxisVisible", "show_category_axis", "showCategoryAxis", "x_axis_visible", "xAxisVisible")),
		SecondaryCategoryAxisVisible:          officeNormalizeChartLabels(firstMapValue(m, "secondary_category_axis_visible", "secondaryCategoryAxisVisible", "show_secondary_category_axis", "showSecondaryCategoryAxis", "secondary_x_axis_visible", "secondaryXAxisVisible", "x2_axis_visible", "x2AxisVisible")),
		CategoryAxisAuto:                      officeNormalizeChartLabels(firstMapValue(m, "category_axis_auto", "categoryAxisAuto", "x_axis_auto", "xAxisAuto")),
		SecondaryCategoryAxisAuto:             officeNormalizeChartLabels(firstMapValue(m, "secondary_category_axis_auto", "secondaryCategoryAxisAuto", "secondary_x_axis_auto", "secondaryXAxisAuto", "x2_axis_auto", "x2AxisAuto")),
		CategoryAxisFormat:                    officeNormalizeChartAxisFormat(anyToStringForLLM(firstMapValue(m, "category_axis_format", "categoryAxisFormat", "x_axis_format", "xAxisFormat"))),
		SecondaryCategoryAxisFormat:           officeNormalizeChartAxisFormat(anyToStringForLLM(firstMapValue(m, "secondary_category_axis_format", "secondaryCategoryAxisFormat", "secondary_x_axis_format", "secondaryXAxisFormat", "x2_axis_format", "x2AxisFormat"))),
		CategoryAxisMin:                       categoryAxisMin,
		CategoryAxisMax:                       categoryAxisMax,
		CategoryAxisBaseTimeUnit:              officeNormalizeChartDateAxisTimeUnit(anyToStringForLLM(firstMapValue(m, "category_axis_base_time_unit", "categoryAxisBaseTimeUnit", "x_axis_base_time_unit", "xAxisBaseTimeUnit"))),
		CategoryAxisMajorUnit:                 officeNormalizeChartAxisUnit(firstMapValue(m, "category_axis_major_unit", "categoryAxisMajorUnit", "x_axis_major_unit", "xAxisMajorUnit")),
		CategoryAxisMinorUnit:                 officeNormalizeChartAxisUnit(firstMapValue(m, "category_axis_minor_unit", "categoryAxisMinorUnit", "x_axis_minor_unit", "xAxisMinorUnit")),
		CategoryAxisMajorTimeUnit:             officeNormalizeChartDateAxisTimeUnit(anyToStringForLLM(firstMapValue(m, "category_axis_major_time_unit", "categoryAxisMajorTimeUnit", "x_axis_major_time_unit", "xAxisMajorTimeUnit"))),
		CategoryAxisMinorTimeUnit:             officeNormalizeChartDateAxisTimeUnit(anyToStringForLLM(firstMapValue(m, "category_axis_minor_time_unit", "categoryAxisMinorTimeUnit", "x_axis_minor_time_unit", "xAxisMinorTimeUnit"))),
		ValueAxisTitle:                        strings.TrimSpace(anyToStringForLLM(firstMapValue(m, "value_axis_title", "valueAxisTitle", "y_axis_title", "yAxisTitle"))),
		SecondaryValueAxisTitle:               strings.TrimSpace(anyToStringForLLM(firstMapValue(m, "secondary_value_axis_title", "secondaryValueAxisTitle", "secondary_y_axis_title", "secondaryYAxisTitle", "y2_axis_title", "y2AxisTitle"))),
		ValueAxisFormat:                       officeNormalizeChartAxisFormat(anyToStringForLLM(firstMapValue(m, "value_axis_format", "valueAxisFormat", "y_axis_format", "yAxisFormat"))),
		SecondaryValueAxisFormat:              officeNormalizeChartAxisFormat(anyToStringForLLM(firstMapValue(m, "secondary_value_axis_format", "secondaryValueAxisFormat", "secondary_y_axis_format", "secondaryYAxisFormat", "y2_axis_format", "y2AxisFormat"))),
		ValueAxisMin:                          officeNormalizeChartAxisBound(firstMapValue(m, "value_axis_min", "valueAxisMin", "y_axis_min", "yAxisMin")),
		ValueAxisMax:                          officeNormalizeChartAxisBound(firstMapValue(m, "value_axis_max", "valueAxisMax", "y_axis_max", "yAxisMax")),
		SecondaryValueAxisMin:                 officeNormalizeChartAxisBound(firstMapValue(m, "secondary_value_axis_min", "secondaryValueAxisMin", "secondary_y_axis_min", "secondaryYAxisMin", "y2_axis_min", "y2AxisMin")),
		SecondaryValueAxisMax:                 officeNormalizeChartAxisBound(firstMapValue(m, "secondary_value_axis_max", "secondaryValueAxisMax", "secondary_y_axis_max", "secondaryYAxisMax", "y2_axis_max", "y2AxisMax")),
		ValueAxisMajorUnit:                    officeNormalizeChartAxisUnit(firstMapValue(m, "value_axis_major_unit", "valueAxisMajorUnit", "y_axis_major_unit", "yAxisMajorUnit")),
		ValueAxisMinorUnit:                    officeNormalizeChartAxisUnit(firstMapValue(m, "value_axis_minor_unit", "valueAxisMinorUnit", "y_axis_minor_unit", "yAxisMinorUnit")),
		SecondaryValueAxisMajorUnit:           officeNormalizeChartAxisUnit(firstMapValue(m, "secondary_value_axis_major_unit", "secondaryValueAxisMajorUnit", "secondary_y_axis_major_unit", "secondaryYAxisMajorUnit", "y2_axis_major_unit", "y2AxisMajorUnit")),
		SecondaryValueAxisMinorUnit:           officeNormalizeChartAxisUnit(firstMapValue(m, "secondary_value_axis_minor_unit", "secondaryValueAxisMinorUnit", "secondary_y_axis_minor_unit", "secondaryYAxisMinorUnit", "y2_axis_minor_unit", "y2AxisMinorUnit")),
		ValueAxisMajorGridlines:               officeNormalizeChartLabels(firstMapValue(m, "value_axis_major_gridlines", "valueAxisMajorGridlines", "y_axis_major_gridlines", "yAxisMajorGridlines")),
		ValueAxisMinorGridlines:               officeNormalizeChartLabels(firstMapValue(m, "value_axis_minor_gridlines", "valueAxisMinorGridlines", "y_axis_minor_gridlines", "yAxisMinorGridlines")),
		SecondaryValueAxisMajorGridlines:      officeNormalizeChartLabels(firstMapValue(m, "secondary_value_axis_major_gridlines", "secondaryValueAxisMajorGridlines", "secondary_y_axis_major_gridlines", "secondaryYAxisMajorGridlines", "y2_axis_major_gridlines", "y2AxisMajorGridlines")),
		SecondaryValueAxisMinorGridlines:      officeNormalizeChartLabels(firstMapValue(m, "secondary_value_axis_minor_gridlines", "secondaryValueAxisMinorGridlines", "secondary_y_axis_minor_gridlines", "secondaryYAxisMinorGridlines", "y2_axis_minor_gridlines", "y2AxisMinorGridlines")),
		ValueAxisCrosses:                      officeNormalizeChartAxisCrosses(anyToStringForLLM(firstMapValue(m, "value_axis_crosses", "valueAxisCrosses", "y_axis_crosses", "yAxisCrosses"))),
		SecondaryValueAxisCrosses:             officeNormalizeChartAxisCrosses(anyToStringForLLM(firstMapValue(m, "secondary_value_axis_crosses", "secondaryValueAxisCrosses", "secondary_y_axis_crosses", "secondaryYAxisCrosses", "y2_axis_crosses", "y2AxisCrosses"))),
		ValueAxisCrossBetween:                 officeNormalizeChartAxisCrossBetween(anyToStringForLLM(firstMapValue(m, "value_axis_cross_between", "valueAxisCrossBetween", "y_axis_cross_between", "yAxisCrossBetween"))),
		SecondaryValueAxisCrossBetween:        officeNormalizeChartAxisCrossBetween(anyToStringForLLM(firstMapValue(m, "secondary_value_axis_cross_between", "secondaryValueAxisCrossBetween", "secondary_y_axis_cross_between", "secondaryYAxisCrossBetween", "y2_axis_cross_between", "y2AxisCrossBetween"))),
		ValueAxisReverseOrder:                 officeNormalizeChartAxisReverseOrder(firstMapValue(m, "value_axis_reverse_order", "valueAxisReverseOrder", "y_axis_reverse_order", "yAxisReverseOrder")),
		SecondaryValueAxisReverseOrder:        officeNormalizeChartAxisReverseOrder(firstMapValue(m, "secondary_value_axis_reverse_order", "secondaryValueAxisReverseOrder", "secondary_y_axis_reverse_order", "secondaryYAxisReverseOrder", "y2_axis_reverse_order", "y2AxisReverseOrder")),
		ValueAxisLabelPosition:                officeNormalizeChartAxisLabelPosition(anyToStringForLLM(firstMapValue(m, "value_axis_label_position", "valueAxisLabelPosition", "y_axis_label_position", "yAxisLabelPosition"))),
		SecondaryValueAxisLabelPosition:       officeNormalizeChartAxisLabelPosition(anyToStringForLLM(firstMapValue(m, "secondary_value_axis_label_position", "secondaryValueAxisLabelPosition", "secondary_y_axis_label_position", "secondaryYAxisLabelPosition", "y2_axis_label_position", "y2AxisLabelPosition"))),
		ValueAxisMajorTickMark:                officeNormalizeChartTickMark(anyToStringForLLM(firstMapValue(m, "value_axis_major_tick_mark", "valueAxisMajorTickMark", "y_axis_major_tick_mark", "yAxisMajorTickMark"))),
		ValueAxisMinorTickMark:                officeNormalizeChartTickMark(anyToStringForLLM(firstMapValue(m, "value_axis_minor_tick_mark", "valueAxisMinorTickMark", "y_axis_minor_tick_mark", "yAxisMinorTickMark"))),
		SecondaryValueAxisMajorTickMark:       officeNormalizeChartTickMark(anyToStringForLLM(firstMapValue(m, "secondary_value_axis_major_tick_mark", "secondaryValueAxisMajorTickMark", "secondary_y_axis_major_tick_mark", "secondaryYAxisMajorTickMark", "y2_axis_major_tick_mark", "y2AxisMajorTickMark"))),
		SecondaryValueAxisMinorTickMark:       officeNormalizeChartTickMark(anyToStringForLLM(firstMapValue(m, "secondary_value_axis_minor_tick_mark", "secondaryValueAxisMinorTickMark", "secondary_y_axis_minor_tick_mark", "secondaryYAxisMinorTickMark", "y2_axis_minor_tick_mark", "y2AxisMinorTickMark"))),
		ShowLegend:                            officeNormalizeChartLabels(firstMapValue(m, "show_legend", "showLegend")),
		LegendPosition:                        officeNormalizeChartLegendPosition(anyToStringForLLM(firstMapValue(m, "legend_position", "legendPosition", "legend_pos", "legendPos"))),
		VaryColors:                            officeNormalizeChartLabels(firstMapValue(m, "vary_colors", "varyColors")),
		StartAngle:                            officeNormalizeChartStartAngle(firstMapValue(m, "start_angle", "startAngle", "first_slice_angle", "firstSliceAngle")),
		HoleSize:                              officeNormalizeChartHoleSize(firstMapValue(m, "hole_size", "holeSize", "donut_hole_size", "donutHoleSize")),
		Smooth:                                officeNormalizeChartLabels(firstMapValue(m, "smooth", "smoothed", "smooth_lines", "smoothLines", "line_smoothing", "lineSmoothing")),
		GapWidth:                              officeNormalizeChartGapWidth(firstMapValue(m, "gap_width", "gapWidth")),
		Overlap:                               officeNormalizeChartOverlap(firstMapValue(m, "overlap")),
		Labels:                                officeNormalizeChartLabels(firstMapValue(m, "labels", "data_labels", "show_labels", "showLabels")),
		LabelPosition:                         officeNormalizeChartLabelPosition(anyToStringForLLM(firstMapValue(m, "label_position", "labelPosition", "labels_position", "data_label_position", "dataLabelPosition"))),
		LabelFormat:                           officeNormalizeChartLabelFormat(anyToStringForLLM(firstMapValue(m, "label_format", "labelFormat", "labels_format", "data_label_format", "dataLabelFormat"))),
		LabelSeparator:                        officeNormalizeChartLabelSeparator(anyToStringForLLM(firstMapValue(m, "label_separator", "labelSeparator", "data_label_separator", "dataLabelSeparator"))),
		ShowLeaderLines:                       officeNormalizeChartLabels(firstMapValue(m, "show_leader_lines", "showLeaderLines", "label_leader_lines", "labelLeaderLines")),
		ShowValue:                             officeNormalizeChartLabels(firstMapValue(m, "show_value", "showValue", "label_value", "labelValue")),
		ShowCategory:                          officeNormalizeChartLabels(firstMapValue(m, "show_category", "showCategory", "label_category", "labelCategory")),
		ShowSeriesName:                        officeNormalizeChartLabels(firstMapValue(m, "show_series_name", "showSeriesName", "label_series_name", "labelSeriesName")),
		ShowPercent:                           officeNormalizeChartLabels(firstMapValue(m, "show_percent", "showPercent", "label_percent", "labelPercent")),
		ShowLegendKey:                         officeNormalizeChartLabels(firstMapValue(m, "show_legend_key", "showLegendKey", "label_legend_key", "labelLegendKey")),
		ShowBubbleSize:                        officeNormalizeChartLabels(firstMapValue(m, "show_bubble_size", "showBubbleSize", "label_bubble_size", "labelBubbleSize")),
		Categories:                            categories,
		Series:                                series,
	}, nil
}

func officeNormalizeChartType(chartType string) string {
	switch strings.ToLower(strings.TrimSpace(chartType)) {
	case "bar", "column", "col", "bars":
		return "bar"
	case "stacked_bar", "bar_stacked", "horizontal_stacked_bar", "stacked-horizontal-bar":
		return "stacked_bar"
	case "stacked_column", "stacked_col", "column_stacked", "stacked":
		return "stacked_column"
	case "percent_stacked_bar", "100_stacked_bar", "100_percent_stacked_bar", "percent-bar":
		return "percent_stacked_bar"
	case "percent_stacked_column", "percent_stacked_col", "100_stacked_column", "100_percent_stacked_column", "percent_stacked":
		return "percent_stacked_column"
	case "line", "trend":
		return "line"
	case "pie":
		return "pie"
	case "donut", "doughnut":
		return "donut"
	case "combo", "mixed", "mixed_bar_line", "bar_line_combo":
		return "combo"
	default:
		return ""
	}
}

func officeNormalizeChartSeriesType(chartType, seriesType string, index int) string {
	seriesType = strings.ToLower(strings.TrimSpace(seriesType))
	switch chartType {
	case "combo":
		switch seriesType {
		case "", "bar", "column", "col":
			if index == 0 || seriesType != "" {
				return "bar"
			}
			return "line"
		case "line", "trend":
			return "line"
		default:
			return ""
		}
	case "line":
		return "line"
	case "pie", "donut":
		return chartType
	case "stacked_bar", "percent_stacked_bar":
		return "bar"
	case "stacked_column", "percent_stacked_column", "bar":
		return "bar"
	default:
		if seriesType == "line" {
			return "line"
		}
		return chartType
	}
}

func officeNormalizeChartSeriesAxis(chartType, seriesType, axis string) string {
	if chartType != "combo" {
		return "primary"
	}
	switch strings.ToLower(strings.TrimSpace(axis)) {
	case "secondary", "secondary_y", "secondary-axis", "secondary_axis", "right", "rhs", "y2", "2":
		return "secondary"
	case "", "primary", "primary_y", "primary-axis", "primary_axis", "left", "lhs", "y1", "1":
		return "primary"
	default:
		if seriesType == "line" && strings.Contains(strings.ToLower(strings.TrimSpace(axis)), "secondary") {
			return "secondary"
		}
		return "primary"
	}
}

func officeNormalizeChartSeriesColor(color string) string {
	color = strings.TrimSpace(color)
	color = strings.TrimPrefix(color, "#")
	color = strings.TrimPrefix(strings.TrimPrefix(color, "0x"), "0X")
	if len(color) == 3 {
		color = strings.Repeat(string(color[0]), 2) + strings.Repeat(string(color[1]), 2) + strings.Repeat(string(color[2]), 2)
	}
	if len(color) != 6 {
		return ""
	}
	color = strings.ToUpper(color)
	for _, ch := range color {
		if (ch < '0' || ch > '9') && (ch < 'A' || ch > 'F') {
			return ""
		}
	}
	return color
}

func officeNormalizeChartPointColors(raw interface{}) []string {
	var items []interface{}
	switch typed := raw.(type) {
	case nil:
		return nil
	case []interface{}:
		items = typed
	case []string:
		items = make([]interface{}, 0, len(typed))
		for _, value := range typed {
			items = append(items, value)
		}
	default:
		return nil
	}
	colors := make([]string, 0, len(items))
	lastNonEmpty := -1
	for idx, item := range items {
		color := officeNormalizeChartSeriesColor(anyToStringForLLM(item))
		colors = append(colors, color)
		if color != "" {
			lastNonEmpty = idx
		}
	}
	if lastNonEmpty < 0 {
		return nil
	}
	return append([]string(nil), colors[:lastNonEmpty+1]...)
}

func officeNormalizeChartPointExplosions(raw interface{}) []int {
	var items []interface{}
	switch typed := raw.(type) {
	case nil:
		return nil
	case []interface{}:
		items = typed
	case []int:
		items = make([]interface{}, 0, len(typed))
		for _, value := range typed {
			items = append(items, value)
		}
	default:
		return nil
	}
	explosions := make([]int, 0, len(items))
	lastNonZero := -1
	for idx, item := range items {
		value, ok := officeChartNumber(item)
		if !ok || value <= 0 {
			explosions = append(explosions, 0)
			continue
		}
		if value > 400 {
			value = 400
		}
		explosion := int(math.Round(value))
		explosions = append(explosions, explosion)
		if explosion > 0 {
			lastNonZero = idx
		}
	}
	if lastNonZero < 0 {
		return nil
	}
	return append([]int(nil), explosions[:lastNonZero+1]...)
}

func officeNormalizeChartPointLabelVisibility(raw interface{}) []string {
	var items []interface{}
	switch typed := raw.(type) {
	case nil:
		return nil
	case []interface{}:
		items = typed
	case []string:
		items = make([]interface{}, 0, len(typed))
		for _, value := range typed {
			items = append(items, value)
		}
	case []bool:
		items = make([]interface{}, 0, len(typed))
		for _, value := range typed {
			items = append(items, value)
		}
	default:
		return nil
	}
	visibility := make([]string, 0, len(items))
	lastNonEmpty := -1
	for idx, item := range items {
		value := officeNormalizeChartLabels(item)
		visibility = append(visibility, value)
		if value != "" {
			lastNonEmpty = idx
		}
	}
	if lastNonEmpty < 0 {
		return nil
	}
	return append([]string(nil), visibility[:lastNonEmpty+1]...)
}

func officeNormalizeChartPointLabelPositions(raw interface{}) []string {
	var items []interface{}
	switch typed := raw.(type) {
	case nil:
		return nil
	case []interface{}:
		items = typed
	case []string:
		items = make([]interface{}, 0, len(typed))
		for _, value := range typed {
			items = append(items, value)
		}
	default:
		return nil
	}
	positions := make([]string, 0, len(items))
	lastNonEmpty := -1
	for idx, item := range items {
		value := officeNormalizeChartLabelPosition(anyToStringForLLM(item))
		positions = append(positions, value)
		if value != "" {
			lastNonEmpty = idx
		}
	}
	if lastNonEmpty < 0 {
		return nil
	}
	return append([]string(nil), positions[:lastNonEmpty+1]...)
}

func officeNormalizeChartPointLabelFormats(raw interface{}) []string {
	var items []interface{}
	switch typed := raw.(type) {
	case nil:
		return nil
	case []interface{}:
		items = typed
	case []string:
		items = make([]interface{}, 0, len(typed))
		for _, value := range typed {
			items = append(items, value)
		}
	default:
		return nil
	}
	formats := make([]string, 0, len(items))
	lastNonEmpty := -1
	for idx, item := range items {
		value := officeNormalizeChartLabelFormat(anyToStringForLLM(item))
		formats = append(formats, value)
		if value != "" {
			lastNonEmpty = idx
		}
	}
	if lastNonEmpty < 0 {
		return nil
	}
	return append([]string(nil), formats[:lastNonEmpty+1]...)
}

func officeNormalizeChartPointLabelSeparators(raw interface{}) []string {
	var items []interface{}
	switch typed := raw.(type) {
	case nil:
		return nil
	case []interface{}:
		items = typed
	case []string:
		items = make([]interface{}, 0, len(typed))
		for _, value := range typed {
			items = append(items, value)
		}
	default:
		return nil
	}
	separators := make([]string, 0, len(items))
	lastNonEmpty := -1
	for idx, item := range items {
		value := officeNormalizeChartLabelSeparator(anyToStringForLLM(item))
		separators = append(separators, value)
		if value != "" {
			lastNonEmpty = idx
		}
	}
	if lastNonEmpty < 0 {
		return nil
	}
	return append([]string(nil), separators[:lastNonEmpty+1]...)
}

func officeNormalizeChartSeriesLineWidth(raw interface{}) float64 {
	width, ok := officeChartNumber(raw)
	if !ok || width <= 0 {
		return 0
	}
	if width < 0.25 {
		return 0.25
	}
	if width > 12 {
		return 12
	}
	return width
}

func officeNormalizeChartSeriesDash(dash string) string {
	switch strings.ToLower(strings.TrimSpace(dash)) {
	case "":
		return ""
	case "solid":
		return "solid"
	case "dash", "dashed":
		return "dash"
	case "dot", "dotted":
		return "sysDot"
	case "dash_dot", "dash-dot", "dashdot":
		return "dashDot"
	default:
		return ""
	}
}

func officeNormalizeChartSeriesMarker(marker string) string {
	switch strings.ToLower(strings.TrimSpace(marker)) {
	case "", "auto", "default":
		return ""
	case "circle":
		return "circle"
	case "diamond":
		return "diamond"
	case "square":
		return "square"
	case "triangle":
		return "triangle"
	case "x":
		return "x"
	case "star":
		return "star"
	case "dash":
		return "dash"
	case "dot":
		return "dot"
	case "none", "off":
		return "none"
	default:
		return ""
	}
}

func officeNormalizeChartLabels(raw interface{}) string {
	switch value := raw.(type) {
	case bool:
		if value {
			return "show"
		}
		return "hide"
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "":
			return ""
		case "show", "shown", "on", "true", "yes", "value", "values", "label", "labels":
			return "show"
		case "hide", "hidden", "off", "false", "no", "none":
			return "hide"
		default:
			return ""
		}
	default:
		return ""
	}
}

func officeNormalizeChartLabelPosition(position string) string {
	switch strings.ToLower(strings.TrimSpace(position)) {
	case "":
		return ""
	case "center", "centre", "middle", "ctr":
		return "ctr"
	case "inside_end", "inside-end", "insideend", "in_end", "in-end", "inend":
		return "inEnd"
	case "outside_end", "outside-end", "outsideend", "out_end", "out-end", "outend":
		return "outEnd"
	case "best_fit", "best-fit", "bestfit":
		return "bestFit"
	case "above", "top", "t":
		return "t"
	case "below", "bottom", "b":
		return "b"
	case "left", "l":
		return "l"
	case "right", "r":
		return "r"
	default:
		return ""
	}
}

func officeNormalizeChartLegendPosition(position string) string {
	switch strings.ToLower(strings.TrimSpace(position)) {
	case "":
		return ""
	case "left", "l":
		return "l"
	case "right", "r":
		return "r"
	case "top", "t":
		return "t"
	case "bottom", "b":
		return "b"
	case "top_right", "top-right", "topright", "tr":
		return "tr"
	default:
		return ""
	}
}

func officeNormalizeChartStartAngle(raw interface{}) int {
	value, ok := officeChartNumber(raw)
	if !ok {
		return 0
	}
	if value < 0 {
		value = 0
	}
	if value > 360 {
		value = 360
	}
	return int(math.Round(value))
}

func officeNormalizeChartHoleSize(raw interface{}) int {
	value, ok := officeChartNumber(raw)
	if !ok || value <= 0 {
		return 0
	}
	if value < 10 {
		value = 10
	}
	if value > 90 {
		value = 90
	}
	return int(math.Round(value))
}

func officeNormalizeChartGapWidth(raw interface{}) *int {
	value, ok := officeChartNumber(raw)
	if !ok {
		return nil
	}
	if value < 0 {
		value = 0
	}
	if value > 500 {
		value = 500
	}
	width := int(math.Round(value))
	return &width
}

func officeNormalizeChartOverlap(raw interface{}) *int {
	value, ok := officeChartNumber(raw)
	if !ok {
		return nil
	}
	if value < -100 {
		value = -100
	}
	if value > 100 {
		value = 100
	}
	overlap := int(math.Round(value))
	return &overlap
}

func officeNormalizeChartLabelFormat(format string) string {
	format = strings.TrimSpace(format)
	if format == "" {
		return ""
	}
	return format
}

func officeNormalizeChartLabelSeparator(separator string) string {
	separator = strings.TrimSpace(separator)
	if separator == "" {
		return ""
	}
	return separator
}

func officeNormalizeChartAxisFormat(format string) string {
	format = strings.TrimSpace(format)
	if format == "" {
		return ""
	}
	return format
}

func officeNormalizeChartCategoryAxisType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "category", "cat", "text", "string", "discrete":
		return "category"
	case "date", "calendar":
		return "date"
	case "time", "datetime", "date-time", "date_time", "timestamp":
		return "datetime"
	default:
		return ""
	}
}

func officeNormalizeChartDateAxisTimeUnit(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "day", "days", "daily":
		return "days"
	case "month", "months", "monthly":
		return "months"
	case "year", "years", "yearly", "annual", "annually":
		return "years"
	default:
		return ""
	}
}

func officeNormalizeChartDateAxisBound(raw interface{}, withTime bool) *float64 {
	if value, ok := officeChartNumber(raw); ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
		bound := value
		return &bound
	}
	if serial, ok := officeExcelDateSerial(raw, withTime); ok {
		if value, ok := officeChartNumber(serial); ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
			bound := value
			return &bound
		}
	}
	return nil
}

func officeParseChartDateAxisBound(raw interface{}, fieldName string, withTime bool) (*float64, error) {
	switch typed := raw.(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
	}
	bound := officeNormalizeChartDateAxisBound(raw, withTime)
	if bound != nil {
		return bound, nil
	}
	return nil, fmt.Errorf("%s must be an ISO-like %s or Excel serial when x_axis_type is %s", fieldName, map[bool]string{true: "date-time", false: "date"}[withTime], map[bool]string{true: "datetime", false: "date"}[withTime])
}

func officeNormalizeChartAxisBound(raw interface{}) *float64 {
	value, ok := officeChartNumber(raw)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	bound := value
	return &bound
}

func officeNormalizeChartAxisUnit(raw interface{}) *float64 {
	value, ok := officeChartNumber(raw)
	if !ok || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	unit := value
	return &unit
}

func officeNormalizeChartAxisLabelPosition(position string) string {
	switch strings.ToLower(strings.TrimSpace(position)) {
	case "":
		return ""
	case "nextto", "next_to", "next-to", "next", "adjacent":
		return "nextTo"
	case "high":
		return "high"
	case "low":
		return "low"
	case "none", "off", "hide", "hidden":
		return "none"
	default:
		return ""
	}
}

func officeNormalizeChartAxisLabelAlignment(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "ctr", "center", "centre", "middle":
		return "ctr"
	case "l", "left":
		return "l"
	case "r", "right":
		return "r"
	default:
		return ""
	}
}

func officeNormalizeChartAxisLabelOffset(raw interface{}) *int {
	value, ok := officeChartNumber(raw)
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	offset := int(math.Round(value))
	if offset < 0 || offset > 1000 {
		return nil
	}
	normalized := offset
	return &normalized
}

func officeNormalizeChartAxisCrosses(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "auto", "auto_zero", "auto-zero", "autozero", "zero":
		return "autoZero"
	case "max", "maximum", "high":
		return "max"
	case "min", "minimum", "low":
		return "min"
	default:
		return ""
	}
}

func officeNormalizeChartAxisCrossBetween(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "between":
		return "between"
	case "midcat", "mid_cat", "mid-cat", "midcategory", "mid_category", "mid-category":
		return "midCat"
	default:
		return ""
	}
}

func officeNormalizeChartAxisReverseOrder(raw interface{}) string {
	switch value := raw.(type) {
	case bool:
		if value {
			return "maxMin"
		}
		return ""
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "":
			return ""
		case "reverse", "reversed", "reverse_order", "reverse-order", "descending", "desc", "maxmin", "max_min", "max-min":
			return "maxMin"
		case "normal", "default", "ascending", "asc", "minmax", "min_max", "min-max":
			return "minMax"
		default:
			return ""
		}
	default:
		return ""
	}
}

func officeNormalizeChartTickMark(mark string) string {
	switch strings.ToLower(strings.TrimSpace(mark)) {
	case "":
		return ""
	case "cross":
		return "cross"
	case "in", "inside":
		return "in"
	case "out", "outside":
		return "out"
	case "none", "off":
		return "none"
	default:
		return ""
	}
}

func officeChartNumber(raw interface{}) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case *float64:
		if value == nil {
			return 0, false
		}
		return *value, true
	case *float32:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case int32:
		return float64(value), true
	case int16:
		return float64(value), true
	case int8:
		return float64(value), true
	case *int:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case *int64:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case *int32:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case *int16:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case *int8:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case uint:
		return float64(value), true
	case uint64:
		return float64(value), true
	case uint32:
		return float64(value), true
	case uint16:
		return float64(value), true
	case uint8:
		return float64(value), true
	case *uint:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case *uint64:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case *uint32:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case *uint16:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case *uint8:
		if value == nil {
			return 0, false
		}
		return float64(*value), true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	case string:
		return officeParseNumericString(value)
	default:
		return 0, false
	}
}

func officeStringSliceArg(args map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		if raw, ok := compatArgValue(args, key); ok {
			return officeStringSliceAny(raw)
		}
	}
	return nil
}

func officeStringSliceAny(raw interface{}) []string {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(anyToStringForLLM(item))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func officeNumberValue(raw interface{}) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func compatBoolArgDefault(args map[string]interface{}, key string, fallback bool) bool {
	raw, ok := compatArgValue(args, key)
	if !ok {
		return fallback
	}
	return compatBoolValueDefault(raw, fallback)
}

func compatBoolValueDefault(raw interface{}, fallback bool) bool {
	value, ok := raw.(bool)
	if !ok {
		return fallback
	}
	return value
}

func firstMapValue(m map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value
		}
	}
	return nil
}
