package tools

import (
	"strconv"
	"strings"
	"unicode"
)

const officeZeroWidthChars = "\u200b\u200c\u200d\ufeff"

func cleanupOfficeDocSpec(spec officeDocSpec) officeDocSpec {
	spec.Title = officeCleanDocText(spec.Title)
	spec.Subtitle = officeCleanDocText(spec.Subtitle)
	spec.Summary = officeJoinDocParagraphs(officeCleanupDocStringList(splitOfficeParagraphs(spec.Summary)))
	spec.ParagraphBlocks = officeCleanupDocBlocks(spec.ParagraphBlocks)
	spec.Paragraphs = officeCleanupDocStringList(spec.Paragraphs)
	spec.Notes = officeCleanupDocStringList(spec.Notes)

	sections := make([]officeDocSection, 0, len(spec.Sections))
	for _, section := range spec.Sections {
		section.Heading = officeCleanDocText(section.Heading)
		section.ParagraphBlocks = officeCleanupDocBlocks(section.ParagraphBlocks)
		section.Paragraphs = officeCleanupDocStringList(section.Paragraphs)
		section.Bullets = officeCleanupDocStringList(section.Bullets)
		section.Table = officeCleanupDocTable(section.Table)
		section.Chart = officeCleanupDocChart(section.Chart)
		if section.Heading == "" && len(section.ParagraphBlocks) == 0 && len(section.Paragraphs) == 0 && len(section.Bullets) == 0 && section.Table == nil && section.Chart == nil {
			continue
		}
		sections = append(sections, section)
	}
	spec.Sections = sections
	return spec
}

func officeCleanupDocTable(table *officeTableSpec) *officeTableSpec {
	if table == nil {
		return nil
	}
	cleaned := &officeTableSpec{
		Headers:          make([]string, 0, len(table.Headers)),
		Rows:             make([][]string, 0, len(table.Rows)),
		ColumnWidths:     append([]float64(nil), table.ColumnWidths...),
		ColumnAlignments: append([]string(nil), table.ColumnAlignments...),
	}
	for _, header := range table.Headers {
		cleaned.Headers = append(cleaned.Headers, officeCleanDocText(header))
	}
	for _, row := range table.Rows {
		cleanedRow := make([]string, 0, len(row))
		nonEmpty := false
		for _, cell := range row {
			value := officeCleanDocText(cell)
			if value != "" {
				nonEmpty = true
			}
			cleanedRow = append(cleanedRow, value)
		}
		if nonEmpty {
			cleaned.Rows = append(cleaned.Rows, cleanedRow)
		}
	}
	if len(cleaned.Headers) == 0 && len(cleaned.Rows) == 0 {
		return nil
	}
	return cleaned
}

