package convert

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const maxTabularGroupValues = 12

type TabularSummary struct {
	Source        string                                   `json:"source,omitempty"`
	RowCount      int                                      `json:"row_count"`
	FormulaCount  int                                      `json:"formula_count,omitempty"`
	Headers       []string                                 `json:"headers,omitempty"`
	DateColumns   []string                                 `json:"date_columns,omitempty"`
	ColumnKinds   map[string]string                        `json:"column_kinds,omitempty"`
	NumericTotals map[string]float64                       `json:"numeric_totals,omitempty"`
	GroupedTotals map[string]map[string]map[string]float64 `json:"grouped_totals,omitempty"`
	TopByMetric   map[string]map[string]TabularTopValue    `json:"top_by_metric,omitempty"`
	ChangedCells  int                                      `json:"changed_cells,omitempty"`
	Highlights    []string                                 `json:"highlights,omitempty"`
}

type TabularTopValue struct {
	Value string  `json:"value"`
	Total float64 `json:"total"`
}

type TabularWorkbookSummary struct {
	SheetSummaries []TabularSummary `json:"sheet_summaries,omitempty"`
	Highlights     []string         `json:"highlights,omitempty"`
}

func SummarizeDelimitedFile(path string) (*TabularSummary, error) {
	workbook, err := loadDelimitedWorkbook(path)
	if err != nil {
		return nil, err
	}
	if workbook == nil || len(workbook.Sheets) == 0 {
		return nil, nil
	}
	return workbook.Sheets[0].Summary, nil
}

func summarizeWorkbookSheets(sheets []spreadsheetSheet) *TabularWorkbookSummary {
	if len(sheets) == 0 {
		return nil
	}
	summary := &TabularWorkbookSummary{
		SheetSummaries: make([]TabularSummary, 0, len(sheets)),
	}
	for _, sheet := range sheets {
		if sheet.Summary == nil {
			continue
		}
		summary.SheetSummaries = append(summary.SheetSummaries, *sheet.Summary)
		prefix := strings.TrimSpace(sheet.Name)
		highlights := sheet.Summary.Highlights
		if len(highlights) > 3 {
			highlights = highlights[:3]
		}
		for _, highlight := range highlights {
			if prefix == "" {
				summary.Highlights = append(summary.Highlights, highlight)
				continue
			}
			summary.Highlights = append(summary.Highlights, fmt.Sprintf("%s: %s", prefix, highlight))
		}
	}
	if len(summary.SheetSummaries) == 0 && len(summary.Highlights) == 0 {
		return nil
	}
	return summary
}

func summarizeTabularRecords(source string, headers []string, records []map[string]interface{}) *TabularSummary {
	summary := &TabularSummary{
		Source:   strings.TrimSpace(source),
		RowCount: len(records),
	}

	headers = compactTabularHeaders(headers, records)
	if len(headers) > 0 {
		summary.Headers = append([]string(nil), headers...)
	}
	if len(records) == 0 {
		return summary
	}

	numericHeaders := detectNumericHeaders(headers, records)
	if len(numericHeaders) > 0 {
		summary.NumericTotals = make(map[string]float64, len(numericHeaders))
		for _, metric := range numericHeaders {
			total := 0.0
			for _, record := range records {
				value, ok := tabularNumericValue(record[metric])
				if !ok {
					continue
				}
				total += value
			}
			summary.NumericTotals[metric] = roundTabularNumber(total)
		}
	}

	groupHeaders := detectGroupHeaders(headers, numericHeaders, records)
	if len(groupHeaders) > 0 && len(numericHeaders) > 0 {
		grouped := make(map[string]map[string]map[string]float64, len(groupHeaders))
		for _, groupHeader := range groupHeaders {
			metrics := make(map[string]map[string]float64, len(numericHeaders))
			for _, metric := range numericHeaders {
				values := make(map[string]float64)
				for _, record := range records {
					groupValue := strings.TrimSpace(tabularStringValue(record[groupHeader]))
					if groupValue == "" {
						continue
					}
					metricValue, ok := tabularNumericValue(record[metric])
					if !ok {
						continue
					}
					values[groupValue] += metricValue
				}
				if len(values) > 0 {
					for key, value := range values {
						values[key] = roundTabularNumber(value)
					}
					metrics[metric] = values
				}
			}
			if len(metrics) > 0 {
				grouped[groupHeader] = metrics
			}
		}
		if len(grouped) > 0 {
			summary.GroupedTotals = grouped
		}
	}

	addDerivedProfitMetric(summary)
	summary.TopByMetric = buildTopByMetric(summary.GroupedTotals)
	summary.Highlights = buildTabularHighlights(summary)
	return summary
}

