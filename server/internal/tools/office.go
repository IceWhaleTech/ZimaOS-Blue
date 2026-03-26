package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
}

type officeTableSpec struct {
	Headers []string
	Rows    [][]string
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
					"description": "For docx output, optional structured sections. Each section can include heading, paragraphs/body/text, bullets/items, and table.",
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
		sections = append(sections, section)
	}
	return sections, nil
}

func parseOfficeTable(raw interface{}) (*officeTableSpec, error) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil, errors.New("table must be an object")
	}
	headers := officeStringSliceAny(firstMapValue(m, "headers", "columns"))
	rowsRaw, ok := m["rows"].([]interface{})
	if !ok {
		return nil, errors.New("table rows must be an array")
	}
	rows := make([][]string, 0, len(rowsRaw))
	for _, item := range rowsRaw {
		rowValues, ok := item.([]interface{})
		if !ok {
			return nil, errors.New("table rows must contain arrays")
		}
		row := make([]string, len(rowValues))
		for idx, value := range rowValues {
			row[idx] = strings.TrimSpace(anyToStringForLLM(value))
		}
		rows = append(rows, row)
	}
	if len(headers) == 0 && len(rows) > 0 {
		headers = make([]string, len(rows[0]))
		for i := range headers {
			headers[i] = fmt.Sprintf("Column %d", i+1)
		}
	}
	return &officeTableSpec{Headers: headers, Rows: rows}, nil
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