func officeCleanupDocChart(chart *officeChartSpec) *officeChartSpec {
	if chart == nil {
		return nil
	}
	cleaned := &officeChartSpec{
		Type:                                  officeNormalizeChartType(chart.Type),
		Title:                                 officeCleanDocText(chart.Title),
		CategoryAxisTitle:                     officeCleanDocText(chart.CategoryAxisTitle),
		SecondaryCategoryAxisTitle:            officeCleanDocText(chart.SecondaryCategoryAxisTitle),
		CategoryAxisType:                      officeNormalizeChartCategoryAxisType(chart.CategoryAxisType),
		CategoryAxisLabelPosition:             officeNormalizeChartAxisLabelPosition(chart.CategoryAxisLabelPosition),
		SecondaryCategoryAxisLabelPosition:    officeNormalizeChartAxisLabelPosition(chart.SecondaryCategoryAxisLabelPosition),
		CategoryAxisReverseOrder:              officeNormalizeChartAxisReverseOrder(chart.CategoryAxisReverseOrder),
		SecondaryCategoryAxisReverseOrder:     officeNormalizeChartAxisReverseOrder(chart.SecondaryCategoryAxisReverseOrder),
		CategoryAxisCrosses:                   officeNormalizeChartAxisCrosses(chart.CategoryAxisCrosses),
		SecondaryCategoryAxisCrosses:          officeNormalizeChartAxisCrosses(chart.SecondaryCategoryAxisCrosses),
		CategoryAxisMajorTickMark:             officeNormalizeChartTickMark(chart.CategoryAxisMajorTickMark),
		CategoryAxisMinorTickMark:             officeNormalizeChartTickMark(chart.CategoryAxisMinorTickMark),
		SecondaryCategoryAxisMajorTickMark:    officeNormalizeChartTickMark(chart.SecondaryCategoryAxisMajorTickMark),
		SecondaryCategoryAxisMinorTickMark:    officeNormalizeChartTickMark(chart.SecondaryCategoryAxisMinorTickMark),
		CategoryAxisLabelAlignment:            officeNormalizeChartAxisLabelAlignment(chart.CategoryAxisLabelAlignment),
		SecondaryCategoryAxisLabelAlignment:   officeNormalizeChartAxisLabelAlignment(chart.SecondaryCategoryAxisLabelAlignment),
		CategoryAxisLabelOffset:               officeNormalizeChartAxisLabelOffset(chart.CategoryAxisLabelOffset),
		SecondaryCategoryAxisLabelOffset:      officeNormalizeChartAxisLabelOffset(chart.SecondaryCategoryAxisLabelOffset),
		CategoryAxisMultiLevelLabels:          officeNormalizeChartLabels(chart.CategoryAxisMultiLevelLabels),
		SecondaryCategoryAxisMultiLevelLabels: officeNormalizeChartLabels(chart.SecondaryCategoryAxisMultiLevelLabels),
		CategoryAxisVisible:                   officeNormalizeChartLabels(chart.CategoryAxisVisible),
		SecondaryCategoryAxisVisible:          officeNormalizeChartLabels(chart.SecondaryCategoryAxisVisible),
		CategoryAxisAuto:                      officeNormalizeChartLabels(chart.CategoryAxisAuto),
		SecondaryCategoryAxisAuto:             officeNormalizeChartLabels(chart.SecondaryCategoryAxisAuto),
		CategoryAxisFormat:                    officeNormalizeChartAxisFormat(chart.CategoryAxisFormat),
		SecondaryCategoryAxisFormat:           officeNormalizeChartAxisFormat(chart.SecondaryCategoryAxisFormat),
		CategoryAxisMin:                       officeNormalizeChartAxisBound(chart.CategoryAxisMin),
		CategoryAxisMax:                       officeNormalizeChartAxisBound(chart.CategoryAxisMax),
		CategoryAxisBaseTimeUnit:              officeNormalizeChartDateAxisTimeUnit(chart.CategoryAxisBaseTimeUnit),
		CategoryAxisMajorUnit:                 officeNormalizeChartAxisUnit(chart.CategoryAxisMajorUnit),
		CategoryAxisMinorUnit:                 officeNormalizeChartAxisUnit(chart.CategoryAxisMinorUnit),
		CategoryAxisMajorTimeUnit:             officeNormalizeChartDateAxisTimeUnit(chart.CategoryAxisMajorTimeUnit),
		CategoryAxisMinorTimeUnit:             officeNormalizeChartDateAxisTimeUnit(chart.CategoryAxisMinorTimeUnit),
		ValueAxisTitle:                        officeCleanDocText(chart.ValueAxisTitle),
		SecondaryValueAxisTitle:               officeCleanDocText(chart.SecondaryValueAxisTitle),
		ValueAxisFormat:                       officeNormalizeChartAxisFormat(chart.ValueAxisFormat),
		SecondaryValueAxisFormat:              officeNormalizeChartAxisFormat(chart.SecondaryValueAxisFormat),
		ValueAxisMin:                          officeNormalizeChartAxisBound(chart.ValueAxisMin),
		ValueAxisMax:                          officeNormalizeChartAxisBound(chart.ValueAxisMax),
		SecondaryValueAxisMin:                 officeNormalizeChartAxisBound(chart.SecondaryValueAxisMin),
		SecondaryValueAxisMax:                 officeNormalizeChartAxisBound(chart.SecondaryValueAxisMax),
		ValueAxisMajorUnit:                    officeNormalizeChartAxisUnit(chart.ValueAxisMajorUnit),
		ValueAxisMinorUnit:                    officeNormalizeChartAxisUnit(chart.ValueAxisMinorUnit),
		SecondaryValueAxisMajorUnit:           officeNormalizeChartAxisUnit(chart.SecondaryValueAxisMajorUnit),
		SecondaryValueAxisMinorUnit:           officeNormalizeChartAxisUnit(chart.SecondaryValueAxisMinorUnit),
		ValueAxisMajorGridlines:               officeNormalizeChartLabels(chart.ValueAxisMajorGridlines),
		ValueAxisMinorGridlines:               officeNormalizeChartLabels(chart.ValueAxisMinorGridlines),
		SecondaryValueAxisMajorGridlines:      officeNormalizeChartLabels(chart.SecondaryValueAxisMajorGridlines),
		SecondaryValueAxisMinorGridlines:      officeNormalizeChartLabels(chart.SecondaryValueAxisMinorGridlines),
		ValueAxisCrosses:                      officeNormalizeChartAxisCrosses(chart.ValueAxisCrosses),
		SecondaryValueAxisCrosses:             officeNormalizeChartAxisCrosses(chart.SecondaryValueAxisCrosses),
		ValueAxisCrossBetween:                 officeNormalizeChartAxisCrossBetween(chart.ValueAxisCrossBetween),
		SecondaryValueAxisCrossBetween:        officeNormalizeChartAxisCrossBetween(chart.SecondaryValueAxisCrossBetween),
		ValueAxisReverseOrder:                 officeNormalizeChartAxisReverseOrder(chart.ValueAxisReverseOrder),
		SecondaryValueAxisReverseOrder:        officeNormalizeChartAxisReverseOrder(chart.SecondaryValueAxisReverseOrder),
		ValueAxisLabelPosition:                officeNormalizeChartAxisLabelPosition(chart.ValueAxisLabelPosition),
		SecondaryValueAxisLabelPosition:       officeNormalizeChartAxisLabelPosition(chart.SecondaryValueAxisLabelPosition),
		ValueAxisMajorTickMark:                officeNormalizeChartTickMark(chart.ValueAxisMajorTickMark),
		ValueAxisMinorTickMark:                officeNormalizeChartTickMark(chart.ValueAxisMinorTickMark),
		SecondaryValueAxisMajorTickMark:       officeNormalizeChartTickMark(chart.SecondaryValueAxisMajorTickMark),
		SecondaryValueAxisMinorTickMark:       officeNormalizeChartTickMark(chart.SecondaryValueAxisMinorTickMark),
		ShowLegend:                            officeNormalizeChartLabels(chart.ShowLegend),
		LegendPosition:                        officeNormalizeChartLegendPosition(chart.LegendPosition),
		VaryColors:                            officeNormalizeChartLabels(chart.VaryColors),
		StartAngle:                            officeNormalizeChartStartAngle(chart.StartAngle),
		HoleSize:                              officeNormalizeChartHoleSize(chart.HoleSize),
		Smooth:                                officeNormalizeChartLabels(chart.Smooth),
		GapWidth:                              officeNormalizeChartGapWidth(chart.GapWidth),
		Overlap:                               officeNormalizeChartOverlap(chart.Overlap),
		Labels:                                officeNormalizeChartLabels(chart.Labels),
		LabelPosition:                         officeNormalizeChartLabelPosition(chart.LabelPosition),
		LabelFormat:                           officeNormalizeChartLabelFormat(chart.LabelFormat),
		LabelSeparator:                        officeNormalizeChartLabelSeparator(chart.LabelSeparator),
		ShowLeaderLines:                       officeNormalizeChartLabels(chart.ShowLeaderLines),
		ShowValue:                             officeNormalizeChartLabels(chart.ShowValue),
		ShowCategory:                          officeNormalizeChartLabels(chart.ShowCategory),
		ShowSeriesName:                        officeNormalizeChartLabels(chart.ShowSeriesName),
		ShowPercent:                           officeNormalizeChartLabels(chart.ShowPercent),
		ShowLegendKey:                         officeNormalizeChartLabels(chart.ShowLegendKey),
		ShowBubbleSize:                        officeNormalizeChartLabels(chart.ShowBubbleSize),
		Categories:                            make([]string, 0, len(chart.Categories)),
		Series:                                make([]officeChartSeries, 0, len(chart.Series)),
	}
	for _, category := range chart.Categories {
		value := officeCleanDocText(category)
		if value != "" {
			cleaned.Categories = append(cleaned.Categories, value)
		}
	}
	for idx, series := range chart.Series {
		name := officeCleanDocText(series.Name)
		if name == "" {
			name = firstNonEmptyOfficeString(cleaned.Title, "Series "+strconv.Itoa(idx+1))
		}
		values := append([]float64(nil), series.Values...)
		if len(values) == 0 {
			continue
		}
		seriesType := officeNormalizeChartSeriesType(cleaned.Type, series.Type, idx)
		cleaned.Series = append(cleaned.Series, officeChartSeries{
			Name:                 name,
			Type:                 seriesType,
			Axis:                 officeNormalizeChartSeriesAxis(cleaned.Type, seriesType, series.Axis),
			Labels:               officeNormalizeChartLabels(series.Labels),
			LabelPosition:        officeNormalizeChartLabelPosition(series.LabelPosition),
			LabelFormat:          officeNormalizeChartLabelFormat(series.LabelFormat),
			LabelSeparator:       officeNormalizeChartLabelSeparator(series.LabelSeparator),
			ShowLeaderLines:      officeNormalizeChartLabels(series.ShowLeaderLines),
			ShowValue:            officeNormalizeChartLabels(series.ShowValue),
			ShowCategory:         officeNormalizeChartLabels(series.ShowCategory),
			ShowSeriesName:       officeNormalizeChartLabels(series.ShowSeriesName),
			ShowPercent:          officeNormalizeChartLabels(series.ShowPercent),
			ShowLegendKey:        officeNormalizeChartLabels(series.ShowLegendKey),
			ShowBubbleSize:       officeNormalizeChartLabels(series.ShowBubbleSize),
			PointShowLabels:      officeNormalizeChartPointLabelVisibility(series.PointShowLabels),
			PointShowValues:      officeNormalizeChartPointLabelVisibility(series.PointShowValues),
			PointShowCategories:  officeNormalizeChartPointLabelVisibility(series.PointShowCategories),
			PointShowSeriesNames: officeNormalizeChartPointLabelVisibility(series.PointShowSeriesNames),
			PointShowPercents:    officeNormalizeChartPointLabelVisibility(series.PointShowPercents),
			PointLabelPositions:  officeNormalizeChartPointLabelPositions(series.PointLabelPositions),
			PointLabelFormats:    officeNormalizeChartPointLabelFormats(series.PointLabelFormats),
			PointLabelSeparators: officeNormalizeChartPointLabelSeparators(series.PointLabelSeparators),
			PointColors:          officeNormalizeChartPointColors(series.PointColors),
			PointExplosions:      officeNormalizeChartPointExplosions(series.PointExplosions),
			Smooth:               officeNormalizeChartLabels(series.Smooth),
			Color:                officeNormalizeChartSeriesColor(series.Color),
			LineWidth:            officeNormalizeChartSeriesLineWidth(series.LineWidth),
			Dash:                 officeNormalizeChartSeriesDash(series.Dash),
			Marker:               officeNormalizeChartSeriesMarker(series.Marker),
			Values:               values,
		})
	}
	if cleaned.Type == "" || len(cleaned.Series) == 0 {
		return nil
	}
	if len(cleaned.Categories) == 0 {
		maxLen := 0
		for _, series := range cleaned.Series {
			if len(series.Values) > maxLen {
				maxLen = len(series.Values)
			}
		}
		for idx := 0; idx < maxLen; idx++ {
			cleaned.Categories = append(cleaned.Categories, "Category "+strconv.Itoa(idx+1))
		}
	}
	return cleaned
}

