package tools

import "testing"

func TestOfficePPTXSlideXMLIncludesNativeTable(t *testing.T) {
	slide := officePPTXSlide{
		Title: "Metrics",
		Lines: []string{"Performance snapshot"},
		Table: &officeTableSpec{
			Headers: []string{"Region", "Revenue", "Status"},
			Rows: [][]string{
				{"North", "120", "Good"},
				{"South", "98", "Watch"},
			},
		},
	}

	xml := officePPTXSlideXML(slide)
	for _, needle := range []string{
		`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">`,
		`<a:tbl>`,
		`<a:tblPr firstRow="1" bandRow="1">`,
		`<a:gridCol `,
		`<a:tr h="`,
		`<a:t>Region</a:t>`,
		`<a:t>North</a:t>`,
		`<a:t>Performance snapshot</a:t>`,
	} {
		if !containsSubstring(xml, needle) {
			t.Fatalf("slide XML missing %q in %s", needle, xml)
		}
	}
	if containsSubstring(xml, `<a:t>Region | Revenue | Status</a:t>`) {
		t.Fatalf("slide XML should not flatten table headers into one text run: %s", xml)
	}
}
