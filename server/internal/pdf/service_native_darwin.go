//go:build darwin

package pdf

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// macOS PDFKit is the primary native PDF extraction engine on darwin.
const darwinPDFKitEngineName = "pdfkit/native"

const (
	darwinPDFDisplayBoxCropBox   = 1
	darwinBitmapImageFileTypePNG = 4
	darwinPDFPointsPerInch       = 72.0
)

type darwinPDFPoint struct {
	X float64
	Y float64
}

type darwinPDFSize struct {
	Width  float64
	Height float64
}

type darwinPDFRect struct {
	Origin darwinPDFPoint
	Size   darwinPDFSize
}

type darwinPDFKitPage struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

type darwinPDFKitOutput struct {
	PageCount int                `json:"page_count"`
	Metadata  map[string]string  `json:"metadata,omitempty"`
	Outline   []OutlineEntry     `json:"outline,omitempty"`
	Pages     []darwinPDFKitPage `json:"pages"`
}

var (
	darwinPDFKitOnce sync.Once
	darwinPDFKitErr  error

	darwinPDFSelAlloc              objc.SEL
	darwinPDFSelInit               objc.SEL
	darwinPDFSelInitWithURL        objc.SEL
	darwinPDFSelRelease            objc.SEL
	darwinPDFSelStringWithUTF8     objc.SEL
	darwinPDFSelUTF8String         objc.SEL
	darwinPDFSelFileURLWithPath    objc.SEL
	darwinPDFSelPageCount          objc.SEL
	darwinPDFSelPageAtIndex        objc.SEL
	darwinPDFSelString             objc.SEL
	darwinPDFSelDocumentAttributes objc.SEL
	darwinPDFSelOutlineRoot        objc.SEL
	darwinPDFSelAllKeys            objc.SEL
	darwinPDFSelObjectAtIndex      objc.SEL
	darwinPDFSelObjectForKey       objc.SEL
	darwinPDFSelCount              objc.SEL
	darwinPDFSelDescription        objc.SEL
	darwinPDFSelLabel              objc.SEL
	darwinPDFSelNumberOfChildren   objc.SEL
	darwinPDFSelChildAtIndex       objc.SEL
	darwinPDFSelDestination        objc.SEL
	darwinPDFSelAction             objc.SEL
	darwinPDFSelPage               objc.SEL
	darwinPDFSelIndexForPage       objc.SEL
	darwinPDFSelBoundsForBox       objc.SEL
	darwinPDFSelThumbnailOfSizeBox objc.SEL
	darwinPDFSelTIFFRepresentation objc.SEL
	darwinPDFSelImageRepWithData   objc.SEL
	darwinPDFSelRepresentationType objc.SEL
	darwinPDFSelBytes              objc.SEL
	darwinPDFSelLength             objc.SEL

	darwinPDFKitExtractFunc = extractDarwinPDFKit
)

var errDarwinPDFKitUnavailable = errors.New("macOS PDFKit native extraction unavailable")

func nativePDFEngineName() string {
	return darwinPDFKitEngineName
}

func nativePDFShouldPreferInfo() bool {
	return true
}

func nativePDFShouldPreferExtract(req ExtractRequest) bool {
	return !req.IncludeLayout
}

func nativePDFResultShouldShortCircuit(req ExtractRequest, result ExtractResult) bool {
	if req.DisableOCR && req.DisableVision {
		return true
	}
	if strings.TrimSpace(result.Text) == "" {
		return false
	}
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "has no extractable text") {
			return false
		}
	}
	return true
}

func tryNativePDFInfo(ctx context.Context, path string, stat os.FileInfo) (DocumentInfo, bool, error) {
	output, ok, err := runDarwinPDFKitExtract(ctx, path)
	if !ok {
		return DocumentInfo{}, false, nil
	}
	if err != nil {
		return DocumentInfo{}, true, err
	}
	return darwinPDFKitDocumentInfo(path, stat, output.PageCount, output.Metadata), true, nil
}

