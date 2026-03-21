//go:build darwin

package pdf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

// macOS PDFKit is the primary native PDF extraction engine on darwin.
const darwinPDFKitEngineName = "pdfkit/swift"

const darwinPDFKitExtractScript = `import Foundation
import PDFKit

struct Page: Codable {
    let number: Int
    let text: String
}

struct Output: Codable {
    let page_count: Int
    let pages: [Page]
}

let args = CommandLine.arguments
guard args.count > 1 else {
    fputs("missing_path\n", stderr)
    exit(2)
}

let path = args[1]
let url = URL(fileURLWithPath: path)
guard let doc = PDFDocument(url: url) else {
    fputs("open_failed\n", stderr)
    exit(3)
}

var pages: [Page] = []
pages.reserveCapacity(doc.pageCount)
for idx in 0..<doc.pageCount {
    let text = doc.page(at: idx)?.string ?? ""
    pages.append(Page(number: idx + 1, text: text))
}

let output = Output(page_count: doc.pageCount, pages: pages)
let encoder = JSONEncoder()
let data = try encoder.encode(output)
FileHandle.standardOutput.write(data)
`

type darwinPDFKitPage struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

type darwinPDFKitOutput struct {
	PageCount int                `json:"page_count"`
	Pages     []darwinPDFKitPage `json:"pages"`
}

var (
	darwinPDFKitSwiftOnce sync.Once
	darwinPDFKitSwiftPath string
)

func nativePDFEngineName() string {
	return darwinPDFKitEngineName
}

func tryNativePDFInfo(ctx context.Context, path string, stat os.FileInfo) (DocumentInfo, bool, error) {
	output, ok, err := runDarwinPDFKitExtract(ctx, path)
	if !ok {
		return DocumentInfo{}, false, nil
	}
	if err != nil {
		return DocumentInfo{}, true, err
	}
	return darwinPDFKitDocumentInfo(path, stat, output.PageCount), true, nil
}

func tryNativePDFExtract(ctx context.Context, req ExtractRequest, path string, stat os.FileInfo) (ExtractResult, bool, error) {
	output, ok, err := runDarwinPDFKitExtract(ctx, path)
	if !ok {
		return ExtractResult{}, false, nil
	}
	if err != nil {
		return ExtractResult{}, true, err
	}

	info := darwinPDFKitDocumentInfo(path, stat, output.PageCount)
	selectedPages, warnings, err := resolveSelectedPages(info.PageCount, req.Pages, req.MaxPages)
	if err != nil {
		return ExtractResult{}, true, err
	}
	if len(selectedPages) == 0 {
		return ExtractResult{Document: info}, true, nil
	}

	maxChars := clampMaxChars(req.MaxChars)
	result := ExtractResult{Document: info, Warnings: warnings}
	remainingChars := maxChars
	remainingRawChars := maxChars

	for _, pageNumber := range selectedPages {
		rawPageText := canonicalizeExtractedPDFText(darwinPDFKitPageText(output, pageNumber))
		pageOnlyText := normalizeText(rawPageText)
		source := "text"
		if pageOnlyText == "" {
			source = "none"
			result.Warnings = append(result.Warnings, fmt.Sprintf("page %d has no extractable text", pageNumber))
		}
		rawPageText = effectiveRawPDFText(rawPageText, pageOnlyText)

		prefix := resultTextPrefix(pageNumber, len(selectedPages))
		pageOutput := pageOnlyText
		if prefix != "" {
			pageOutput = prefix + pageOnlyText
		}

		clippedOutput, outputChars, wasClipped := clipRunes(pageOutput, remainingChars)
		if clippedOutput != "" {
			if result.Text != "" {
				result.Text += "\n\n"
			}
			result.Text += clippedOutput
			remainingChars -= outputChars
		}

		rawOutput := rawPageText
		if prefix != "" {
			rawOutput = prefix + rawPageText
		}
		clippedRawOutput, rawOutputChars, rawWasClipped := clipRunes(rawOutput, remainingRawChars)
		if clippedRawOutput != "" {
			if result.RawText != "" {
				result.RawText += "\n\n"
			}
			result.RawText += clippedRawOutput
			remainingRawChars -= rawOutputChars
		}

		result.SelectedPages = append(result.SelectedPages, pageNumber)
		if req.IncludePages {
			pageEntryText := pageOnlyText
			pageEntryChars := utf8.RuneCountInString(pageOnlyText)
			if wasClipped {
				availableForText := outputChars - utf8.RuneCountInString(prefix)
				if availableForText < 0 {
					availableForText = 0
				}
				pageEntryText, pageEntryChars, _ = clipRunes(pageOnlyText, availableForText)
			}
			pageEntryRawText := rawPageText
			if rawWasClipped {
				availableForRawText := rawOutputChars - utf8.RuneCountInString(prefix)
				if availableForRawText < 0 {
					availableForRawText = 0
				}
				pageEntryRawText, _, _ = clipRunes(rawPageText, availableForRawText)
			}
			pageEntry := PageText{Number: pageNumber, Text: pageEntryText, CharCount: pageEntryChars, Source: source, Empty: pageOnlyText == ""}
			if strings.TrimSpace(pageEntryRawText) != "" && strings.TrimSpace(pageEntryRawText) != strings.TrimSpace(pageEntryText) {
				pageEntry.RawText = pageEntryRawText
			}
			result.Pages = append(result.Pages, pageEntry)
		}

		if wasClipped || rawWasClipped || remainingChars <= 0 || remainingRawChars <= 0 {
			result.Truncated = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("output truncated at %d characters", maxChars))
			break
		}
	}

	result.CharCount = utf8.RuneCountInString(result.Text)
	return result, true, nil
}

