package tools

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unsafe"
)

type officeZipEntry struct {
	Name     string
	Content  string
	SizeHint int
	WriteTo  func(io.Writer) error
}

func officeBuildZip(entries []officeZipEntry) ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(estimateOfficeZipBuffer(entries))
	zw := newFastZipWriter(&buf)
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		w, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("create office zip entry %s: %w", name, err)
		}
		if entry.WriteTo != nil {
			if err := entry.WriteTo(w); err != nil {
				_ = zw.Close()
				return nil, fmt.Errorf("write office zip entry %s: %w", name, err)
			}
			continue
		}
		if _, err := officeWriteStringNoCopy(w, entry.Content); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("write office zip entry %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close office zip: %w", err)
	}
	return buf.Bytes(), nil
}

func estimateOfficeZipBuffer(entries []officeZipEntry) int {
	total := len(entries) * 256
	contentBytes := 0
	for _, entry := range entries {
		switch {
		case entry.SizeHint > 0:
			contentBytes += entry.SizeHint
		default:
			contentBytes += len(entry.Content)
		}
	}
	if contentBytes <= 0 {
		return total
	}
	if contentBytes <= 8<<20 {
		return total + contentBytes
	}
	return total + (contentBytes / 2)
}

func officeWriteStringNoCopy(w io.Writer, content string) (int, error) {
	if content == "" {
		return 0, nil
	}
	// The zip writer consumes the bytes during Write and does not retain or mutate them,
	// so a read-only slice view avoids an otherwise large string->[]byte allocation.
	return w.Write(unsafe.Slice(unsafe.StringData(content), len(content)))
}

func officePackageRelsXML(mainTarget string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="%s"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`, officeXMLText(strings.TrimSpace(mainTarget)))
}

func officeCorePropsXML(title, subject string) string {
	now := officeNowISO()
	title = strings.TrimSpace(title)
	subject = strings.TrimSpace(subject)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:title>%s</dc:title>
  <dc:subject>%s</dc:subject>
  <dc:creator>ZimaOS Blue</dc:creator>
  <cp:lastModifiedBy>ZimaOS Blue</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>
</cp:coreProperties>`, officeXMLText(title), officeXMLText(subject), now, now)
}