func tryNativePDFExtract(ctx context.Context, req ExtractRequest, path string, stat os.FileInfo) (ExtractResult, bool, error) {
	output, ok, err := runDarwinPDFKitExtract(ctx, path)
	if !ok {
		return ExtractResult{}, false, nil
	}
	if err != nil {
		return ExtractResult{}, true, err
	}

	info := darwinPDFKitDocumentInfo(path, stat, output.PageCount, output.Metadata)
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
	remainingMarkdownChars := maxChars
	pageEntries := make([]PageText, 0, len(selectedPages))
	markdownSections := make([]string, 0, len(selectedPages))
	presentations := make([]pdfPresentation, 0, len(selectedPages))

	for _, pageNumber := range selectedPages {
		rawPageText := canonicalizeExtractedPDFText(darwinPDFKitPageText(output, pageNumber))
		pageOnlyText := normalizeText(rawPageText)
		source := "text"
		if pageOnlyText == "" {
			source = "none"
			result.Warnings = append(result.Warnings, fmt.Sprintf("page %d has no extractable text", pageNumber))
		}
		rawPageText = effectiveRawPDFText(rawPageText, pageOnlyText)
		presentation := buildSyntheticPDFPresentation(pdfPageSource{
			PageNumber: pageNumber,
			RawText:    rawPageText,
			Text:       pageOnlyText,
			Source:     source,
		})
		presentations = append(presentations, presentation)

		prefix := resultTextPrefix(pageNumber, len(selectedPages))
		pageOutput := presentation.Text
		if prefix != "" {
			pageOutput = prefix + presentation.Text
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

		if req.IncludeMarkdown && strings.TrimSpace(presentation.Markdown) != "" {
			pageMarkdown := strings.TrimSpace(presentation.Markdown)
			if len(selectedPages) > 1 {
				pageMarkdown = fmt.Sprintf("[Page %d]\n\n%s", pageNumber, pageMarkdown)
			}
			clippedMarkdown, markdownChars, markdownWasClipped := clipRunes(pageMarkdown, remainingMarkdownChars)
			if clippedMarkdown != "" {
				markdownSections = append(markdownSections, clippedMarkdown)
				remainingMarkdownChars -= markdownChars
			}
			if markdownWasClipped {
				result.Truncated = true
			}
		}

		result.SelectedPages = append(result.SelectedPages, pageNumber)
		if req.IncludePages || req.IncludeLayout {
			pageEntryText := presentation.Text
			pageEntryChars := utf8.RuneCountInString(presentation.Text)
			if wasClipped {
				availableForText := outputChars - utf8.RuneCountInString(prefix)
				if availableForText < 0 {
					availableForText = 0
				}
				pageEntryText, pageEntryChars, _ = clipRunes(presentation.Text, availableForText)
			}
			pageEntryRawText := rawPageText
			if rawWasClipped {
				availableForRawText := rawOutputChars - utf8.RuneCountInString(prefix)
				if availableForRawText < 0 {
					availableForRawText = 0
				}
				pageEntryRawText, _, _ = clipRunes(rawPageText, availableForRawText)
			}
			pageEntry := PageText{Number: pageNumber, Text: pageEntryText, CharCount: pageEntryChars, Source: source, Empty: strings.TrimSpace(presentation.Text) == ""}
			if strings.TrimSpace(pageEntryRawText) != "" && strings.TrimSpace(pageEntryRawText) != strings.TrimSpace(pageEntryText) {
				pageEntry.RawText = pageEntryRawText
			}
			if req.IncludeMarkdown {
				pageEntry.Markdown = presentation.Markdown
			}
			if req.IncludeLayout {
				pageEntry.Blocks = presentation.Blocks
				pageEntry.Tables = presentation.Tables
			}
			pageEntries = append(pageEntries, pageEntry)
		}

		if wasClipped || rawWasClipped || remainingChars <= 0 || remainingRawChars <= 0 {
			result.Truncated = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("output truncated at %d characters", maxChars))
			break
		}
	}

	if req.IncludePages || req.IncludeLayout {
		result.Pages = pageEntries
	}
	if req.IncludeMarkdown && len(markdownSections) > 0 {
		result.Markdown = strings.Join(markdownSections, "\n\n---\n\n")
	}
	if req.IncludeOutline {
		if outline := darwinPDFKitOutline(output, selectedPages); len(outline) > 0 {
			result.Outline = outline
		} else {
			outline := make([]OutlineEntry, 0, 8)
			for _, presentation := range presentations {
				outline = append(outline, presentation.HeadingText...)
			}
			result.Outline = outline
		}
	}
	result.CharCount = utf8.RuneCountInString(result.Text)
	return result, true, nil
}

