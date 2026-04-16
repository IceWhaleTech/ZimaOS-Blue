package tools

import (
	"regexp"
	"strings"
	"testing"
)

func TestOfficePPTXSlideXMLIncludesNativeChart(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Revenue",
		Lines: []string{"Quarterly performance"},
		Chart: &officeChartSpec{
			Type:       "bar",
			Categories: []string{"North", "South"},
			Series: []officeChartSeries{
				{Name: "Revenue", Values: []float64{120, 98}},
			},
		},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart">`,
		`<c:chart r:id="rId2"/>`,
		`<a:t>Quarterly performance</a:t>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
}

func TestOfficeBuildPresentationSlidesExtractsChartCalloutsFromBullets(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Title: "Board Metrics",
		Sections: []officeDocSection{
			{
				Heading:    "Revenue Mix",
				Paragraphs: []string{"This longer narrative should remain in the body because it gives more context than a compact callout card should try to hold on the slide."},
				Bullets:    []string{"Revenue: $12.4M", "Watch conversion quality"},
				Chart: &officeChartSpec{
					Type:       "bar",
					Categories: []string{"Q1", "Q2"},
					Series: []officeChartSeries{
						{Name: "Revenue", Values: []float64{120, 132}},
					},
				},
			},
		},
	})

	var chartSlide *officePPTXSlide
	for idx := range slides {
		if slides[idx].Chart != nil {
			chartSlide = &slides[idx]
			break
		}
	}
	if chartSlide == nil {
		t.Fatalf("expected a chart slide in %#v", slides)
	}
	if len(chartSlide.Callouts) != 2 {
		t.Fatalf("len(callouts) = %d, want 2 (%#v)", len(chartSlide.Callouts), chartSlide.Callouts)
	}
	if chartSlide.Callouts[0].Label != "Revenue" || chartSlide.Callouts[0].Value != "$12.4M" {
		t.Fatalf("first callout = %#v, want Revenue/$12.4M", chartSlide.Callouts[0])
	}
	if chartSlide.Callouts[1].Body != "Watch conversion quality" || chartSlide.Callouts[1].Tone != "warning" {
		t.Fatalf("second callout = %#v, want warning body callout", chartSlide.Callouts[1])
	}
	if len(chartSlide.Blocks) != 1 {
		t.Fatalf("len(blocks) = %d, want 1 (%#v)", len(chartSlide.Blocks), chartSlide.Blocks)
	}
	if chartSlide.Blocks[0].Text != "This longer narrative should remain in the body because it gives more context than a compact callout card should try to hold on the slide." {
		t.Fatalf("remaining block = %#v", chartSlide.Blocks[0])
	}
}

func TestCleanupOfficeDocSpecPreservesDecimalCalloutBullets(t *testing.T) {
	cleaned := cleanupOfficeDocSpec(officeDocSpec{
		Sections: []officeDocSection{
			{
				Heading: "Revenue Mix",
				Bullets: []string{"Revenue: $12.4M", "Margin: 31.5%"},
			},
		},
	})

	if got := cleaned.Sections[0].Bullets[0]; got != "Revenue: $12.4M" {
		t.Fatalf("cleaned bullet[0] = %q, want Revenue: $12.4M", got)
	}
	if got := cleaned.Sections[0].Bullets[1]; got != "Margin: 31.5%" {
		t.Fatalf("cleaned bullet[1] = %q, want Margin: 31.5%%", got)
	}
}

func TestOfficeBuildPresentationSlidesSkipsTOCWhenDeckHasNoCoverMetadata(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Sections: []officeDocSection{
			{Heading: "Core Upgrades", Bullets: []string{"MoE architecture", "Long context"}},
			{Heading: "Benchmarks", Bullets: []string{"AIME", "LiveCodeBench"}},
			{Heading: "Use Cases", Bullets: []string{"Coding", "Agents"}},
		},
	})

	if len(slides) != 3 {
		t.Fatalf("len(slides) = %d, want 3 (%#v)", len(slides), slides)
	}
	if slides[0].Title == "Table of Contents" {
		t.Fatalf("first slide should not be TOC when deck has no title/subtitle/summary: %#v", slides)
	}
}

func TestOfficeBuildPresentationSlidesExtractsStandaloneCalloutsFromShortBullets(t *testing.T) {
	slides := officeBuildPresentationSlides(officeDocSpec{
		Title: "Qwen 3.5",
		Sections: []officeDocSection{
			{
				Heading: "Core Upgrades",
				Bullets: []string{
					"MoE architecture",
					"36T training tokens",
					"Hybrid reasoning",
					"128K context window",
				},
			},
		},
	})

	var target *officePPTXSlide
	for idx := range slides {
		if slides[idx].Title == "Core Upgrades" {
			target = &slides[idx]
			break
		}
	}
	if target == nil {
		t.Fatalf("expected Core Upgrades slide in %#v", slides)
	}
	if len(target.Callouts) != 4 {
		t.Fatalf("len(callouts) = %d, want 4 (%#v)", len(target.Callouts), target.Callouts)
	}
	if len(target.Blocks) != 0 {
		t.Fatalf("len(blocks) = %d, want 0 once short bullets become callouts (%#v)", len(target.Blocks), target.Blocks)
	}
	if target.Callouts[0].Body != "MoE architecture" {
		t.Fatalf("first callout = %#v, want body callout", target.Callouts[0])
	}
}

func TestOfficePPTXSlideXMLUsesStandaloneCalloutCardsWithoutBackgroundPatch(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Core Upgrades",
		Theme: resolveOfficeTheme("analysis", ""),
		Callouts: []officePPTXCallout{
			{Body: "MoE architecture", Tone: "primary"},
			{Body: "36T training tokens", Tone: "success"},
			{Body: "Hybrid reasoning", Tone: "warning"},
			{Body: "128K context window", Tone: "muted"},
		},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`name="Standalone Callout 1"`,
		`name="Standalone Callout 4"`,
		`<a:t>MoE architecture</a:t>`,
		`<a:t>36T training tokens</a:t>`,
		`<a:t>Hybrid reasoning</a:t>`,
		`<a:t>128K context window</a:t>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
	if containsSubstring(xml, `name="Standalone Callout Grid"`) {
		t.Fatalf("standalone callouts should not render a large background patch in %s", xml)
	}
	if containsSubstring(xml, `name="Content"`) && containsSubstring(xml, `<a:t>MoE architecture</a:t></a:r></a:p></p:txBody></p:sp><p:sp><p:nvSpPr><p:cNvPr id="60"`) {
		t.Fatalf("standalone callout text should not be duplicated in the default content body: %s", xml)
	}
}