func compactTabularHeaders(headers []string, records []map[string]interface{}) []string {
	seen := make(map[string]struct{}, len(headers))
	out := make([]string, 0, len(headers))
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		if _, ok := seen[header]; ok {
			continue
		}
		seen[header] = struct{}{}
		out = append(out, header)
	}
	if len(out) > 0 {
		return out
	}
	keySet := make(map[string]struct{})
	for _, record := range records {
		for key := range record {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			keySet[key] = struct{}{}
		}
	}
	if len(keySet) == 0 {
		return nil
	}
	out = make([]string, 0, len(keySet))
	for key := range keySet {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func detectNumericHeaders(headers []string, records []map[string]interface{}) []string {
	out := make([]string, 0, len(headers))
	for _, header := range headers {
		numericSeen := 0
		nonNumericSeen := 0
		for _, record := range records {
			value := record[header]
			if tabularValueEmpty(value) {
				continue
			}
			if _, ok := tabularNumericValue(value); ok {
				numericSeen++
				continue
			}
			nonNumericSeen++
		}
		if numericSeen > 0 && nonNumericSeen == 0 {
			out = append(out, header)
		}
	}
	return out
}

func detectGroupHeaders(headers, numericHeaders []string, records []map[string]interface{}) []string {
	numericSet := make(map[string]struct{}, len(numericHeaders))
	for _, header := range numericHeaders {
		numericSet[header] = struct{}{}
	}
	out := make([]string, 0, len(headers))
	for _, header := range headers {
		if _, ok := numericSet[header]; ok {
			continue
		}
		values := make(map[string]struct{})
		for _, record := range records {
			value := strings.TrimSpace(tabularStringValue(record[header]))
			if value == "" {
				continue
			}
			values[value] = struct{}{}
			if len(values) > maxTabularGroupValues {
				break
			}
		}
		if len(values) >= 2 && len(values) <= maxTabularGroupValues {
			out = append(out, header)
		}
	}
	return out
}

func addDerivedProfitMetric(summary *TabularSummary) {
	if summary == nil || len(summary.NumericTotals) == 0 {
		return
	}
	revenue, okRevenue := summary.NumericTotals["Revenue"]
	cost, okCost := summary.NumericTotals["Cost"]
	if !okRevenue || !okCost {
		return
	}
	summary.NumericTotals["Profit"] = roundTabularNumber(revenue - cost)
	if len(summary.GroupedTotals) == 0 {
		return
	}
	for _, metrics := range summary.GroupedTotals {
		revenueByGroup, hasRevenue := metrics["Revenue"]
		costByGroup, hasCost := metrics["Cost"]
		if !hasRevenue || !hasCost {
			continue
		}
		profitByGroup := make(map[string]float64)
		groupNames := make(map[string]struct{}, len(revenueByGroup)+len(costByGroup))
		for name := range revenueByGroup {
			groupNames[name] = struct{}{}
		}
		for name := range costByGroup {
			groupNames[name] = struct{}{}
		}
		for name := range groupNames {
			profitByGroup[name] = roundTabularNumber(revenueByGroup[name] - costByGroup[name])
		}
		metrics["Profit"] = profitByGroup
	}
}

func buildTopByMetric(groupedTotals map[string]map[string]map[string]float64) map[string]map[string]TabularTopValue {
	if len(groupedTotals) == 0 {
		return nil
	}
	out := make(map[string]map[string]TabularTopValue)
	for dimension, metrics := range groupedTotals {
		for metric, groups := range metrics {
			name, total, ok := topTabularGroup(groups)
			if !ok {
				continue
			}
			if _, exists := out[metric]; !exists {
				out[metric] = make(map[string]TabularTopValue)
			}
			out[metric][dimension] = TabularTopValue{
				Value: name,
				Total: roundTabularNumber(total),
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func topTabularGroup(groups map[string]float64) (string, float64, bool) {
	bestName := ""
	bestTotal := 0.0
	ok := false
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		total := roundTabularNumber(groups[name])
		if !ok || total > bestTotal {
			bestName = name
			bestTotal = total
			ok = true
		}
	}
	return bestName, bestTotal, ok
}

func buildTabularHighlights(summary *TabularSummary) []string {
	if summary == nil {
		return nil
	}
	highlights := make([]string, 0, 8)
	for _, metric := range sortMetricsByPriority(summary.NumericTotals) {
		highlights = append(highlights, fmt.Sprintf("Total %s: %s", humanizeTabularLabel(metric), formatTabularNumber(summary.NumericTotals[metric])))
		if len(highlights) >= 4 {
			break
		}
	}

	topCount := 0
	for _, metric := range sortMetricsByPriorityFromTop(summary.TopByMetric) {
		byDimension := summary.TopByMetric[metric]
		for _, dimension := range sortDimensionsByPriority(byDimension) {
			top := byDimension[dimension]
			highlights = append(highlights, fmt.Sprintf(
				"Top %s by %s: %s (%s)",
				humanizeTabularLabel(metric),
				humanizeTabularLabel(dimension),
				top.Value,
				formatTabularNumber(top.Total),
			))
			topCount++
			if topCount >= 4 {
				return highlights
			}
		}
	}
	return highlights
}

func sortMetricsByPriority(values map[string]float64) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi := tabularMetricPriority(keys[i])
		pj := tabularMetricPriority(keys[j])
		if pi != pj {
			return pi < pj
		}
		return keys[i] < keys[j]
	})
	return keys
}

func sortMetricsByPriorityFromTop(values map[string]map[string]TabularTopValue) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi := tabularMetricPriority(keys[i])
		pj := tabularMetricPriority(keys[j])
		if pi != pj {
			return pi < pj
		}
		return keys[i] < keys[j]
	})
	return keys
}

