package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

func buildTextPDF(text string) []byte {
	stream := fmt.Sprintf("BT\n/F1 24 Tf\n72 96 Td\n(%s) Tj\nET\n", escapePDFText(text))
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 144] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, obj := range objects {
		offsets[index+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", index+1, obj)
	}
	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefStart)
	return buf.Bytes()
}

func buildTextFieldPDF(name, alternateName, value string) []byte {
	stream := strings.Join([]string{
		"BT",
		"/F1 18 Tf",
		"48 112 Td",
		fmt.Sprintf("(Name:) Tj"),
		"ET",
	}, "\n") + "\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /AcroForm 6 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 160] /Annots [4 0 R] /Contents 7 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Type /Annot /Subtype /Widget /FT /Tx /T (%s) /TU (%s) /Rect [96 84 252 108] /V (%s) /F 4 /Ff 0 /DA (/F1 12 Tf 0 g) /P 3 0 R >>", escapePDFText(name), escapePDFText(alternateName), escapePDFText(value)),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Fields [4 0 R] /NeedAppearances true /DA (/F1 12 Tf 0 g) /DR << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, obj := range objects {
		offsets[index+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", index+1, obj)
	}
	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefStart)
	return buf.Bytes()
}

func buildComboFieldPDF(name, alternateName, value string, options []string) []byte {
	optParts := make([]string, 0, len(options))
	for _, option := range options {
		optParts = append(optParts, fmt.Sprintf("(%s)", escapePDFText(option)))
	}
	stream := strings.Join([]string{
		"BT",
		"/F1 18 Tf",
		"48 112 Td",
		fmt.Sprintf("(Choice:) Tj"),
		"ET",
	}, "\n") + "\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /AcroForm 6 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 160] /Annots [4 0 R] /Contents 7 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Type /Annot /Subtype /Widget /FT /Ch /T (%s) /TU (%s) /Rect [96 84 252 108] /V (%s) /Opt [%s] /F 4 /Ff 131072 /DA (/F1 12 Tf 0 g) /P 3 0 R >>", escapePDFText(name), escapePDFText(alternateName), escapePDFText(value), strings.Join(optParts, " ")),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Fields [4 0 R] /NeedAppearances true /DA (/F1 12 Tf 0 g) /DR << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, obj := range objects {
		offsets[index+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", index+1, obj)
	}
	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefStart)
	return buf.Bytes()
}

func buildCheckboxFieldPDF(name, alternateName string, checked bool) []byte {
	state := "/Off"
	if checked {
		state = "/Yes"
	}
	offAppearance := "<< /Type /XObject /Subtype /Form /BBox [0 0 16 16] /Length 0 >>\nstream\n\nendstream"
	onStream := "q\nBT\n/ZaDb 14 Tf\n2 2 Td\n(4) Tj\nET\nQ\n"
	onAppearance := fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 16 16] /Resources << /Font << /ZaDb 5 0 R >> >> /Length %d >>\nstream\n%sendstream", len(onStream), onStream)
	stream := strings.Join([]string{
		"BT",
		"/F1 18 Tf",
		"48 112 Td",
		fmt.Sprintf("(Check:) Tj"),
		"ET",
	}, "\n") + "\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /AcroForm 6 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 160] /Annots [4 0 R] /Contents 7 0 R /Resources << /Font << /F1 8 0 R /ZaDb 5 0 R >> >> >>",
		fmt.Sprintf("<< /Type /Annot /Subtype /Widget /FT /Btn /T (%s) /TU (%s) /Rect [96 84 112 100] /V %s /AS %s /AP << /N << /Off 9 0 R /Yes 10 0 R >> >> /F 4 /Ff 0 /MK << /CA (4) >> /DA (/ZaDb 12 Tf 0 g) /P 3 0 R >>", escapePDFText(name), escapePDFText(alternateName), state, state),
		"<< /Type /Font /Subtype /Type1 /BaseFont /ZapfDingbats >>",
		"<< /Fields [4 0 R] /NeedAppearances true /DA (/ZaDb 12 Tf 0 g) /DR << /Font << /ZaDb 5 0 R /F1 8 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		offAppearance,
		onAppearance,
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, obj := range objects {
		offsets[index+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", index+1, obj)
	}
	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefStart)
	return buf.Bytes()
}