func officeCleanupDocStringList(values []string) []string {
	out := make([]string, 0, len(values))
	fingerprints := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := officeDedupeAdjacentSentences(officeCleanDocText(value))
		if cleaned == "" {
			continue
		}
		fingerprint := officeDocFingerprint(cleaned)
		if fingerprint == "" {
			continue
		}
		if idx := officeFindDuplicateDocString(fingerprints, fingerprint); idx >= 0 {
			if officeDocQuality(cleaned) > officeDocQuality(out[idx]) {
				out[idx] = cleaned
				fingerprints[idx] = fingerprint
			}
			continue
		}
		out = append(out, cleaned)
		fingerprints = append(fingerprints, fingerprint)
	}
	return out
}

func officeCleanupDocBlocks(values []officeDocBlock) []officeDocBlock {
	out := make([]officeDocBlock, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		cleaned, ok := officeCleanDocBlock(value)
		if !ok {
			continue
		}
		key := string(cleaned.Kind) + "\x00" + cleaned.Text + "\x00" + cleaned.Source
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, cleaned)
	}
	return out
}

func officeCleanDocBlock(block officeDocBlock) (officeDocBlock, bool) {
	switch block.Kind {
	case officeDocBlockQuote:
		block.Text = officeNormalizeQuotedBlockText(block.Text)
	case officeDocBlockCode:
		block.Text = officeNormalizeCodeBlockText(block.Text)
	case officeDocBlockSeparator:
		if separator, ok := officeMarkdownThematicBreak(block.Text); ok {
			block.Text = separator
		} else {
			block.Text = "---"
		}
	case officeDocBlockImage:
		block.Text = officeCleanDocText(block.Text)
		block.Source = strings.TrimSpace(block.Source)
	default:
		block.Kind = officeDocBlockParagraph
		block.Text = officeCleanDocText(block.Text)
	}
	if block.Kind == officeDocBlockImage {
		return block, block.Source != ""
	}
	return block, block.Text != ""
}

