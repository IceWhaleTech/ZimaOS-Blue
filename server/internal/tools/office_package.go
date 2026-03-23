package tools

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

type officeZipEntry struct {
	Name    string
	Content string
}

func officeBuildZip(entries []officeZipEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
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
		if _, err := w.Write([]byte(entry.Content)); err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("write office zip entry %s: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close office zip: %w", err)
	}
	return buf.Bytes(), nil
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