func TestOfficePPTXSlideXMLIncludesThemeAwareChartCalloutRail(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Revenue Mix",
		Theme: resolveOfficeTheme("midnight", ""),
		Chart: &officeChartSpec{
			Type:       "bar",
			Categories: []string{"Q1", "Q2"},
			Series: []officeChartSeries{
				{Name: "Revenue", Values: []float64{120, 132}},
			},
		},
		Callouts: []officePPTXCallout{
			{Label: "Revenue", Value: "$12.4M", Tone: "primary"},
			{Body: "Watch conversion quality", Tone: "warning"},
		},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`name="Chart Callout Rail"`,
		`name="Chart Callout 1"`,
		`name="Chart Callout 2"`,
		`<a:t>Key Takeaways</a:t>`,
		`<a:t>Revenue</a:t>`,
		`<a:t>$12.4M</a:t>`,
		`<a:t>Watch conversion quality</a:t>`,
		`typeface="Helvetica Neue"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
		`val="4D7CFE"`,
		`val="C89A45"`,
		`cx="8031480"`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
	if containsSubstring(xml, `name="Chart Callout Accent"`) {
		t.Fatalf("chart callout column should not render a separate accent patch in %s", xml)
	}
}

func TestOfficePPTXSlideXMLLocalizesChineseChartCalloutRailTitle(t *testing.T) {
	slide := officePPTXSlide{
		Title: "趋势对比",
		Theme: resolveOfficeTheme("midnight", ""),
		Chart: &officeChartSpec{
			Type:       "bar",
			Categories: []string{"Q1", "Q2"},
			Series: []officeChartSeries{
				{Name: "准确率", Values: []float64{72, 88}},
			},
		},
		Callouts: []officePPTXCallout{
			{Label: "准确率", Value: "93%", Tone: "primary"},
			{Label: "重点", Body: "首轮响应更快", Tone: "primary"},
		},
	}

	xml := officePPTXSlideXML(slide)
	if !containsSubstring(xml, `<a:t>关键要点</a:t>`) {
		t.Fatalf("expected Chinese mixed callout rail title in %s", xml)
	}
	if containsSubstring(xml, `<a:t>Key Takeaways</a:t>`) {
		t.Fatalf("expected Chinese mixed callout rail title, got English in %s", xml)
	}
}

func TestOfficePPTXSlideXMLLocalizesChineseChartMetricsRailTitle(t *testing.T) {
	slide := officePPTXSlide{
		Title: "趋势对比",
		Theme: resolveOfficeTheme("midnight", ""),
		Chart: &officeChartSpec{
			Type:       "bar",
			Categories: []string{"Q1", "Q2"},
			Series: []officeChartSeries{
				{Name: "准确率", Values: []float64{72, 88}},
			},
		},
		Callouts: []officePPTXCallout{
			{Label: "准确率", Value: "93%", Tone: "primary"},
			{Label: "延迟", Value: "180ms", Tone: "primary"},
		},
	}

	xml := officePPTXSlideXML(slide)
	if !containsSubstring(xml, `<a:t>关键指标</a:t>`) {
		t.Fatalf("expected Chinese metrics rail title in %s", xml)
	}
	if containsSubstring(xml, `<a:t>Key Metrics</a:t>`) {
		t.Fatalf("expected Chinese metrics rail title, got English in %s", xml)
	}
}

func TestBuildOfficePPTXOmitsRedundantChartTitleWhenSlideHeadingExists(t *testing.T) {
	pptxData, _, err := buildOfficePPTX(officeDocSpec{
		Theme: resolveOfficeTheme("midnight", ""),
		Sections: []officeDocSection{
			{
				Heading: "趋势对比",
				Chart: &officeChartSpec{
					Type:       "combo",
					Title:      "关键指标趋势",
					Categories: []string{"Base", "v1", "v2", "v3"},
					Series: []officeChartSeries{
						{Name: "准确率", Type: "bar", Values: []float64{72, 81, 88, 93}},
						{Name: "延迟", Type: "line", Axis: "secondary", Values: []float64{310, 250, 210, 180}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOfficePPTX returned error: %v", err)
	}

	slideXML := officeZipEntryText(t, pptxData, "ppt/slides/slide1.xml")
	chartXML := officeZipEntryText(t, pptxData, "ppt/charts/chart1.xml")

	if !containsSubstring(slideXML, `<a:t>趋势对比</a:t>`) {
		t.Fatalf("expected slide heading to remain visible in %s", slideXML)
	}
	if containsSubstring(chartXML, `<a:t>关键指标趋势</a:t>`) {
		t.Fatalf("expected chart1.xml to omit redundant in-chart title, got %s", chartXML)
	}
	if containsSubstring(chartXML, `<c:autoTitleDeleted val="0"/>`) {
		t.Fatalf("expected chart1.xml to mark the in-chart title as deleted, got %s", chartXML)
	}
}

func TestParseOfficeDocSectionsPreservesCompactMetricLinesForChartCallouts(t *testing.T) {
	sections, err := parseOfficeDocSections([]interface{}{
		map[string]interface{}{
			"heading": "趋势对比",
			"body":    "准确率：93%\n延迟：180ms\n重点：首轮响应更快",
			"chart": map[string]interface{}{
				"type":       "combo",
				"categories": []interface{}{"Base", "v1", "v2", "v3"},
				"series": []interface{}{
					map[string]interface{}{
						"name":   "准确率",
						"type":   "bar",
						"values": []interface{}{72, 81, 88, 93},
					},
					map[string]interface{}{
						"name":   "延迟",
						"type":   "line",
						"axis":   "secondary",
						"values": []interface{}{310, 250, 210, 180},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeDocSections returned error: %v", err)
	}

	slides := officeBuildPresentationSlides(officeDocSpec{Sections: sections})
	if len(slides) != 1 {
		t.Fatalf("len(slides) = %d, want 1 (%#v)", len(slides), slides)
	}
	chartSlide := slides[0]
	if chartSlide.Chart == nil {
		t.Fatalf("expected a chart slide in %#v", slides)
	}
	if len(chartSlide.Callouts) != 3 {
		t.Fatalf("len(callouts) = %d, want 3 (%#v)", len(chartSlide.Callouts), chartSlide.Callouts)
	}
	if chartSlide.Callouts[0].Label != "准确率" || chartSlide.Callouts[0].Value != "93%" {
		t.Fatalf("first callout = %#v, want 准确率/93%%", chartSlide.Callouts[0])
	}
	if chartSlide.Callouts[1].Label != "延迟" || chartSlide.Callouts[1].Value != "180ms" {
		t.Fatalf("second callout = %#v, want 延迟/180ms", chartSlide.Callouts[1])
	}
	if chartSlide.Callouts[2].Label != "重点" || chartSlide.Callouts[2].Value != "" || chartSlide.Callouts[2].Body != "首轮响应更快" {
		t.Fatalf("third callout = %#v, want descriptive body callout", chartSlide.Callouts[2])
	}
	if len(chartSlide.Blocks) != 0 {
		t.Fatalf("len(blocks) = %d, want 0 once compact metrics become callouts (%#v)", len(chartSlide.Blocks), chartSlide.Blocks)
	}
}

func TestOfficePPTXChartCalloutColorsUseUnifiedSurfaceForDarkThemes(t *testing.T) {
	theme := resolveOfficeTheme("midnight", "")
	for _, tc := range []struct {
		name        string
		callout     officePPTXCallout
		wantFill    string
		wantLine    string
		wantLabel   string
		wantValue   string
		wantBody    string
	}{
		{
			name:      "primary",
			callout:   officePPTXCallout{Label: "Revenue", Value: "$12.4M", Tone: "primary"},
			wantFill:  theme.Surface,
			wantLine:  theme.Accent,
			wantLabel: theme.Accent,
			wantValue: theme.PrimaryDark,
			wantBody:  theme.Slate,
		},
		{
			name:      "warning",
			callout:   officePPTXCallout{Body: "Watch conversion quality", Tone: "warning"},
			wantFill:  theme.Surface,
			wantLine:  theme.Warning,
			wantLabel: theme.Warning,
			wantValue: theme.PrimaryDark,
			wantBody:  theme.PrimaryDark,
		},
		{
			name:      "muted",
			callout:   officePPTXCallout{Body: "Long context", Tone: "muted"},
			wantFill:  theme.Surface,
			wantLine:  theme.Border,
			wantLabel: theme.Secondary,
			wantValue: theme.PrimaryDark,
			wantBody:  theme.Slate,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fill, line, label, value, body := officePPTXChartCalloutColors(tc.callout, theme)
			if fill != tc.wantFill || line != tc.wantLine || label != tc.wantLabel || value != tc.wantValue || body != tc.wantBody {
				t.Fatalf("colors = %q/%q/%q/%q/%q, want %q/%q/%q/%q/%q", fill, line, label, value, body, tc.wantFill, tc.wantLine, tc.wantLabel, tc.wantValue, tc.wantBody)
			}
		})
	}
}

func TestOfficePPTXChartXMLIncludesBarChartSeries(t *testing.T) {
	chartXML := officePPTXChartXML(officeChartSpec{
		Type:       "bar",
		Categories: []string{"North", "South"},
		Series: []officeChartSeries{
			{Name: "Revenue", Values: []float64{120, 98}},
		},
	})
	for _, needle := range []string{
		`<c:barChart>`,
		`<c:barDir val="col"/>`,
		`<c:v>Revenue</c:v>`,
		`<c:v>North</c:v>`,
		`<c:v>120</c:v>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesLineChartSeries(t *testing.T) {
	chartXML := officePPTXChartXML(officeChartSpec{
		Type:       "line",
		Categories: []string{"Jan", "Feb", "Mar"},
		Series: []officeChartSeries{
			{Name: "Trend", Values: []float64{10, 12, 15}},
		},
	})
	for _, needle := range []string{
		`<c:lineChart>`,
		`<c:grouping val="standard"/>`,
		`<c:marker><c:symbol val="circle"/></c:marker>`,
		`<c:v>Trend</c:v>`,
		`<c:v>Mar</c:v>`,
		`<c:v>15</c:v>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("line chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficeBuildPPTXChartPackageIncludesEmbeddedWorkbookForStringCategories(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "bar",
		"categories": []interface{}{"North", "South"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 98}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartPackage, err := officeBuildPPTXChartPackage(*chart, 1)
	if err != nil {
		t.Fatalf("officeBuildPPTXChartPackage() error = %v", err)
	}
	if chartPackage.Workbook == nil {
		t.Fatal("expected chart package to include embedded workbook for string categories")
	}
	if chartPackage.ChartRelsXML == "" {
		t.Fatal("expected chart package to include chart relationships xml")
	}

	for _, needle := range []string{
		`<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>`,
		`<c:cat><c:strRef><c:f>Data!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>North</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>South</c:v></c:pt>`,
		`<c:val><c:numRef><c:f>Data!$B$2:$B$3</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="2"/>`,
	} {
		if !containsSubstring(chartPackage.ChartXML, needle) {
			t.Fatalf("string-category chart package XML missing %q in %s", needle, chartPackage.ChartXML)
		}
	}
	if containsSubstring(chartPackage.ChartXML, `<c:strLit>`) {
		t.Fatalf("expected string-category chart package XML to avoid strLit in %s", chartPackage.ChartXML)
	}
	if !containsSubstring(chartPackage.ChartRelsXML, `Target="../embeddings/Microsoft_Excel_Worksheet1.xlsx"`) {
		t.Fatalf("expected chart relationships to target embedded workbook, got %s", chartPackage.ChartRelsXML)
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryDateCategoryAxis(t *testing.T) {
	firstSerial, ok := officeExcelDateSerial("2026-01-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-01-01")
	}
	secondSerial, ok := officeExcelDateSerial("2026-02-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-02-01")
	}

	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "line",
		"x_axis_type": "date",
		"categories":  []interface{}{"2026-01-01", "2026-02-01"},
		"series": []interface{}{
			map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dateAx>`,
		`<c:cat><c:numLit><c:formatCode>yyyy-mm-dd</c:formatCode><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>`,
		`<c:baseTimeUnit val="days"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("date-axis chart XML missing %q in %s", needle, chartXML)
		}
	}
	if containsSubstring(chartXML, `<c:catAx>`) {
		t.Fatalf("expected primary category axis to switch away from catAx in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryDateTimeCategoryAxis(t *testing.T) {
	firstSerial, ok := officeExcelDateSerial("2026-01-01 09:30", true)
	if !ok {
		t.Fatal("expected date-time serial for 2026-01-01 09:30")
	}
	secondSerial, ok := officeExcelDateSerial("2026-01-01 15:45", true)
	if !ok {
		t.Fatal("expected date-time serial for 2026-01-01 15:45")
	}

	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "line",
		"x_axis_type": "datetime",
		"categories":  []interface{}{"2026-01-01 09:30", "2026-01-01 15:45"},
		"series": []interface{}{
			map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dateAx>`,
		`<c:cat><c:numLit><c:formatCode>yyyy-mm-dd hh:mm</c:formatCode><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("date-time-axis chart XML missing %q in %s", needle, chartXML)
		}
	}
	if containsSubstring(chartXML, `<c:catAx>`) {
		t.Fatalf("expected primary date-time category axis to switch away from catAx in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryDateCategoryAxisBaseTimeUnit(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                  "line",
		"x_axis_type":           "date",
		"x_axis_base_time_unit": "months",
		"categories":            []interface{}{"2026-01-01", "2026-02-01"},
		"series":                []interface{}{map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if !containsSubstring(chartXML, `<c:baseTimeUnit val="months"/>`) {
		t.Fatalf("expected date-axis chart XML to include months baseTimeUnit in %s", chartXML)
	}
	if containsSubstring(chartXML, `<c:baseTimeUnit val="days"/>`) {
		t.Fatalf("expected date-axis chart XML to override default days baseTimeUnit in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryDateCategoryAxisMajorMinorTimeUnits(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                   "line",
		"x_axis_type":            "date",
		"x_axis_major_time_unit": "years",
		"x_axis_minor_time_unit": "months",
		"categories":             []interface{}{"2026-01-01", "2027-01-01"},
		"series":                 []interface{}{map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:majorTimeUnit val="years"/>`,
		`<c:minorTimeUnit val="months"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected date-axis chart XML to include %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryDateCategoryAxisMajorMinorUnits(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":              "line",
		"x_axis_type":       "date",
		"x_axis_major_unit": 12,
		"x_axis_minor_unit": 1,
		"categories":        []interface{}{"2026-01-01", "2027-01-01"},
		"series":            []interface{}{map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:majorUnit val="12"/>`,
		`<c:minorUnit val="1"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected date-axis chart XML to include %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryDateCategoryAxisBounds(t *testing.T) {
	minSerial, ok := officeExcelDateSerial("2025-12-01", false)
	if !ok {
		t.Fatal("expected date serial for 2025-12-01")
	}
	maxSerial, ok := officeExcelDateSerial("2026-12-31", false)
	if !ok {
		t.Fatal("expected date serial for 2026-12-31")
	}

	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "line",
		"x_axis_type": "date",
		"x_axis_min":  "2025-12-01",
		"x_axis_max":  "2026-12-31",
		"categories":  []interface{}{"2026-01-01", "2027-01-01"},
		"series":      []interface{}{map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:min val="` + minSerial + `"/>`,
		`<c:max val="` + maxSerial + `"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected date-axis chart XML to include %q in %s", needle, chartXML)
		}
	}
}

func TestParseOfficeChartRejectsInvalidDateCategoryAxisBounds(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]interface{}
		wantErr string
	}{
		{
			name: "invalid min date",
			payload: map[string]interface{}{
				"type":        "line",
				"x_axis_type": "date",
				"x_axis_min":  "not-a-date",
				"categories":  []interface{}{"2026-01-01", "2027-01-01"},
				"series":      []interface{}{map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}}},
			},
			wantErr: "x_axis_min must be an ISO-like date or Excel serial",
		},
		{
			name: "inverted date bounds",
			payload: map[string]interface{}{
				"type":        "line",
				"x_axis_type": "date",
				"x_axis_min":  "2028-01-01",
				"x_axis_max":  "2026-01-01",
				"categories":  []interface{}{"2026-01-01", "2027-01-01"},
				"series":      []interface{}{map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}}},
			},
			wantErr: "x_axis_min must be less than or equal to x_axis_max",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseOfficeChart(tt.payload)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parseOfficeChart() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestOfficePPTXChartXMLIncludesComboDateCategoryAxes(t *testing.T) {
	firstSerial, ok := officeExcelDateSerial("2026-01-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-01-01")
	}
	thirdSerial, ok := officeExcelDateSerial("2026-03-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-03-01")
	}

	chart, err := parseOfficeChart(map[string]interface{}{
		"type":          "combo",
		"x_axis_type":   "date",
		"x2_axis_title": "Month (Top)",
		"categories":    []interface{}{"2026-01-01", "2026-02-01", "2026-03-01"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if strings.Count(chartXML, `<c:dateAx>`) != 2 {
		t.Fatalf("expected 2 date axes in %s", chartXML)
	}
	if strings.Count(chartXML, `<c:catAx>`) != 0 {
		t.Fatalf("expected combo date-axis chart to avoid catAx in %s", chartXML)
	}
	for _, needle := range []string{
		`<a:t>Month (Top)</a:t>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="2"><c:v>` + thirdSerial + `</c:v></c:pt>`,
		`<c:axPos val="t"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("combo date-axis chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboDateTimeCategoryAxes(t *testing.T) {
	firstSerial, ok := officeExcelDateSerial("2026-01-01 09:30", true)
	if !ok {
		t.Fatal("expected date-time serial for 2026-01-01 09:30")
	}
	thirdSerial, ok := officeExcelDateSerial("2026-01-01 18:15", true)
	if !ok {
		t.Fatal("expected date-time serial for 2026-01-01 18:15")
	}

	chart, err := parseOfficeChart(map[string]interface{}{
		"type":          "combo",
		"x_axis_type":   "datetime",
		"x2_axis_title": "Time (Top)",
		"categories":    []interface{}{"2026-01-01 09:30", "2026-01-01 13:00", "2026-01-01 18:15"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if strings.Count(chartXML, `<c:dateAx>`) != 2 {
		t.Fatalf("expected 2 date axes in %s", chartXML)
	}
	if strings.Count(chartXML, `<c:catAx>`) != 0 {
		t.Fatalf("expected combo datetime-axis chart to avoid catAx in %s", chartXML)
	}
	for _, needle := range []string{
		`<a:t>Time (Top)</a:t>`,
		`<c:cat><c:numLit><c:formatCode>yyyy-mm-dd hh:mm</c:formatCode><c:ptCount val="3"/>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="2"><c:v>` + thirdSerial + `</c:v></c:pt>`,
		`<c:axPos val="t"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("combo datetime-axis chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboDateCategoryAxesBounds(t *testing.T) {
	minSerial, ok := officeExcelDateSerial("2025-12-01", false)
	if !ok {
		t.Fatal("expected date serial for 2025-12-01")
	}
	maxSerial, ok := officeExcelDateSerial("2028-12-31", false)
	if !ok {
		t.Fatal("expected date serial for 2028-12-31")
	}

	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "combo",
		"x_axis_type": "date",
		"x_axis_min":  "2025-12-01",
		"x_axis_max":  "2028-12-31",
		"categories":  []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if strings.Count(chartXML, `<c:min val="`+minSerial+`"/>`) != 2 {
		t.Fatalf("expected combo date-axis chart XML to include 2 min bounds in %s", chartXML)
	}
	if strings.Count(chartXML, `<c:max val="`+maxSerial+`"/>`) != 2 {
		t.Fatalf("expected combo date-axis chart XML to include 2 max bounds in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesComboDateCategoryAxesMajorMinorUnits(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":              "combo",
		"x_axis_type":       "date",
		"x_axis_major_unit": 12,
		"x_axis_minor_unit": 1,
		"categories":        []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if strings.Count(chartXML, `<c:majorUnit val="12"/>`) != 2 {
		t.Fatalf("expected combo date-axis chart XML to include 2 majorUnit nodes in %s", chartXML)
	}
	if strings.Count(chartXML, `<c:minorUnit val="1"/>`) != 2 {
		t.Fatalf("expected combo date-axis chart XML to include 2 minorUnit nodes in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesComboDateCategoryAxesMajorMinorTimeUnits(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                   "combo",
		"x_axis_type":            "date",
		"x_axis_major_time_unit": "years",
		"x_axis_minor_time_unit": "months",
		"categories":             []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if strings.Count(chartXML, `<c:majorTimeUnit val="years"/>`) != 2 {
		t.Fatalf("expected combo date-axis chart XML to include 2 year majorTimeUnit nodes in %s", chartXML)
	}
	if strings.Count(chartXML, `<c:minorTimeUnit val="months"/>`) != 2 {
		t.Fatalf("expected combo date-axis chart XML to include 2 month minorTimeUnit nodes in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesComboDateCategoryAxesBaseTimeUnit(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                  "combo",
		"x_axis_type":           "date",
		"x_axis_base_time_unit": "years",
		"categories":            []interface{}{"2026-01-01", "2026-02-01", "2026-03-01"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if strings.Count(chartXML, `<c:baseTimeUnit val="years"/>`) != 2 {
		t.Fatalf("expected combo date-axis chart XML to include 2 year baseTimeUnit nodes in %s", chartXML)
	}
	if containsSubstring(chartXML, `<c:baseTimeUnit val="days"/>`) {
		t.Fatalf("expected combo date-axis chart XML to override default days baseTimeUnit in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesStackedBarSeries(t *testing.T) {
	chartXML := officePPTXChartXML(officeChartSpec{
		Type:       "stacked_bar",
		Categories: []string{"North", "South"},
		Series: []officeChartSeries{
			{Name: "Actual", Values: []float64{120, 98}},
			{Name: "Target", Values: []float64{140, 110}},
		},
	})
	for _, needle := range []string{
		`<c:barChart>`,
		`<c:barDir val="bar"/>`,
		`<c:grouping val="stacked"/>`,
		`<c:v>Actual</c:v>`,
		`<c:v>Target</c:v>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("stacked bar chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesPercentStackedColumnSeries(t *testing.T) {
	chartXML := officePPTXChartXML(officeChartSpec{
		Type:       "percent_stacked_column",
		Categories: []string{"Q1", "Q2"},
		Series: []officeChartSeries{
			{Name: "Won", Values: []float64{60, 55}},
			{Name: "Lost", Values: []float64{40, 45}},
		},
	})
	for _, needle := range []string{
		`<c:barChart>`,
		`<c:barDir val="col"/>`,
		`<c:grouping val="percentStacked"/>`,
		`<c:v>Won</c:v>`,
		`<c:v>Lost</c:v>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("percent stacked column chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesPieSeries(t *testing.T) {
	chartXML := officePPTXChartXML(officeChartSpec{
		Type:       "pie",
		Categories: []string{"Won", "Lost", "Open"},
		Series: []officeChartSeries{
			{Name: "Pipeline", Values: []float64{55, 25, 20}},
		},
	})
	for _, needle := range []string{
		`<c:pieChart>`,
		`<c:varyColors val="1"/>`,
		`<c:v>Pipeline</c:v>`,
		`<c:v>Won</c:v>`,
		`<c:v>55</c:v>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("pie chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesDonutSeries(t *testing.T) {
	chartXML := officePPTXChartXML(officeChartSpec{
		Type:       "donut",
		Categories: []string{"Adoption", "Pending", "Blocked"},
		Series: []officeChartSeries{
			{Name: "Status", Values: []float64{70, 20, 10}},
		},
	})
	for _, needle := range []string{
		`<c:doughnutChart>`,
		`<c:varyColors val="1"/>`,
		`<c:holeSize val="50"/>`,
		`<c:v>Status</c:v>`,
		`<c:v>Adoption</c:v>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("donut chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboSeries(t *testing.T) {
	chartXML := officePPTXChartXML(officeChartSpec{
		Type:       "combo",
		Categories: []string{"Q1", "Q2", "Q3"},
		Series: []officeChartSeries{
			{Name: "Revenue", Type: "bar", Values: []float64{120, 132, 140}},
			{Name: "Margin", Type: "line", Values: []float64{28, 31, 34}},
		},
	})
	for _, needle := range []string{
		`<c:barChart>`,
		`<c:lineChart>`,
		`<c:v>Revenue</c:v>`,
		`<c:v>Margin</c:v>`,
		`<c:v>Q3</c:v>`,
		`<c:v>34</c:v>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("combo chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboSecondaryAxisSeries(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:barChart>`,
		`<c:lineChart>`,
		`<c:axPos val="r"/>`,
		`<c:axPos val="t"/>`,
		`<c:v>Margin</c:v>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("combo secondary-axis chart XML missing %q in %s", needle, chartXML)
		}
	}
	if strings.Count(chartXML, `<c:valAx>`) != 2 {
		t.Fatalf("expected 2 value axes in %s", chartXML)
	}
	if strings.Count(chartXML, `<c:catAx>`) != 2 {
		t.Fatalf("expected 2 category axes in %s", chartXML)
	}

	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	axisPattern := regexp.MustCompile(`<c:axId val="(\d+)"/>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	barAxisIDs := axisPattern.FindAllStringSubmatch(blocks[0], -1)
	lineAxisIDs := axisPattern.FindAllStringSubmatch(blocks[1], -1)
	if len(barAxisIDs) != 2 || len(lineAxisIDs) != 2 {
		t.Fatalf("expected 2 axis ids per chart block in %s", chartXML)
	}
	if barAxisIDs[0][1] == lineAxisIDs[0][1] && barAxisIDs[1][1] == lineAxisIDs[1][1] {
		t.Fatalf("expected combo secondary axis to bind line series to different axes in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesAxisTitles(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":         "bar",
		"x_axis_title": "Quarter",
		"y_axis_title": "Revenue ($M)",
		"categories":   []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:catAx>`,
		`<c:valAx>`,
		`<a:t>Quarter</a:t>`,
		`<a:t>Revenue ($M)</a:t>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("axis-title chart XML missing %q in %s", needle, chartXML)
		}
	}
	if strings.Count(chartXML, `<c:title>`) != 2 {
		t.Fatalf("expected 2 axis titles in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesComboSecondaryAxisTitle(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":          "combo",
		"y_axis_title":  "Revenue ($M)",
		"y2_axis_title": "Margin %",
		"categories":    []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:axPos val="l"/>`,
		`<c:axPos val="r"/>`,
		`<a:t>Revenue ($M)</a:t>`,
		`<a:t>Margin %</a:t>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("combo axis-title chart XML missing %q in %s", needle, chartXML)
		}
	}
	if strings.Count(chartXML, `<c:title>`) != 2 {
		t.Fatalf("expected 2 value-axis titles in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesComboSecondaryCategoryAxisTitle(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":          "combo",
		"x2_axis_title": "Quarter (Top)",
		"categories":    []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:axPos val="t"/>`,
		`<a:t>Quarter (Top)</a:t>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("combo secondary category-axis title XML missing %q in %s", needle, chartXML)
		}
	}
	if strings.Count(chartXML, `<c:title>`) != 1 {
		t.Fatalf("expected 1 secondary category-axis title in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryValueAxisFormat(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":          "bar",
		"y_axis_format": "$#,##0",
		"categories":    []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if !containsSubstring(chartXML, `<c:valAx>`) || !containsSubstring(chartXML, `<c:numFmt formatCode="$#,##0" sourceLinked="0"/>`) {
		t.Fatalf("primary axis format chart XML missing custom numFmt in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryValueAxisFormat(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":           "combo",
		"y_axis_format":  "$#,##0",
		"y2_axis_format": "0.0%",
		"categories":     []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected 2 value-axis blocks in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:axPos val="l"/>`) || !containsSubstring(valueAxisBlocks[0], `<c:numFmt formatCode="$#,##0" sourceLinked="0"/>`) {
		t.Fatalf("expected primary value axis format in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:axPos val="r"/>`) || !containsSubstring(valueAxisBlocks[1], `<c:numFmt formatCode="0.0%" sourceLinked="0"/>`) {
		t.Fatalf("expected secondary value axis format in %s", valueAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryValueAxisBounds(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "bar",
		"y_axis_min": 0,
		"y_axis_max": 200,
		"categories": []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:valAx>`,
		`<c:min val="0"/>`,
		`<c:max val="200"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("primary axis bounds chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryValueAxisBounds(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "combo",
		"y_axis_min":  0,
		"y_axis_max":  200,
		"y2_axis_min": 0,
		"y2_axis_max": 40,
		"categories":  []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected 2 value-axis blocks in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:axPos val="l"/>`) || !containsSubstring(valueAxisBlocks[0], `<c:min val="0"/>`) || !containsSubstring(valueAxisBlocks[0], `<c:max val="200"/>`) {
		t.Fatalf("expected primary value axis bounds in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:axPos val="r"/>`) || !containsSubstring(valueAxisBlocks[1], `<c:min val="0"/>`) || !containsSubstring(valueAxisBlocks[1], `<c:max val="40"/>`) {
		t.Fatalf("expected secondary value axis bounds in %s", valueAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryValueAxisUnits(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":              "bar",
		"y_axis_major_unit": 25,
		"y_axis_minor_unit": 5,
		"categories":        []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:valAx>`,
		`<c:majorUnit val="25"/>`,
		`<c:minorUnit val="5"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("primary axis units chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryValueAxisUnits(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":               "combo",
		"y_axis_major_unit":  25,
		"y_axis_minor_unit":  5,
		"y2_axis_major_unit": 10,
		"y2_axis_minor_unit": 2,
		"categories":         []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected 2 value-axis blocks in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:axPos val="l"/>`) || !containsSubstring(valueAxisBlocks[0], `<c:majorUnit val="25"/>`) || !containsSubstring(valueAxisBlocks[0], `<c:minorUnit val="5"/>`) {
		t.Fatalf("expected primary value axis units in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:axPos val="r"/>`) || !containsSubstring(valueAxisBlocks[1], `<c:majorUnit val="10"/>`) || !containsSubstring(valueAxisBlocks[1], `<c:minorUnit val="2"/>`) {
		t.Fatalf("expected secondary value axis units in %s", valueAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryValueAxisTickGridControls(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                   "bar",
		"y_axis_major_gridlines": false,
		"y_axis_major_tick_mark": "cross",
		"y_axis_minor_tick_mark": "in",
		"categories":             []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 1 {
		t.Fatalf("expected 1 value-axis block in %s", chartXML)
	}
	if containsSubstring(valueAxisBlocks[0], `<c:majorGridlines`) {
		t.Fatalf("expected primary value axis to omit major gridlines in %s", valueAxisBlocks[0])
	}
	for _, needle := range []string{
		`<c:majorTickMark val="cross"/>`,
		`<c:minorTickMark val="in"/>`,
	} {
		if !containsSubstring(valueAxisBlocks[0], needle) {
			t.Fatalf("primary value axis tick/grid XML missing %q in %s", needle, valueAxisBlocks[0])
		}
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryValueAxisTickGridControls(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                    "combo",
		"y_axis_major_gridlines":  false,
		"y_axis_major_tick_mark":  "none",
		"y_axis_minor_tick_mark":  "none",
		"y2_axis_major_gridlines": true,
		"y2_axis_major_tick_mark": "cross",
		"y2_axis_minor_tick_mark": "in",
		"categories":              []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected 2 value-axis blocks in %s", chartXML)
	}
	if containsSubstring(valueAxisBlocks[0], `<c:majorGridlines`) {
		t.Fatalf("expected primary value axis to omit major gridlines in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:majorGridlines`) {
		t.Fatalf("expected secondary value axis to include major gridlines in %s", valueAxisBlocks[1])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:majorTickMark val="cross"/>`) || !containsSubstring(valueAxisBlocks[1], `<c:minorTickMark val="in"/>`) {
		t.Fatalf("expected secondary value axis custom tick marks in %s", valueAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryValueAxisMinorGridlines(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                   "bar",
		"y_axis_major_gridlines": false,
		"y_axis_minor_gridlines": true,
		"categories":             []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 1 {
		t.Fatalf("expected 1 value-axis block in %s", chartXML)
	}
	if containsSubstring(valueAxisBlocks[0], `<c:majorGridlines`) {
		t.Fatalf("expected primary value axis to omit major gridlines in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:minorGridlines`) {
		t.Fatalf("expected primary value axis to include minor gridlines in %s", valueAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryValueAxisMinorGridlines(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                    "combo",
		"y_axis_minor_gridlines":  false,
		"y2_axis_minor_gridlines": true,
		"categories":              []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected 2 value-axis blocks in %s", chartXML)
	}
	if containsSubstring(valueAxisBlocks[0], `<c:minorGridlines`) {
		t.Fatalf("expected primary value axis to omit minor gridlines in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:minorGridlines`) {
		t.Fatalf("expected secondary value axis to include minor gridlines in %s", valueAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryValueAxisCrosses(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":           "bar",
		"y_axis_crosses": "max",
		"categories":     []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 1 {
		t.Fatalf("expected 1 value-axis block in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:crosses val="max"/>`) {
		t.Fatalf("expected primary value axis crosses override in %s", valueAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryValueAxisCrosses(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":            "combo",
		"y_axis_crosses":  "max",
		"y2_axis_crosses": "min",
		"categories":      []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected 2 value-axis blocks in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:crosses val="max"/>`) {
		t.Fatalf("expected primary value axis crosses override in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:crosses val="min"/>`) {
		t.Fatalf("expected secondary value axis crosses override in %s", valueAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryValueAxisCrossBetween(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                 "bar",
		"y_axis_cross_between": "mid_cat",
		"categories":           []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 1 {
		t.Fatalf("expected 1 value-axis block in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:crossBetween val="midCat"/>`) {
		t.Fatalf("expected primary value axis crossBetween override in %s", valueAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryValueAxisCrossBetween(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                  "combo",
		"y2_axis_cross_between": "mid_cat",
		"categories":            []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected 2 value-axis blocks in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:crossBetween val="between"/>`) {
		t.Fatalf("expected primary value axis default crossBetween in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:crossBetween val="midCat"/>`) {
		t.Fatalf("expected secondary value axis crossBetween override in %s", valueAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryValueAxisReverseOrder(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                 "bar",
		"y_axis_reverse_order": true,
		"categories":           []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 1 {
		t.Fatalf("expected 1 value-axis block in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:orientation val="maxMin"/>`) {
		t.Fatalf("expected primary value axis reverse-order scaling in %s", valueAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryValueAxisReverseOrder(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                  "combo",
		"y2_axis_reverse_order": true,
		"categories":            []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected 2 value-axis blocks in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:orientation val="minMax"/>`) {
		t.Fatalf("expected primary value axis default orientation in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:orientation val="maxMin"/>`) {
		t.Fatalf("expected secondary value axis reverse-order scaling in %s", valueAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryCategoryAxisLabelPosition(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                  "bar",
		"x_axis_label_position": "high",
		"categories":            []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:tickLblPos val="high"/>`) {
		t.Fatalf("expected primary category axis label position in %s", categoryAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryCategoryAxisLabelPosition(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                   "combo",
		"x_axis_label_position":  "high",
		"x2_axis_label_position": "low",
		"categories":             []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:tickLblPos val="high"/>`) {
		t.Fatalf("expected primary category axis label position in %s", categoryAxisBlocks[0])
	}
	if !containsSubstring(categoryAxisBlocks[1], `<c:tickLblPos val="low"/>`) {
		t.Fatalf("expected secondary category axis label position in %s", categoryAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryCategoryAxisReverseOrder(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                 "bar",
		"x_axis_reverse_order": true,
		"categories":           []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:orientation val="maxMin"/>`) {
		t.Fatalf("expected primary category axis reverse-order scaling in %s", categoryAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryCategoryAxisReverseOrder(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                  "combo",
		"x2_axis_reverse_order": true,
		"categories":            []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:orientation val="minMax"/>`) {
		t.Fatalf("expected primary category axis default orientation in %s", categoryAxisBlocks[0])
	}
	if !containsSubstring(categoryAxisBlocks[1], `<c:orientation val="maxMin"/>`) {
		t.Fatalf("expected secondary category axis reverse-order scaling in %s", categoryAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryCategoryAxisCrosses(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":           "bar",
		"x_axis_crosses": "max",
		"categories":     []interface{}{"Q1", "Q2"},
		"series":         []interface{}{map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:crosses val="max"/>`) {
		t.Fatalf("expected primary category axis crosses override in %s", categoryAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryCategoryAxisCrosses(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":            "combo",
		"x2_axis_crosses": "min",
		"categories":      []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:crosses val="autoZero"/>`) {
		t.Fatalf("expected primary category axis default crosses in %s", categoryAxisBlocks[0])
	}
	if !containsSubstring(categoryAxisBlocks[1], `<c:crosses val="min"/>`) {
		t.Fatalf("expected secondary category axis crosses override in %s", categoryAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryCategoryAxisTickMarks(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                   "bar",
		"x_axis_major_tick_mark": "cross",
		"x_axis_minor_tick_mark": "in",
		"categories":             []interface{}{"Q1", "Q2"},
		"series":                 []interface{}{map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	for _, needle := range []string{
		`<c:majorTickMark val="cross"/>`,
		`<c:minorTickMark val="in"/>`,
	} {
		if !containsSubstring(categoryAxisBlocks[0], needle) {
			t.Fatalf("expected primary category axis tick-mark override %q in %s", needle, categoryAxisBlocks[0])
		}
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryCategoryAxisTickMarks(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                    "combo",
		"x2_axis_major_tick_mark": "out",
		"x2_axis_minor_tick_mark": "cross",
		"categories":              []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	for _, needle := range []string{
		`<c:majorTickMark val="out"/>`,
		`<c:minorTickMark val="cross"/>`,
	} {
		if !containsSubstring(categoryAxisBlocks[1], needle) {
			t.Fatalf("expected secondary category axis tick-mark override %q in %s", needle, categoryAxisBlocks[1])
		}
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryCategoryAxisLabelLayout(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                   "bar",
		"x_axis_label_alignment": "left",
		"x_axis_label_offset":    250,
		"categories":             []interface{}{"Q1", "Q2"},
		"series":                 []interface{}{map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	for _, needle := range []string{
		`<c:lblAlgn val="l"/>`,
		`<c:lblOffset val="250"/>`,
	} {
		if !containsSubstring(categoryAxisBlocks[0], needle) {
			t.Fatalf("expected primary category axis label-layout override %q in %s", needle, categoryAxisBlocks[0])
		}
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryCategoryAxisLabelLayout(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                    "combo",
		"x2_axis_label_alignment": "right",
		"x2_axis_label_offset":    500,
		"categories":              []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:lblAlgn val="ctr"/>`) || !containsSubstring(categoryAxisBlocks[0], `<c:lblOffset val="100"/>`) {
		t.Fatalf("expected primary category axis defaults in %s", categoryAxisBlocks[0])
	}
	for _, needle := range []string{
		`<c:lblAlgn val="r"/>`,
		`<c:lblOffset val="500"/>`,
	} {
		if !containsSubstring(categoryAxisBlocks[1], needle) {
			t.Fatalf("expected secondary category axis label-layout override %q in %s", needle, categoryAxisBlocks[1])
		}
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryCategoryAxisMultiLevelLabelsToggle(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                      "bar",
		"x_axis_multi_level_labels": false,
		"categories":                []interface{}{"Q1", "Q2"},
		"series":                    []interface{}{map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:noMultiLvlLbl val="1"/>`) {
		t.Fatalf("expected primary category axis to disable multi-level labels in %s", categoryAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryCategoryAxisMultiLevelLabelsToggle(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                       "combo",
		"x_axis_multi_level_labels":  true,
		"x2_axis_multi_level_labels": false,
		"categories":                 []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:noMultiLvlLbl val="0"/>`) {
		t.Fatalf("expected primary category axis to keep multi-level labels enabled in %s", categoryAxisBlocks[0])
	}
	if !containsSubstring(categoryAxisBlocks[1], `<c:noMultiLvlLbl val="1"/>`) {
		t.Fatalf("expected secondary category axis to disable multi-level labels in %s", categoryAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryCategoryAxisVisibility(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":           "bar",
		"x_axis_visible": false,
		"categories":     []interface{}{"Q1", "Q2"},
		"series":         []interface{}{map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:delete val="1"/>`) {
		t.Fatalf("expected primary category axis to be hidden in %s", categoryAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryCategoryAxisVisibility(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":            "combo",
		"x_axis_visible":  true,
		"x2_axis_visible": true,
		"categories":      []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:delete val="0"/>`) {
		t.Fatalf("expected primary category axis to remain visible in %s", categoryAxisBlocks[0])
	}
	if !containsSubstring(categoryAxisBlocks[1], `<c:delete val="0"/>`) {
		t.Fatalf("expected secondary category axis to become visible in %s", categoryAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryCategoryAxisFormat(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":          "bar",
		"x_axis_format": "m/d/yyyy",
		"categories":    []interface{}{"Q1", "Q2"},
		"series":        []interface{}{map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:numFmt formatCode="m/d/yyyy" sourceLinked="0"/>`) {
		t.Fatalf("expected primary category axis format override in %s", categoryAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryCategoryAxisFormat(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":           "combo",
		"x2_axis_format": "[$-409]mmm-yy",
		"categories":     []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:numFmt formatCode="General" sourceLinked="1"/>`) {
		t.Fatalf("expected primary category axis default format in %s", categoryAxisBlocks[0])
	}
	if !containsSubstring(categoryAxisBlocks[1], `<c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/>`) {
		t.Fatalf("expected secondary category axis format override in %s", categoryAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryCategoryAxisAuto(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "bar",
		"x_axis_auto": false,
		"categories":  []interface{}{"Q1", "Q2"},
		"series":      []interface{}{map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}}},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:auto val="0"/>`) {
		t.Fatalf("expected primary category axis auto override in %s", categoryAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryCategoryAxisAuto(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":         "combo",
		"x_axis_auto":  true,
		"x2_axis_auto": false,
		"categories":   []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:auto val="1"/>`) {
		t.Fatalf("expected primary category axis default auto behavior in %s", categoryAxisBlocks[0])
	}
	if !containsSubstring(categoryAxisBlocks[1], `<c:auto val="0"/>`) {
		t.Fatalf("expected secondary category axis auto override in %s", categoryAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPrimaryValueAxisLabelPosition(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                  "bar",
		"y_axis_label_position": "high",
		"categories":            []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 1 {
		t.Fatalf("expected 1 value-axis block in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:tickLblPos val="high"/>`) {
		t.Fatalf("expected primary value axis label position in %s", valueAxisBlocks[0])
	}
}

func TestOfficePPTXChartXMLIncludesSecondaryValueAxisLabelPosition(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":                   "combo",
		"y_axis_label_position":  "high",
		"y2_axis_label_position": "low",
		"categories":             []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected 2 value-axis blocks in %s", chartXML)
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:tickLblPos val="high"/>`) {
		t.Fatalf("expected primary value axis label position in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:tickLblPos val="low"/>`) {
		t.Fatalf("expected secondary value axis label position in %s", valueAxisBlocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesSeriesColors(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "color": "#D97706", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "color": "2563EB", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:v>Revenue</c:v>`,
		`<c:v>Margin</c:v>`,
		`<a:srgbClr val="D97706"/>`,
		`<a:srgbClr val="2563EB"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("series-colored chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLForThemeAppliesDefaultSeriesPalette(t *testing.T) {
	chartXML := officePPTXChartXMLForTheme(officeChartSpec{
		Type:       "bar",
		Categories: []string{"Q1", "Q2"},
		Series: []officeChartSeries{
			{Name: "Revenue", Values: []float64{120, 132}},
			{Name: "Margin", Values: []float64{28, 31}},
			{Name: "Pipeline", Values: []float64{80, 96}},
		},
	}, resolveOfficeTheme("midnight", ""))

	for _, needle := range []string{
		`<c:v>Revenue</c:v>`,
		`<c:v>Margin</c:v>`,
		`<c:v>Pipeline</c:v>`,
		`<a:srgbClr val="242C38"/>`,
		`<a:srgbClr val="4D7CFE"/>`,
		`<a:srgbClr val="8A93A2"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("theme-colored chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLForThemeAppliesDefaultPiePointPalette(t *testing.T) {
	chartXML := officePPTXChartXMLForTheme(officeChartSpec{
		Type:       "pie",
		Categories: []string{"North", "South", "West"},
		Series: []officeChartSeries{
			{Name: "Revenue", Values: []float64{120, 98, 110}},
		},
	}, resolveOfficeTheme("midnight", ""))

	for _, needle := range []string{
		`<c:dPt><c:idx val="0"/><c:spPr><a:solidFill><a:srgbClr val="242C38"/></a:solidFill>`,
		`<c:dPt><c:idx val="1"/><c:spPr><a:solidFill><a:srgbClr val="4D7CFE"/></a:solidFill>`,
		`<c:dPt><c:idx val="2"/><c:spPr><a:solidFill><a:srgbClr val="8A93A2"/></a:solidFill>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("theme pie chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLForThemeUsesThemeTypography(t *testing.T) {
	chartXML := officePPTXChartXMLForTheme(officeChartSpec{
		Type:              "bar",
		Title:             "Board Metrics",
		CategoryAxisTitle: "Quarter",
		ValueAxisTitle:    "Revenue ($M)",
		Categories:        []string{"Q1", "Q2"},
		Series: []officeChartSeries{
			{Name: "Revenue", Values: []float64{120, 132}},
			{Name: "Margin", Values: []float64{28, 31}},
		},
	}, resolveOfficeTheme("midnight", ""))

	for _, needle := range []string{
		`<a:latin typeface="Helvetica Neue"/>`,
		`<a:latin typeface="Helvetica"/>`,
		`<a:ea typeface="Hiragino Sans GB"/>`,
		`<a:srgbClr val="F5F3EE"/>`,
		`<a:srgbClr val="D8DCE5"/>`,
		`<c:legend><c:legendPos val="r"/><c:layout/><c:txPr>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("theme-typography chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLForThemeUsesThemeAxisChrome(t *testing.T) {
	chartXML := officePPTXChartXMLForTheme(officeChartSpec{
		Type:       "bar",
		Categories: []string{"Q1", "Q2"},
		Series: []officeChartSeries{
			{Name: "Revenue", Values: []float64{120, 132}},
			{Name: "Margin", Values: []float64{28, 31}},
		},
	}, resolveOfficeTheme("midnight", ""))

	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 1 {
		t.Fatalf("expected 1 category-axis block in %s", chartXML)
	}
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 1 {
		t.Fatalf("expected 1 value-axis block in %s", chartXML)
	}

	for _, needle := range []string{
		`<c:txPr>`,
		`<a:latin typeface="Helvetica"/>`,
		`<a:ea typeface="Hiragino Sans GB"/>`,
		`<a:srgbClr val="334155"/>`,
		`<c:spPr><a:ln w="12700"><a:solidFill><a:srgbClr val="CBD5E1"/></a:solidFill></a:ln></c:spPr>`,
	} {
		if !containsSubstring(categoryAxisBlocks[0], needle) {
			t.Fatalf("theme category-axis XML missing %q in %s", needle, categoryAxisBlocks[0])
		}
	}

	for _, needle := range []string{
		`<c:majorGridlines><c:spPr><a:ln w="12700"><a:solidFill><a:srgbClr val="F1F5F9"/></a:solidFill></a:ln></c:spPr></c:majorGridlines>`,
		`<c:txPr>`,
		`<a:latin typeface="Helvetica"/>`,
		`<a:ea typeface="Hiragino Sans GB"/>`,
		`<a:srgbClr val="334155"/>`,
		`<c:spPr><a:ln w="12700"><a:solidFill><a:srgbClr val="CBD5E1"/></a:solidFill></a:ln></c:spPr>`,
	} {
		if !containsSubstring(valueAxisBlocks[0], needle) {
			t.Fatalf("theme value-axis XML missing %q in %s", needle, valueAxisBlocks[0])
		}
	}
}

func TestOfficePPTXChartXMLIncludesSeriesLineStyling(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "line",
		"categories": []interface{}{"Jan", "Feb", "Mar"},
		"series": []interface{}{
			map[string]interface{}{
				"name":       "Trend",
				"color":      "#2563EB",
				"line_width": 3,
				"dash":       "dash",
				"marker":     "diamond",
				"values":     []interface{}{10, 12, 15},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:lineChart>`,
		`<c:v>Trend</c:v>`,
		`<c:marker><c:symbol val="diamond"/></c:marker>`,
		`<a:ln w="38100">`,
		`<a:prstDash val="dash"/>`,
		`<a:srgbClr val="2563EB"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("series-styled chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesLineSmoothing(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "line",
		"smooth":     true,
		"categories": []interface{}{"Jan", "Feb", "Mar"},
		"series": []interface{}{
			map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12, 15}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if !containsSubstring(chartXML, `<c:smooth val="1"/>`) {
		t.Fatalf("line smoothing chart XML missing smooth override in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesComboSeriesSmoothingOverride(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "smooth": true, "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	if containsSubstring(blocks[0], `<c:smooth val="1"/>`) {
		t.Fatalf("expected bar block to omit line smoothing in %s", blocks[0])
	}
	if !containsSubstring(blocks[1], `<c:smooth val="1"/>`) {
		t.Fatalf("expected line block to include smoothing override in %s", blocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesPiePointColors(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "pie",
		"categories": []interface{}{"Won", "Lost", "Open"},
		"series": []interface{}{
			map[string]interface{}{
				"name":         "Pipeline",
				"slice_colors": []interface{}{"#22C55E", "EF4444", "F59E0B"},
				"values":       []interface{}{55, 25, 20},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:pieChart>`,
		`<c:dPt><c:idx val="0"/><c:spPr><a:solidFill><a:srgbClr val="22C55E"/></a:solidFill>`,
		`<c:dPt><c:idx val="1"/><c:spPr><a:solidFill><a:srgbClr val="EF4444"/></a:solidFill>`,
		`<c:dPt><c:idx val="2"/><c:spPr><a:solidFill><a:srgbClr val="F59E0B"/></a:solidFill>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("point-colored pie chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboSeriesPointColors(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{
				"name":         "Margin",
				"type":         "line",
				"point_colors": []interface{}{"#2563EB", "10B981", "F59E0B"},
				"values":       []interface{}{28, 31, 34},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	if containsSubstring(blocks[0], `<c:dPt>`) {
		t.Fatalf("expected bar block to omit point colors in %s", blocks[0])
	}
	for _, needle := range []string{
		`<c:dPt><c:idx val="0"/><c:spPr><a:solidFill><a:srgbClr val="2563EB"/></a:solidFill>`,
		`<c:dPt><c:idx val="1"/><c:spPr><a:solidFill><a:srgbClr val="10B981"/></a:solidFill>`,
		`<c:dPt><c:idx val="2"/><c:spPr><a:solidFill><a:srgbClr val="F59E0B"/></a:solidFill>`,
	} {
		if !containsSubstring(blocks[1], needle) {
			t.Fatalf("expected line block to include %q in %s", needle, blocks[1])
		}
	}
}

func TestOfficePPTXChartXMLIncludesDonutSliceExplosions(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "donut",
		"categories": []interface{}{"Adoption", "Pending", "Blocked"},
		"series": []interface{}{
			map[string]interface{}{
				"name":             "Status",
				"slice_explosions": []interface{}{18, 0, 32},
				"values":           []interface{}{70, 20, 10},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:doughnutChart>`,
		`<c:dPt><c:idx val="0"/><c:explosion val="18"/>`,
		`<c:dPt><c:idx val="2"/><c:explosion val="32"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("slice-exploded donut chart XML missing %q in %s", needle, chartXML)
		}
	}
	if containsSubstring(chartXML, `<c:dPt><c:idx val="1"/><c:explosion val="0"/>`) {
		t.Fatalf("expected zero explosion slices to be omitted in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesLegendPositionOverride(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":            "bar",
		"show_legend":     true,
		"legend_position": "top",
		"categories":      []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
			map[string]interface{}{"name": "Target", "values": []interface{}{140, 145}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:legend><c:legendPos val="t"/><c:layout/>`,
		`<c:txPr>`,
		`<a:defRPr lang="en-US" sz="1100">`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("legend-position chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLOmitsLegendWhenDisabled(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "pie",
		"show_legend": false,
		"categories":  []interface{}{"Won", "Lost", "Open"},
		"series": []interface{}{
			map[string]interface{}{"name": "Pipeline", "values": []interface{}{55, 25, 20}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if containsSubstring(chartXML, `<c:legend>`) {
		t.Fatalf("expected chart XML to omit legend when disabled: %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesCircularPlotControls(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "donut",
		"vary_colors": false,
		"start_angle": 120,
		"hole_size":   64,
		"categories":  []interface{}{"Adoption", "Pending", "Blocked"},
		"series": []interface{}{
			map[string]interface{}{"name": "Status", "values": []interface{}{70, 20, 10}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:doughnutChart>`,
		`<c:varyColors val="0"/>`,
		`<c:firstSliceAng val="120"/>`,
		`<c:holeSize val="64"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("circular-plot chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesBarVaryColorsOverride(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "bar",
		"vary_colors": true,
		"categories":  []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
			map[string]interface{}{"name": "Target", "values": []interface{}{140, 145}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	if !containsSubstring(chartXML, `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/><c:varyColors val="1"/>`) {
		t.Fatalf("bar chart XML missing varyColors override in %s", chartXML)
	}
}

func TestOfficePPTXChartXMLIncludesComboVaryColorsOverride(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":        "combo",
		"vary_colors": true,
		"categories":  []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	for _, block := range blocks {
		if !containsSubstring(block, `<c:varyColors val="1"/>`) {
			t.Fatalf("expected combo block to include varyColors override in %s", block)
		}
	}
}

func TestOfficePPTXChartXMLIncludesBarLayoutControls(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "bar",
		"gap_width":  60,
		"overlap":    -20,
		"categories": []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
			map[string]interface{}{"name": "Target", "values": []interface{}{140, 145}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:barChart>`,
		`<c:gapWidth val="60"/>`,
		`<c:overlap val="-20"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("bar-layout chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboBarLayoutControls(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"gap_width":  72,
		"overlap":    18,
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	if !containsSubstring(blocks[0], `<c:gapWidth val="72"/>`) || !containsSubstring(blocks[0], `<c:overlap val="18"/>`) {
		t.Fatalf("expected bar block to include layout controls in %s", blocks[0])
	}
	if containsSubstring(blocks[1], `<c:gapWidth`) || containsSubstring(blocks[1], `<c:overlap`) {
		t.Fatalf("expected line block to omit bar layout controls in %s", blocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesChartDataLabels(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "bar",
		"labels":     true,
		"categories": []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:barChart>`,
		`<c:dLbls>`,
		`<c:showVal val="1"/>`,
		`<c:showCatName val="0"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("data-labeled chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboSeriesDataLabels(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{"name": "Margin", "type": "line", "labels": true, "values": []interface{}{28, 31, 34}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	if containsSubstring(blocks[0], `<c:dLbls>`) {
		t.Fatalf("expected bar block to omit data labels in %s", blocks[0])
	}
	if !containsSubstring(blocks[1], `<c:dLbls>`) || !containsSubstring(blocks[1], `<c:showVal val="1"/>`) {
		t.Fatalf("expected line block to include data labels in %s", blocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesLabelPositionAndFormat(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":           "bar",
		"labels":         true,
		"label_position": "outside_end",
		"label_format":   "$#,##0",
		"categories":     []interface{}{"Q1", "Q2"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:dLblPos val="outEnd"/>`,
		`<c:numFmt formatCode="$#,##0" sourceLinked="0"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("formatted label chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboSeriesLabelPositionAndFormat(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{
				"name":           "Margin",
				"type":           "line",
				"labels":         true,
				"label_position": "above",
				"label_format":   "0.0",
				"values":         []interface{}{28, 31, 34},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	if containsSubstring(blocks[0], `<c:dLbls>`) {
		t.Fatalf("expected bar block to omit data labels in %s", blocks[0])
	}
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:dLblPos val="t"/>`,
		`<c:numFmt formatCode="0.0" sourceLinked="0"/>`,
	} {
		if !containsSubstring(blocks[1], needle) {
			t.Fatalf("expected line block to include %q in %s", needle, blocks[1])
		}
	}
}

func TestOfficePPTXChartXMLIncludesPieLabelContentControls(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":          "pie",
		"labels":        true,
		"show_value":    false,
		"show_category": false,
		"show_percent":  true,
		"categories":    []interface{}{"Won", "Lost"},
		"series": []interface{}{
			map[string]interface{}{"name": "Pipeline", "values": []interface{}{55, 45}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:showVal val="0"/>`,
		`<c:showCatName val="0"/>`,
		`<c:showPercent val="1"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("label-content chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboSeriesLabelContentControls(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{
				"name":             "Margin",
				"type":             "line",
				"labels":           true,
				"show_value":       false,
				"show_series_name": true,
				"values":           []interface{}{28, 31, 34},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	if containsSubstring(blocks[0], `<c:dLbls>`) {
		t.Fatalf("expected bar block to omit data labels in %s", blocks[0])
	}
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:showVal val="0"/>`,
		`<c:showSerName val="1"/>`,
		`<c:showPercent val="0"/>`,
	} {
		if !containsSubstring(blocks[1], needle) {
			t.Fatalf("expected line block to include %q in %s", needle, blocks[1])
		}
	}
}

func TestOfficePPTXChartXMLIncludesPieLegendKeyAndBubbleSizeControls(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":             "pie",
		"labels":           true,
		"show_legend_key":  true,
		"show_bubble_size": true,
		"categories":       []interface{}{"Won", "Lost"},
		"series": []interface{}{
			map[string]interface{}{"name": "Pipeline", "values": []interface{}{55, 45}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:showLegendKey val="1"/>`,
		`<c:showBubbleSize val="1"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("legend-key/bubble-size chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboSeriesLegendKeyAndBubbleSizeControls(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{
				"name":             "Margin",
				"type":             "line",
				"labels":           true,
				"show_legend_key":  true,
				"show_bubble_size": true,
				"values":           []interface{}{28, 31, 34},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	if containsSubstring(blocks[0], `<c:dLbls>`) {
		t.Fatalf("expected bar block to omit data labels in %s", blocks[0])
	}
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:showLegendKey val="1"/>`,
		`<c:showBubbleSize val="1"/>`,
	} {
		if !containsSubstring(blocks[1], needle) {
			t.Fatalf("expected line block to include %q in %s", needle, blocks[1])
		}
	}
}

func TestOfficePPTXChartXMLIncludesComboSeriesLabelSeparator(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "combo",
		"categories": []interface{}{"Q1", "Q2", "Q3"},
		"series": []interface{}{
			map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
			map[string]interface{}{
				"name":            "Margin",
				"type":            "line",
				"labels":          true,
				"label_separator": " | ",
				"values":          []interface{}{28, 31, 34},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	blockPattern := regexp.MustCompile(`<c:(barChart|lineChart)>.*?</c:(barChart|lineChart)>`)
	blocks := blockPattern.FindAllString(chartXML, -1)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 chart blocks in %s", chartXML)
	}
	if containsSubstring(blocks[0], `<c:separator>`) {
		t.Fatalf("expected bar block to omit label separator in %s", blocks[0])
	}
	if !containsSubstring(blocks[1], `<c:separator>|</c:separator>`) {
		t.Fatalf("expected line block to include label separator in %s", blocks[1])
	}
}

func TestOfficePPTXChartXMLIncludesDonutLeaderLineControl(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":              "donut",
		"labels":            true,
		"show_leader_lines": true,
		"categories":        []interface{}{"Won", "Lost"},
		"series": []interface{}{
			map[string]interface{}{"name": "Pipeline", "values": []interface{}{55, 45}},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:showLeaderLines val="1"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("donut leader-line chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesDonutSliceLabelVisibilityOverrides(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "donut",
		"labels":     true,
		"categories": []interface{}{"Won", "Lost", "Open"},
		"series": []interface{}{
			map[string]interface{}{
				"name":              "Pipeline",
				"slice_show_labels": []interface{}{true, false, true},
				"values":            []interface{}{55, 25, 20},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:dLbl><c:idx val="0"/><c:delete val="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:delete val="1"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:delete val="0"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("donut slice-label visibility chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesDonutSliceLabelPositionOverrides(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "donut",
		"labels":     true,
		"categories": []interface{}{"Won", "Lost", "Open"},
		"series": []interface{}{
			map[string]interface{}{
				"name":                  "Pipeline",
				"slice_label_positions": []interface{}{"outside_end", "center", "best_fit"},
				"values":                []interface{}{55, 25, 20},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:dLbl><c:idx val="0"/><c:dLblPos val="outEnd"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:dLblPos val="ctr"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:dLblPos val="bestFit"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("donut slice-label position chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesDonutSliceLabelFormatOverrides(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "donut",
		"labels":     true,
		"categories": []interface{}{"Won", "Lost", "Open"},
		"series": []interface{}{
			map[string]interface{}{
				"name":                "Pipeline",
				"slice_label_formats": []interface{}{"0.0%", "$#,##0", "0.0"},
				"values":              []interface{}{55, 25, 20},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:dLbl><c:idx val="0"/><c:numFmt formatCode="0.0%" sourceLinked="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:numFmt formatCode="$#,##0" sourceLinked="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:numFmt formatCode="0.0" sourceLinked="0"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("donut slice-label format chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesDonutSliceLabelSeparatorOverrides(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "donut",
		"labels":     true,
		"categories": []interface{}{"Won", "Lost", "Open"},
		"series": []interface{}{
			map[string]interface{}{
				"name":                   "Pipeline",
				"slice_label_separators": []interface{}{" / ", " | ", " - "},
				"values":                 []interface{}{55, 25, 20},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:dLbl><c:idx val="0"/><c:separator>/</c:separator></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:separator>|</c:separator></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:separator>-</c:separator></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("donut slice-label separator chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesDonutSliceLabelContentOverrides(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "donut",
		"labels":     true,
		"categories": []interface{}{"Won", "Lost", "Open"},
		"series": []interface{}{
			map[string]interface{}{
				"name":                  "Pipeline",
				"slice_show_values":     []interface{}{true, false, true},
				"slice_show_categories": []interface{}{false, true, false},
				"values":                []interface{}{55, 25, 20},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:dLbl><c:idx val="0"/><c:showVal val="1"/><c:showCatName val="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:showVal val="0"/><c:showCatName val="1"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:showVal val="1"/><c:showCatName val="0"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("donut slice-label content chart XML missing %q in %s", needle, chartXML)
		}
	}
}

func TestOfficePPTXChartXMLIncludesDonutSlicePercentAndSeriesNameOverrides(t *testing.T) {
	chart, err := parseOfficeChart(map[string]interface{}{
		"type":       "donut",
		"labels":     true,
		"categories": []interface{}{"Won", "Lost", "Open"},
		"series": []interface{}{
			map[string]interface{}{
				"name":                    "Pipeline",
				"slice_show_percents":     []interface{}{true, false, true},
				"slice_show_series_names": []interface{}{false, true, false},
				"values":                  []interface{}{55, 25, 20},
			},
		},
	})
	if err != nil {
		t.Fatalf("parseOfficeChart() error = %v", err)
	}

	chartXML := officePPTXChartXML(*chart)
	for _, needle := range []string{
		`<c:dLbls>`,
		`<c:dLbl><c:idx val="0"/><c:showSerName val="0"/><c:showPercent val="1"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:showSerName val="1"/><c:showPercent val="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:showSerName val="0"/><c:showPercent val="1"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("donut slice percent/series-name chart XML missing %q in %s", needle, chartXML)
		}
	}
}