func darwinPDFKitDocumentInfo(path string, stat os.FileInfo, pageCount int) DocumentInfo {
	return DocumentInfo{
		Path:       path,
		FileName:   filepath.Base(path),
		SizeBytes:  stat.Size(),
		ModifiedAt: stat.ModTime().UTC(),
		PageCount:  pageCount,
		Engine:     nativePDFEngineName(),
	}
}

func darwinPDFKitPageText(output *darwinPDFKitOutput, pageNumber int) string {
	if output == nil {
		return ""
	}
	index := pageNumber - 1
	if index >= 0 && index < len(output.Pages) && output.Pages[index].Number == pageNumber {
		return output.Pages[index].Text
	}
	for _, page := range output.Pages {
		if page.Number == pageNumber {
			return page.Text
		}
	}
	return ""
}

func runDarwinPDFKitExtract(ctx context.Context, path string) (*darwinPDFKitOutput, bool, error) {
	swiftPath := darwinPDFKitSwiftBinary()
	if swiftPath == "" {
		return nil, false, nil
	}

	cmd := exec.CommandContext(ctx, swiftPath, "-e", darwinPDFKitExtractScript, path)
	outputBytes, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if msg == "" {
				msg = exitErr.Error()
			}
			return nil, true, fmt.Errorf("extract pdf with macOS PDFKit: %s", msg)
		}
		return nil, true, fmt.Errorf("extract pdf with macOS PDFKit: %w", err)
	}

	var output darwinPDFKitOutput
	if err := json.Unmarshal(outputBytes, &output); err != nil {
		return nil, true, fmt.Errorf("decode macOS PDFKit output: %w", err)
	}
	if output.PageCount < 0 {
		return nil, true, fmt.Errorf("invalid macOS PDFKit page count: %d", output.PageCount)
	}
	return &output, true, nil
}

func darwinPDFKitSwiftBinary() string {
	darwinPDFKitSwiftOnce.Do(func() {
		if path, err := exec.LookPath("swift"); err == nil {
			darwinPDFKitSwiftPath = path
		}
	})
	return darwinPDFKitSwiftPath
}