func darwinPDFKitDocumentInfo(path string, stat os.FileInfo, pageCount int, metadata map[string]string) DocumentInfo {
	return DocumentInfo{
		Path:       path,
		FileName:   filepath.Base(path),
		SizeBytes:  stat.Size(),
		ModifiedAt: stat.ModTime().UTC(),
		PageCount:  pageCount,
		Engine:     nativePDFEngineName(),
		Metadata:   cloneDarwinPDFMetadata(metadata),
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

func darwinPDFKitOutline(output *darwinPDFKitOutput, selectedPages []int) []OutlineEntry {
	if output == nil || len(output.Outline) == 0 {
		return nil
	}
	if len(selectedPages) == 0 {
		return append([]OutlineEntry(nil), output.Outline...)
	}
	selected := make(map[int]struct{}, len(selectedPages))
	for _, pageNumber := range selectedPages {
		selected[pageNumber] = struct{}{}
	}
	out := make([]OutlineEntry, 0, len(output.Outline))
	for _, entry := range output.Outline {
		if entry.PageNumber == 0 {
			out = append(out, entry)
			continue
		}
		if _, ok := selected[entry.PageNumber]; ok {
			out = append(out, entry)
		}
	}
	return out
}

func cloneDarwinPDFMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(metadata))
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func runDarwinPDFKitExtract(ctx context.Context, path string) (*darwinPDFKitOutput, bool, error) {
	output, err := darwinPDFKitExtractFunc(ctx, path)
	if err != nil {
		if errors.Is(err, errDarwinPDFKitUnavailable) {
			return nil, false, nil
		}
		return nil, true, fmt.Errorf("extract pdf with macOS PDFKit: %w", err)
	}
	if output == nil {
		return nil, true, fmt.Errorf("extract pdf with macOS PDFKit: empty result")
	}
	if output.PageCount < 0 {
		return nil, true, fmt.Errorf("invalid macOS PDFKit page count: %d", output.PageCount)
	}
	return output, true, nil
}

func initDarwinPDFKitSelectors() error {
	darwinPDFKitOnce.Do(func() {
		if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			darwinPDFKitErr = fmt.Errorf("%w: load Foundation.framework: %v", errDarwinPDFKitUnavailable, err)
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/PDFKit.framework/PDFKit", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			darwinPDFKitErr = fmt.Errorf("%w: load PDFKit.framework: %v", errDarwinPDFKitUnavailable, err)
			return
		}
		if _, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_LAZY|purego.RTLD_GLOBAL); err != nil {
			darwinPDFKitErr = fmt.Errorf("%w: load AppKit.framework: %v", errDarwinPDFKitUnavailable, err)
			return
		}

		darwinPDFSelAlloc = objc.RegisterName("alloc")
		darwinPDFSelInit = objc.RegisterName("init")
		darwinPDFSelInitWithURL = objc.RegisterName("initWithURL:")
		darwinPDFSelRelease = objc.RegisterName("release")
		darwinPDFSelStringWithUTF8 = objc.RegisterName("stringWithUTF8String:")
		darwinPDFSelUTF8String = objc.RegisterName("UTF8String")
		darwinPDFSelFileURLWithPath = objc.RegisterName("fileURLWithPath:")
		darwinPDFSelPageCount = objc.RegisterName("pageCount")
		darwinPDFSelPageAtIndex = objc.RegisterName("pageAtIndex:")
		darwinPDFSelString = objc.RegisterName("string")
		darwinPDFSelDocumentAttributes = objc.RegisterName("documentAttributes")
		darwinPDFSelOutlineRoot = objc.RegisterName("outlineRoot")
		darwinPDFSelAllKeys = objc.RegisterName("allKeys")
		darwinPDFSelObjectAtIndex = objc.RegisterName("objectAtIndex:")
		darwinPDFSelObjectForKey = objc.RegisterName("objectForKey:")
		darwinPDFSelCount = objc.RegisterName("count")
		darwinPDFSelDescription = objc.RegisterName("description")
		darwinPDFSelLabel = objc.RegisterName("label")
		darwinPDFSelNumberOfChildren = objc.RegisterName("numberOfChildren")
		darwinPDFSelChildAtIndex = objc.RegisterName("childAtIndex:")
		darwinPDFSelDestination = objc.RegisterName("destination")
		darwinPDFSelAction = objc.RegisterName("action")
		darwinPDFSelPage = objc.RegisterName("page")
		darwinPDFSelIndexForPage = objc.RegisterName("indexForPage:")
		darwinPDFSelBoundsForBox = objc.RegisterName("boundsForBox:")
		darwinPDFSelThumbnailOfSizeBox = objc.RegisterName("thumbnailOfSize:forBox:")
		darwinPDFSelTIFFRepresentation = objc.RegisterName("TIFFRepresentation")
		darwinPDFSelImageRepWithData = objc.RegisterName("imageRepWithData:")
		darwinPDFSelRepresentationType = objc.RegisterName("representationUsingType:properties:")
		darwinPDFSelBytes = objc.RegisterName("bytes")
		darwinPDFSelLength = objc.RegisterName("length")
	})
	return darwinPDFKitErr
}

