package convert

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

func readDOCXLocal(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open docx archive: %w", err)
	}
	defer reader.Close()

	file := findZipFile(reader.File, "word/document.xml")
	if file == nil {
		return "", fmt.Errorf("docx document.xml missing")
	}
	text, err := extractOOXMLText(file, ooxmlTextExtractConfig{
		ParagraphElements: map[string]struct{}{"p": {}},
		TextElements:      map[string]struct{}{"t": {}, "instrText": {}},
		BreakElements:     map[string]string{"tab": "\t", "br": "\n", "cr": "\n"},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("docx document contained no readable text")
	}
	return text, nil
}

func readPPTXLocal(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open pptx archive: %w", err)
	}
	defer reader.Close()

	slides := collectSortedZipFiles(reader.File, "ppt/slides/slide", ".xml")
	if len(slides) == 0 {
		return "", fmt.Errorf("pptx slides missing")
	}

	parts := make([]string, 0, len(slides))
	for idx, slide := range slides {
		text, err := extractOOXMLText(slide, ooxmlTextExtractConfig{
			ParagraphElements: map[string]struct{}{"p": {}},
			TextElements:      map[string]struct{}{"t": {}},
			BreakElements:     map[string]string{"br": "\n", "tab": "\t"},
		})
		if err != nil {
			return "", err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("[Slide %d]\n%s", idx+1, text))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("pptx slides contained no readable text")
	}
	return strings.Join(parts, "\n\n"), nil
}

type ooxmlTextExtractConfig struct {
	ParagraphElements map[string]struct{}
	TextElements      map[string]struct{}
	BreakElements     map[string]string
}

func extractOOXMLText(file *zip.File, cfg ooxmlTextExtractConfig) (string, error) {
	if file == nil {
		return "", fmt.Errorf("ooxml file is required")
	}
	rc, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open %s: %w", file.Name, err)
	}
	defer rc.Close()

	decoder := xml.NewDecoder(rc)
	var (
		captureText bool
		current     strings.Builder
		paragraphs  []string
	)

	flushParagraph := func() {
		text := normalizeOOXMLParagraphText(current.String())
		current.Reset()
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("decode %s: %w", file.Name, err)
		}
		switch typed := token.(type) {
		case xml.StartElement:
			name := typed.Name.Local
			if _, ok := cfg.TextElements[name]; ok {
				captureText = true
			}
			if replacement, ok := cfg.BreakElements[name]; ok {
				current.WriteString(replacement)
			}
		case xml.EndElement:
			name := typed.Name.Local
			if _, ok := cfg.TextElements[name]; ok {
				captureText = false
			}
			if _, ok := cfg.ParagraphElements[name]; ok {
				flushParagraph()
			}
		case xml.CharData:
			if captureText {
				current.WriteString(string(typed))
			}
		}
	}
	flushParagraph()
	return strings.Join(paragraphs, "\n\n"), nil
}

func normalizeOOXMLParagraphText(raw string) string {
	raw = strings.ReplaceAll(raw, "\u00a0", " ")
	lines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func findZipFile(files []*zip.File, name string) *zip.File {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, file := range files {
		if strings.ToLower(strings.TrimSpace(file.Name)) == target {
			return file
		}
	}
	return nil
}

func collectSortedZipFiles(files []*zip.File, prefix, suffix string) []*zip.File {
	type numberedFile struct {
		order int
		file  *zip.File
	}
	items := make([]numberedFile, 0, len(files))
	lowerPrefix := strings.ToLower(prefix)
	lowerSuffix := strings.ToLower(suffix)
	for _, file := range files {
		name := strings.ToLower(strings.TrimSpace(file.Name))
		if !strings.HasPrefix(name, lowerPrefix) || !strings.HasSuffix(name, lowerSuffix) {
			continue
		}
		middle := strings.TrimSuffix(strings.TrimPrefix(name, lowerPrefix), lowerSuffix)
		order, err := strconv.Atoi(middle)
		if err != nil {
			continue
		}
		items = append(items, numberedFile{order: order, file: file})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].order == items[j].order {
			return items[i].file.Name < items[j].file.Name
		}
		return items[i].order < items[j].order
	})
	out := make([]*zip.File, 0, len(items))
	for _, item := range items {
		out = append(out, item.file)
	}
	return out
}
