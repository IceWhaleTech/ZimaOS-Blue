package tools

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkOfficeBuildZipFast(b *testing.B) {
	entries := benchmarkOfficeZipEntries()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := officeBuildZip(entries); err != nil {
			b.Fatalf("officeBuildZip failed: %v", err)
		}
	}
}

func BenchmarkOfficeBuildZipLegacy(b *testing.B) {
	entries := benchmarkOfficeZipEntries()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := legacyOfficeBuildZip(entries); err != nil {
			b.Fatalf("legacyOfficeBuildZip failed: %v", err)
		}
	}
}

func benchmarkOfficeZipEntries() []officeZipEntry {
	entries := []officeZipEntry{
		{Name: "[Content_Types].xml", Content: strings.Repeat("<Override PartName=\"/a.xml\" ContentType=\"application/xml\"/>", 32)},
		{Name: "_rels/.rels", Content: strings.Repeat("<Relationship Id=\"rId1\" Target=\"doc.xml\"/>", 24)},
	}
	for idx := 0; idx < 18; idx++ {
		var content strings.Builder
		content.Grow(16 << 10)
		content.WriteString(`<?xml version="1.0" encoding="UTF-8"?><root>`)
		for line := 0; line < 220; line++ {
			content.WriteString(`<p>`)
			content.WriteString(fmt.Sprintf("Entry %d line %d benchmark content for native Office zip packaging throughput.", idx+1, line+1))
			content.WriteString(`</p>`)
		}
		content.WriteString(`</root>`)
		entries = append(entries, officeZipEntry{
			Name:    fmt.Sprintf("doc/part%d.xml", idx+1),
			Content: content.String(),
		})
	}
	return entries
}

func legacyOfficeBuildZip(entries []officeZipEntry) ([]byte, error) {
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
			return nil, err
		}
		if _, err := w.Write([]byte(entry.Content)); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
