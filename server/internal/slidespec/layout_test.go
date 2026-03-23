package slidespec

import "testing"

func TestBuildLayoutSpecUsesAgendaTemplateElements(t *testing.T) {
	spec := BuildLayoutSpec(Brief{
		RenderMode:  RenderModeSlide,
		TemplateID:  TemplateAgenda,
		StylePreset: StylePresetBananaSlides,
		Title:       "Q3 rollout agenda",
		Bullets:     []string{"Foundation", "Pilot launch", "Scale-up"},
		AspectRatio: "16:9",
	}, 1280, 720, false)

	if spec.TemplateID != TemplateAgenda {
		t.Fatalf("template_id = %q, want %q", spec.TemplateID, TemplateAgenda)
	}
	stepCards := 0
	displayIndices := 0
	dividers := 0
	imageElements := 0
	for _, element := range spec.Elements {
		if element.Name == "step-card" {
			stepCards++
		}
		if element.Name == "step-index" && element.FontFamily == "display" {
			displayIndices++
		}
		if element.Name == "agenda-divider" && element.Kind == "chart" && element.ChartType == "divider" {
			dividers++
		}
		if element.Kind == "image" {
			imageElements++
		}
	}
	if stepCards != 3 {
		t.Fatalf("step card count = %d, want 3", stepCards)
	}
	if displayIndices != 3 {
		t.Fatalf("display index count = %d, want 3", displayIndices)
	}
	if dividers != 1 {
		t.Fatalf("divider count = %d, want 1", dividers)
	}
	if imageElements != 0 {
		t.Fatalf("image element count = %d, want 0", imageElements)
	}
}

func TestBuildLayoutSpecUsesMetricsTemplateElements(t *testing.T) {
	spec := BuildLayoutSpec(Brief{
		RenderMode:  RenderModeSlide,
		TemplateID:  TemplateMetrics,
		StylePreset: StylePresetNanoSlides,
		Title:       "Growth metrics",
		Bullets:     []string{"Conversion +18%", "CAC -22%", "ROI 3.4x"},
		AspectRatio: "16:9",
	}, 1280, 720, false)

	if spec.TemplateID != TemplateMetrics {
		t.Fatalf("template_id = %q, want %q", spec.TemplateID, TemplateMetrics)
	}
	metricValues := 0
	monoMetrics := 0
	trendCharts := 0
	foundROI := false
	foundPercentTrend := false
	foundMultiplierTrend := false
	for _, element := range spec.Elements {
		if element.Name == "metric-value" {
			metricValues++
			if element.FontFamily == "mono" {
				monoMetrics++
			}
			if element.Text == "3.4x" {
				foundROI = true
			}
		}
		if element.Name == "metric-trend" && element.Kind == "chart" && element.ChartType == "sparkline" && len(element.Values) == 5 {
			trendCharts++
			if element.ValueFormat == "percent" {
				foundPercentTrend = true
			}
			if element.ValueFormat == "multiplier" {
				foundMultiplierTrend = true
			}
		}
	}
	if metricValues != 3 {
		t.Fatalf("metric value count = %d, want 3", metricValues)
	}
	if monoMetrics != 3 {
		t.Fatalf("mono metric count = %d, want 3", monoMetrics)
	}
	if trendCharts != 3 {
		t.Fatalf("trend chart count = %d, want 3", trendCharts)
	}
	if !foundROI {
		t.Fatalf("metric values = %#v, want parsed ROI value", spec.Elements)
	}
	if !foundPercentTrend {
		t.Fatalf("metric trends = %#v, want percent value format on percentage metrics", spec.Elements)
	}
	if !foundMultiplierTrend {
		t.Fatalf("metric trends = %#v, want multiplier value format on ROI metric", spec.Elements)
	}
}