func extractDarwinPDFKit(ctx context.Context, path string) (*darwinPDFKitOutput, error) {
	if err := initDarwinPDFKitSelectors(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pool := newDarwinPDFAutoreleasePool()
	if pool != 0 {
		defer releaseDarwinPDFObject(pool)
	}

	urlClass := objc.ID(objc.GetClass("NSURL"))
	documentClass := objc.ID(objc.GetClass("PDFDocument"))
	if urlClass == 0 || documentClass == 0 {
		return nil, errDarwinPDFKitUnavailable
	}

	nsPath := darwinPDFNSString(path)
	if nsPath == 0 {
		return nil, fmt.Errorf("create path string for PDFKit")
	}
	url := urlClass.Send(darwinPDFSelFileURLWithPath, nsPath)
	if url == 0 {
		return nil, fmt.Errorf("create file URL for PDFKit")
	}

	document := documentClass.Send(darwinPDFSelAlloc).Send(darwinPDFSelInitWithURL, url)
	if document == 0 {
		return nil, fmt.Errorf("open PDF document")
	}
	defer releaseDarwinPDFObject(document)

	pageCount := int(objc.Send[uint64](document, darwinPDFSelPageCount))
	pages := make([]darwinPDFKitPage, 0, pageCount)
	for index := 0; index < pageCount; index++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page := document.Send(darwinPDFSelPageAtIndex, uintptr(index))
		text := ""
		if page != 0 {
			text = darwinPDFObjectString(page.Send(darwinPDFSelString))
		}
		pages = append(pages, darwinPDFKitPage{
			Number: index + 1,
			Text:   text,
		})
	}

	return &darwinPDFKitOutput{
		PageCount: pageCount,
		Metadata:  darwinPDFDocumentAttributes(document),
		Outline:   darwinPDFOutlineEntries(document, document.Send(darwinPDFSelOutlineRoot), 1),
		Pages:     pages,
	}, nil
}