func officeNormalizeQuotedBlockText(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, officeCleanDocText(line))
	}
	return officeTrimStructuredDocLines(out)
}

func officeNormalizeCodeBlockText(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, strings.TrimRight(line, " \t"))
	}
	return officeTrimStructuredDocLines(out)
}

func officeTrimStructuredDocLines(lines []string) string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

func officeFindDuplicateDocString(existing []string, candidate string) int {
	for idx, current := range existing {
		if current == candidate || officeDocNearDuplicate(current, candidate) {
			return idx
		}
	}
	return -1
}

func officeDocNearDuplicate(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	shorter, longer := left, right
	if runeCount(shorter) > runeCount(longer) {
		shorter, longer = longer, shorter
	}
	if runeCount(shorter) < 12 {
		return false
	}
	return strings.Contains(longer, shorter) && runeCount(shorter)*100 >= runeCount(longer)*88
}

func officeDocQuality(text string) int {
	score := runeCount(officeDocFingerprint(text)) * 10
	if last := officeLastNonSpaceRune(text); officeIsSentenceEnding(last) {
		score++
	}
	return score
}

func officeJoinDocParagraphs(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, "\n\n")
}

func officeCleanDocText(text string) string {
	text = officeNormalizeInlineWhitespace(text)
	if text == "" {
		return ""
	}
	previous := ""
	for previous != text {
		previous = text
		text = officeTrimDocSpacing(text)
		for strings.Contains(text, "【【") {
			text = strings.ReplaceAll(text, "【【", "【")
		}
		for strings.Contains(text, "】】") {
			text = strings.ReplaceAll(text, "】】", "】")
		}
		for strings.Contains(text, "【】") {
			text = strings.ReplaceAll(text, "【】", "")
		}
		text = officeNormalizeInlineWhitespace(text)
	}
	return text
}