func buildListFieldPDF(name, alternateName, value string, options []string) []byte {
	optParts := make([]string, 0, len(options))
	for _, option := range options {
		optParts = append(optParts, fmt.Sprintf("(%s)", escapePDFText(option)))
	}
	stream := strings.Join([]string{
		"BT",
		"/F1 18 Tf",
		"48 112 Td",
		fmt.Sprintf("(List:) Tj"),
		"ET",
	}, "\n") + "\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /AcroForm 6 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 180] /Annots [4 0 R] /Contents 7 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Type /Annot /Subtype /Widget /FT /Ch /T (%s) /TU (%s) /Rect [96 64 252 108] /V (%s) /Opt [%s] /I [1] /F 4 /Ff 0 /DA (/F1 12 Tf 0 g) /P 3 0 R >>", escapePDFText(name), escapePDFText(alternateName), escapePDFText(value), strings.Join(optParts, " ")),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Fields [4 0 R] /NeedAppearances true /DA (/F1 12 Tf 0 g) /DR << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, obj := range objects {
		offsets[index+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", index+1, obj)
	}
	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefStart)
	return buf.Bytes()
}

func buildRadioFieldPDF(name, alternateName, selectedExport string, options []string) []byte {
	if len(options) < 2 {
		panic("radio field requires at least two options")
	}

	stream := strings.Join([]string{
		"BT",
		"/F1 18 Tf",
		"48 132 Td",
		fmt.Sprintf("(Radio:) Tj"),
		"ET",
	}, "\n") + "\n"

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R /AcroForm 4 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 360 200] /Annots [6 0 R 7 0 R] /Contents 8 0 R /Resources << /Font << /F1 9 0 R /ZaDb 10 0 R >> >> >>",
		fmt.Sprintf("<< /Fields [5 0 R] /NeedAppearances true /DA (/ZaDb 12 Tf 0 g) /DR << /Font << /ZaDb 10 0 R /F1 9 0 R >> >> >>"),
		fmt.Sprintf("<< /FT /Btn /T (%s) /TU (%s) /Ff 32768 /V /%s /Kids [6 0 R 7 0 R] >>", escapePDFText(name), escapePDFText(alternateName), escapePDFName(options[0])),
		"", // widget 1 placeholder
		"", // widget 2 placeholder
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /ZapfDingbats >>",
		"<< /Type /XObject /Subtype /Form /BBox [0 0 16 16] /Length 0 >>\nstream\n\nendstream",
		"<< /Type /XObject /Subtype /Form /BBox [0 0 16 16] /Length 0 >>\nstream\n\nendstream",
		"<< /Type /XObject /Subtype /Form /BBox [0 0 16 16] /Length 0 >>\nstream\n\nendstream",
		"<< /Type /XObject /Subtype /Form /BBox [0 0 16 16] /Length 0 >>\nstream\n\nendstream",
	}

	r1State := "/Off"
	r2State := "/Off"
	parentValue := escapePDFName(selectedExport)
	if strings.EqualFold(selectedExport, options[0]) {
		r1State = "/" + escapePDFName(options[0])
		parentValue = escapePDFName(options[0])
	} else if strings.EqualFold(selectedExport, options[1]) {
		r2State = "/" + escapePDFName(options[1])
		parentValue = escapePDFName(options[1])
	}
	objects[4] = fmt.Sprintf("<< /FT /Btn /T (%s) /TU (%s) /Ff 32768 /V /%s /Kids [6 0 R 7 0 R] >>", escapePDFText(name), escapePDFText(alternateName), parentValue)
	objects[5] = fmt.Sprintf("<< /Type /Annot /Subtype /Widget /Parent 5 0 R /Rect [96 108 112 124] /AP << /N << /Off 11 0 R /%s 12 0 R >> >> /AS %s /F 4 /P 3 0 R >>", escapePDFName(options[0]), r1State)
	objects[6] = fmt.Sprintf("<< /Type /Annot /Subtype /Widget /Parent 5 0 R /Rect [96 76 112 92] /AP << /N << /Off 13 0 R /%s 14 0 R >> >> /AS %s /F 4 /P 3 0 R >>", escapePDFName(options[1]), r2State)

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, obj := range objects {
		offsets[index+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", index+1, obj)
	}
	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefStart)
	return buf.Bytes()
}

func escapePDFText(text string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)")
	return replacer.Replace(text)
}

func escapePDFName(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, " ", "_")
	text = strings.ReplaceAll(text, "/", "_")
	text = strings.ReplaceAll(text, "#", "_")
	if text == "" {
		return "Option"
	}
	return text
}