func renderDarwinPDFKitPagePNG(ctx context.Context, path string, pageNumber int, dpi int) ([]byte, bool, error) {
	if err := initDarwinPDFKitSelectors(); err != nil {
		if errors.Is(err, errDarwinPDFKitUnavailable) {
			return nil, false, nil
		}
		return nil, true, err
	}
	if err := ctx.Err(); err != nil {
		return nil, true, err
	}

	pool := newDarwinPDFAutoreleasePool()
	if pool != 0 {
		defer releaseDarwinPDFObject(pool)
	}

	urlClass := objc.ID(objc.GetClass("NSURL"))
	documentClass := objc.ID(objc.GetClass("PDFDocument"))
	imageRepClass := objc.ID(objc.GetClass("NSBitmapImageRep"))
	if urlClass == 0 || documentClass == 0 || imageRepClass == 0 {
		return nil, false, nil
	}

	nsPath := darwinPDFNSString(path)
	if nsPath == 0 {
		return nil, true, fmt.Errorf("create path string for PDFKit")
	}
	url := urlClass.Send(darwinPDFSelFileURLWithPath, nsPath)
	if url == 0 {
		return nil, true, fmt.Errorf("create file URL for PDFKit")
	}

	document := documentClass.Send(darwinPDFSelAlloc).Send(darwinPDFSelInitWithURL, url)
	if document == 0 {
		return nil, true, fmt.Errorf("open PDF document")
	}
	defer releaseDarwinPDFObject(document)

	pageCount := int(objc.Send[uint64](document, darwinPDFSelPageCount))
	if pageNumber < 1 || pageNumber > pageCount {
		return nil, true, fmt.Errorf("page %d out of range (document has %d pages)", pageNumber, pageCount)
	}
	page := document.Send(darwinPDFSelPageAtIndex, uintptr(pageNumber-1))
	if page == 0 {
		return nil, true, fmt.Errorf("open PDF page %d", pageNumber)
	}

	bounds := objc.Send[darwinPDFRect](page, darwinPDFSelBoundsForBox, uintptr(darwinPDFDisplayBoxCropBox))
	targetSize := darwinPDFSize{
		Width:  math.Max(1, math.Ceil(bounds.Size.Width*float64(dpi)/darwinPDFPointsPerInch)),
		Height: math.Max(1, math.Ceil(bounds.Size.Height*float64(dpi)/darwinPDFPointsPerInch)),
	}
	thumbnail := objc.Send[objc.ID](page, darwinPDFSelThumbnailOfSizeBox, targetSize, uintptr(darwinPDFDisplayBoxCropBox))
	if thumbnail == 0 {
		return nil, true, fmt.Errorf("render PDF page %d thumbnail", pageNumber)
	}

	tiffData := thumbnail.Send(darwinPDFSelTIFFRepresentation)
	if tiffData == 0 {
		return nil, true, fmt.Errorf("encode PDF page %d TIFF", pageNumber)
	}
	imageRep := imageRepClass.Send(darwinPDFSelImageRepWithData, tiffData)
	if imageRep == 0 {
		return nil, true, fmt.Errorf("create bitmap representation for page %d", pageNumber)
	}
	pngData := objc.Send[objc.ID](imageRep, darwinPDFSelRepresentationType, uintptr(darwinBitmapImageFileTypePNG), objc.ID(0))
	if pngData == 0 {
		return nil, true, fmt.Errorf("encode PDF page %d PNG", pageNumber)
	}
	out := darwinPDFDataBytes(pngData)
	if len(out) == 0 {
		return nil, true, fmt.Errorf("render PDF page %d PNG: empty output", pageNumber)
	}
	return out, true, nil
}