func officeNormalizeInlineWhitespace(text string) string {
	var sb strings.Builder
	lastWasSpace := false
	for _, r := range text {
		switch {
		case strings.ContainsRune(officeZeroWidthChars, r):
			continue
		case r == '\u00a0' || r == '\u3000':
			r = ' '
		}
		if unicode.IsSpace(r) {
			if sb.Len() == 0 || lastWasSpace {
				continue
			}
			sb.WriteByte(' ')
			lastWasSpace = true
			continue
		}
		sb.WriteRune(r)
		lastWasSpace = false
	}
	return strings.TrimSpace(sb.String())
}

func officeTrimDocSpacing(text string) string {
	runes := []rune(text)
	var sb strings.Builder
	lastWritten := rune(0)
	for idx, r := range runes {
		if !unicode.IsSpace(r) {
			sb.WriteRune(r)
			lastWritten = r
			continue
		}
		next := officeNextNonSpaceRune(runes, idx+1)
		if officeShouldDropDocSpace(lastWritten, next) {
			continue
		}
		if sb.Len() == 0 || lastWritten == ' ' {
			continue
		}
		sb.WriteByte(' ')
		lastWritten = ' '
	}
	return strings.TrimSpace(sb.String())
}

func officeShouldDropDocSpace(prev, next rune) bool {
	if prev == 0 || next == 0 {
		return true
	}
	if officeIsOpenPunctuation(prev) || officeIsClosePunctuation(next) {
		return true
	}
	return officeIsCJK(prev) && officeIsCJK(next)
}

