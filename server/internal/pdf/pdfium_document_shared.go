package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
)

func buildDocumentInfo(instance pdfium.Pdfium, path string, stat os.FileInfo, document references.FPDF_DOCUMENT) (DocumentInfo, error) {
	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: document})
	if err != nil {
		return DocumentInfo{}, fmt.Errorf("get page count: %w", err)
	}
	info := DocumentInfo{
		Path:       path,
		FileName:   filepath.Base(path),
		SizeBytes:  stat.Size(),
		ModifiedAt: stat.ModTime().UTC(),
		PageCount:  pageCount.PageCount,
		Engine:     engineName,
	}
	if metadata, err := instance.GetMetaData(&requests.GetMetaData{Document: document}); err == nil && metadata != nil {
		info.Metadata = metadataToMap(metadata.Tags)
	}
	return info, nil
}

func openDocument(instance pdfium.Pdfium, path string) (*responses.OpenDocument, error) {
	doc, err := instance.OpenDocument(&requests.OpenDocument{FilePath: &path})
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	return doc, nil
}

func closeDocument(instance pdfium.Pdfium, document references.FPDF_DOCUMENT) {
	if document == "" {
		return
	}
	_, _ = instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: document})
}

func metadataToMap(tags []responses.GetMetaDataTag) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := strings.TrimSpace(tag.Tag)
		value := strings.TrimSpace(tag.Value)
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