func darwinPDFDocumentAttributes(document objc.ID) map[string]string {
	if document == 0 {
		return nil
	}
	attributes := document.Send(darwinPDFSelDocumentAttributes)
	if attributes == 0 {
		return nil
	}
	keys := attributes.Send(darwinPDFSelAllKeys)
	if keys == 0 {
		return nil
	}
	count := int(objc.Send[uint64](keys, darwinPDFSelCount))
	if count <= 0 {
		return nil
	}
	out := make(map[string]string, count)
	for idx := 0; idx < count; idx++ {
		keyObject := keys.Send(darwinPDFSelObjectAtIndex, uintptr(idx))
		if keyObject == 0 {
			continue
		}
		key := strings.TrimSpace(darwinPDFObjectString(keyObject))
		if key == "" {
			continue
		}
		valueObject := attributes.Send(darwinPDFSelObjectForKey, keyObject)
		if valueObject == 0 {
			continue
		}
		value := strings.TrimSpace(darwinPDFObjectString(valueObject))
		if value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func darwinPDFOutlineEntries(document objc.ID, node objc.ID, level int) []OutlineEntry {
	if document == 0 || node == 0 {
		return nil
	}
	childCount := int(objc.Send[uint64](node, darwinPDFSelNumberOfChildren))
	if childCount <= 0 {
		return nil
	}
	out := make([]OutlineEntry, 0, childCount)
	for idx := 0; idx < childCount; idx++ {
		child := node.Send(darwinPDFSelChildAtIndex, uintptr(idx))
		if child == 0 {
			continue
		}
		title := strings.TrimSpace(darwinPDFObjectString(child.Send(darwinPDFSelLabel)))
		if title != "" {
			out = append(out, OutlineEntry{
				Title:      title,
				Level:      max(1, level),
				PageNumber: darwinPDFOutlinePageNumber(document, child),
			})
		}
		out = append(out, darwinPDFOutlineEntries(document, child, level+1)...)
	}
	return out
}

func darwinPDFOutlinePageNumber(document objc.ID, item objc.ID) int {
	if document == 0 || item == 0 {
		return 0
	}
	destination := item.Send(darwinPDFSelDestination)
	if destination == 0 {
		action := item.Send(darwinPDFSelAction)
		if action != 0 {
			destination = action.Send(darwinPDFSelDestination)
		}
	}
	if destination == 0 {
		return 0
	}
	page := destination.Send(darwinPDFSelPage)
	if page == 0 {
		return 0
	}
	return int(objc.Send[uint64](document, darwinPDFSelIndexForPage, page)) + 1
}

func newDarwinPDFAutoreleasePool() objc.ID {
	poolClass := objc.ID(objc.GetClass("NSAutoreleasePool"))
	if poolClass == 0 {
		return 0
	}
	return poolClass.Send(darwinPDFSelAlloc).Send(darwinPDFSelInit)
}

func releaseDarwinPDFObject(id objc.ID) {
	if id == 0 {
		return
	}
	id.Send(darwinPDFSelRelease)
}

func darwinPDFNSString(value string) objc.ID {
	cstr := append([]byte(value), 0)
	cls := objc.ID(objc.GetClass("NSString"))
	if cls == 0 {
		return 0
	}
	return cls.Send(darwinPDFSelStringWithUTF8, uintptr(unsafe.Pointer(&cstr[0])))
}

func darwinPDFGoString(nsStr objc.ID) string {
	if nsStr == 0 {
		return ""
	}
	ptr := objc.Send[uintptr](nsStr, darwinPDFSelUTF8String)
	if ptr == 0 {
		return ""
	}
	return darwinPDFCString(ptr)
}

func darwinPDFObjectString(object objc.ID) string {
	if object == 0 {
		return ""
	}
	description := object
	if darwinPDFSelDescription != 0 {
		if described := object.Send(darwinPDFSelDescription); described != 0 {
			description = described
		}
	}
	return darwinPDFGoString(description)
}

func darwinPDFDataBytes(data objc.ID) []byte {
	if data == 0 {
		return nil
	}
	length := int(objc.Send[uint64](data, darwinPDFSelLength))
	if length <= 0 {
		return nil
	}
	ptr := objc.Send[uintptr](data, darwinPDFSelBytes)
	if ptr == 0 {
		return nil
	}
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length)...)
}

//go:nocheckptr
func darwinPDFCString(ptr uintptr) string {
	length := 0
	for {
		b := *(*byte)(unsafe.Add(unsafe.Pointer(ptr), length))
		if b == 0 {
			break
		}
		length++
	}
	if length == 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), length))
}