func sortDimensionsByPriority(values map[string]TabularTopValue) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi := tabularDimensionPriority(keys[i])
		pj := tabularDimensionPriority(keys[j])
		if pi != pj {
			return pi < pj
		}
		return keys[i] < keys[j]
	})
	return keys
}

func tabularMetricPriority(metric string) int {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "revenue":
		return 0
	case "profit":
		return 1
	case "units_sold":
		return 2
	case "amount":
		return 3
	case "q1_budget":
		return 4
	case "q2_budget":
		return 5
	case "q3_budget":
		return 6
	case "q4_budget":
		return 7
	case "cost":
		return 8
	default:
		return 100
	}
}

func tabularDimensionPriority(metric string) int {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "region":
		return 0
	case "product":
		return 1
	case "department":
		return 2
	case "employee":
		return 3
	case "owner":
		return 4
	default:
		return 100
	}
}

func tabularNumericValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case jsonNumber:
		f, err := strconv.ParseFloat(string(v), 64)
		return f, err == nil
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(text, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

type jsonNumber string

func tabularStringValue(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

func tabularValueEmpty(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	default:
		return false
	}
}

func roundTabularNumber(value float64) float64 {
	return math.Round(value*100) / 100
}

func humanizeTabularLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return label
	}
	parts := strings.FieldsFunc(label, func(r rune) bool {
		return r == '_' || r == '-'
	})
	if len(parts) == 0 {
		return label
	}
	for idx, part := range parts {
		switch strings.ToUpper(part) {
		case "Q1", "Q2", "Q3", "Q4", "APM", "CSV", "XLSX":
			parts[idx] = strings.ToUpper(part)
		default:
			if part == "" {
				continue
			}
			parts[idx] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}

func formatTabularNumber(value float64) string {
	value = roundTabularNumber(value)
	if math.Abs(value-math.Round(value)) < 0.000001 {
		return formatTabularInteger(int64(math.Round(value)))
	}
	text := strconv.FormatFloat(value, 'f', 2, 64)
	text = strings.TrimSuffix(text, "00")
	text = strings.TrimSuffix(text, "0")
	text = strings.TrimSuffix(text, ".")
	parts := strings.SplitN(text, ".", 2)
	intPart, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return text
	}
	formatted := formatTabularInteger(intPart)
	if len(parts) == 2 && parts[1] != "" {
		return formatted + "." + parts[1]
	}
	return formatted
}

func formatTabularInteger(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}
	text := strconv.FormatInt(value, 10)
	if len(text) <= 3 {
		if negative {
			return "-" + text
		}
		return text
	}
	parts := make([]string, 0, (len(text)+2)/3)
	for len(text) > 3 {
		parts = append(parts, text[len(text)-3:])
		text = text[:len(text)-3]
	}
	if text != "" {
		parts = append(parts, text)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	out := strings.Join(parts, ",")
	if negative {
		return "-" + out
	}
	return out
}