func officeDedupeAdjacentSentences(text string) string {
	sentences := officeSplitSentences(text)
	if len(sentences) <= 1 {
		return text
	}
	out := make([]string, 0, len(sentences))
	lastFingerprint := ""
	for _, sentence := range sentences {
		cleaned := officeNormalizeInlineWhitespace(sentence)
		if cleaned == "" {
			continue
		}
		fingerprint := officeDocFingerprint(cleaned)
		if fingerprint != "" && fingerprint == lastFingerprint {
			continue
		}
		out = append(out, cleaned)
		lastFingerprint = fingerprint
	}
	return officeJoinTextFragments(out)
}

func officeSplitSentences(text string) []string {
	var (
		out []string
		sb  strings.Builder
	)
	runes := []rune(text)
	flush := func() {
		if sentence := officeNormalizeInlineWhitespace(sb.String()); sentence != "" {
			out = append(out, sentence)
		}
		sb.Reset()
	}
	for idx, r := range runes {
		sb.WriteRune(r)
		if officeShouldSplitSentenceAt(runes, idx) {
			flush()
		}
	}
	flush()
	return out
}

func officeShouldSplitSentenceAt(runes []rune, idx int) bool {
	if idx < 0 || idx >= len(runes) {
		return false
	}
	r := runes[idx]
	if !officeIsSentenceEnding(r) {
		return false
	}
	if r == '.' {
		prev := officePrevNonSpaceRune(runes, idx-1)
		next := officeNextNonSpaceRune(runes, idx+1)
		if unicode.IsDigit(prev) && unicode.IsDigit(next) {
			return false
		}
	}
	return true
}

func officeDocFingerprint(text string) string {
	var sb strings.Builder
	for _, r := range text {
		switch {
		case unicode.IsLetter(r):
			sb.WriteRune(unicode.ToLower(r))
		case unicode.IsDigit(r):
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func officeJoinTextFragments(fragments []string) string {
	var sb strings.Builder
	for _, fragment := range fragments {
		part := officeNormalizeInlineWhitespace(fragment)
		if part == "" {
			continue
		}
		if sb.Len() == 0 {
			sb.WriteString(part)
			continue
		}
		prev := officeLastNonSpaceRune(sb.String())
		next := officeFirstNonSpaceRune(part)
		if officeShouldInsertFragmentSpace(prev, next) {
			sb.WriteByte(' ')
		}
		sb.WriteString(part)
	}
	return strings.TrimSpace(sb.String())
}

func officeShouldInsertFragmentSpace(prev, next rune) bool {
	if prev == 0 || next == 0 {
		return false
	}
	if officeIsOpenPunctuation(prev) || officeIsClosePunctuation(next) {
		return false
	}
	if officeIsSentenceEnding(prev) {
		return !officeIsCJK(next)
	}
	if officeIsCJK(prev) || officeIsCJK(next) {
		return false
	}
	return true
}

func officeFirstNonSpaceRune(text string) rune {
	for _, r := range text {
		if !unicode.IsSpace(r) {
			return r
		}
	}
	return 0
}

func officeLastNonSpaceRune(text string) rune {
	runes := []rune(text)
	for idx := len(runes) - 1; idx >= 0; idx-- {
		if !unicode.IsSpace(runes[idx]) {
			return runes[idx]
		}
	}
	return 0
}

func officeNextNonSpaceRune(runes []rune, start int) rune {
	for idx := start; idx < len(runes); idx++ {
		if !unicode.IsSpace(runes[idx]) {
			return runes[idx]
		}
	}
	return 0
}

func officePrevNonSpaceRune(runes []rune, start int) rune {
	for idx := start; idx >= 0; idx-- {
		if !unicode.IsSpace(runes[idx]) {
			return runes[idx]
		}
	}
	return 0
}

func officeIsSentenceEnding(r rune) bool {
	return strings.ContainsRune("。！？!?；;.:：", r)
}

func officeIsOpenPunctuation(r rune) bool {
	return strings.ContainsRune("([{<\"'“‘【《「『", r)
}

func officeIsClosePunctuation(r rune) bool {
	return strings.ContainsRune(")]}>\"'”’】》」』，。！？!?；;：:、,.", r)
}

func officeIsCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}
