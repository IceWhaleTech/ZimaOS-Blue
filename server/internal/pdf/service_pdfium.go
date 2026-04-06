//go:build !darwin

package pdf

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
)

func defaultPDFRuntimePoolFactory() pdfRuntimePoolFactory {
	return func(config pdfRuntimeConfig) (any, error) {
		return webassembly.Init(webassembly.Config{
			MinIdle:      config.MinIdle,
			MaxIdle:      config.MaxIdle,
			MaxTotal:     config.MaxTotal,
			ReuseWorkers: config.ReuseWorkers,
			Stdout:       config.Stdout,
			Stderr:       config.Stderr,
			WASM:         config.WASM,
		})
	}
}

// Info reads document-level information for a PDF file.
func (s *Service) Info(ctx context.Context, path string) (DocumentInfo, error) {
	if err := ctx.Err(); err != nil {
		return DocumentInfo{}, err
	}
	resolvedPath, stat, err := resolvePath(path)
	if err != nil {
		return DocumentInfo{}, err
	}
	nativeInfo, nativeOK, nativeErr := tryNativePDFInfo(ctx, resolvedPath, stat)
	if nativePDFShouldPreferInfo() && nativeOK && nativeErr == nil {
		return nativeInfo, nil
	}
	instance, err := s.getInstance(ctx)
	if err != nil {
		if nativeOK {
			if nativeErr != nil {
				return DocumentInfo{}, nativeErr
			}
			return nativeInfo, nil
		}
		return DocumentInfo{}, err
	}
	defer instance.Close()

	doc, err := openDocument(instance, resolvedPath)
	if err != nil {
		if nativeOK {
			if nativeErr != nil {
				return DocumentInfo{}, nativeErr
			}
			return nativeInfo, nil
		}
		return DocumentInfo{}, err
	}
	defer closeDocument(instance, doc.Document)

	info, infoErr := buildDocumentInfo(instance, resolvedPath, stat, doc.Document)
	if infoErr == nil {
		return info, nil
	}
	if nativeOK {
		if nativeErr != nil {
			return DocumentInfo{}, nativeErr
		}
		return nativeInfo, nil
	}
	return DocumentInfo{}, infoErr
}

// Extract reads text content from a PDF file.
func (s *Service) Extract(ctx context.Context, req ExtractRequest) (ExtractResult, error) {
	if err := ctx.Err(); err != nil {
		return ExtractResult{}, err
	}
	resolvedPath, stat, err := resolvePath(req.Path)
	if err != nil {
		return ExtractResult{}, err
	}
	nativeResult, nativeOK, nativeErr := tryNativePDFExtract(ctx, req, resolvedPath, stat)
	if nativePDFShouldPreferExtract(req) && nativeOK && nativeErr == nil && nativePDFResultShouldShortCircuit(req, nativeResult) {
		return nativeResult, nil
	}
	result, err := s.extractWithPDFium(ctx, req, resolvedPath, stat)
	if err == nil {
		return result, nil
	}
	if nativeOK {
		if nativeErr != nil {
			return ExtractResult{}, nativeErr
		}
		return nativeResult, nil
	}
	return ExtractResult{}, err
}

func (s *Service) extractPageOCR(ctx context.Context, instance pdfium.Pdfium, document references.FPDF_DOCUMENT, pageNumber int) (ocrruntime.Result, error) {
	rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
		DPI:  ocrRenderDPI,
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: document, Index: pageNumber - 1}},
	})
	if err != nil {
		return ocrruntime.Result{}, fmt.Errorf("render page: %w", err)
	}
	defer rendered.Cleanup()

	var buf bytes.Buffer
	if err := png.Encode(&buf, rendered.Result.Image); err != nil {
		return ocrruntime.Result{}, fmt.Errorf("encode page image: %w", err)
	}
	return s.ocr.Extract(ctx, buf.Bytes())
}

func (s *Service) extractPageVision(ctx context.Context, instance pdfium.Pdfium, document references.FPDF_DOCUMENT, pageNumber int, vision VisionService) (VisionResult, error) {
	rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
		DPI:  ocrRenderDPI,
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: document, Index: pageNumber - 1}},
	})
	if err != nil {
		return VisionResult{}, fmt.Errorf("render page: %w", err)
	}
	defer rendered.Cleanup()

	var buf bytes.Buffer
	if err := png.Encode(&buf, rendered.Result.Image); err != nil {
		return VisionResult{}, fmt.Errorf("encode page image: %w", err)
	}
	return vision.Extract(ctx, buf.Bytes())
}

func pdfiumRuntimeURLCandidates() []string {
	return skillbundle.GitHubRawURLCandidates(
		pdfiumRuntimeRepoOwner,
		pdfiumRuntimeRepoName,
		pdfiumRuntimeRepoRef,
		pdfiumRuntimeSourcePath,
	)
}

func (s *Service) getInstance(ctx context.Context) (pdfium.Pdfium, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	pool, ok := s.pool.(pdfium.Pool)
	if !ok || pool == nil {
		return nil, fmt.Errorf("pdfium pool unavailable")
	}
	instance, err := pool.GetInstance(instanceAcquireTimeout)
	if err != nil {
		return nil, fmt.Errorf("acquire pdfium instance: %w", err)
	}
	return instance, nil
}

func (s *Service) ensureReady(ctx context.Context) error {
	s.initOnce.Do(func() {
		wasmBytes, err := s.ensureRuntimeWASMBytes(ctx)
		if err != nil {
			s.initErr = err
			return
		}
		if s.initPool == nil {
			s.initErr = fmt.Errorf("pdfium runtime factory unavailable")
			return
		}
		s.pool, s.initErr = s.initPool(pdfRuntimeConfig{
			MinIdle:      1,
			MaxIdle:      1,
			MaxTotal:     1,
			ReuseWorkers: true,
			Stdout:       io.Discard,
			Stderr:       io.Discard,
			WASM:         wasmBytes,
		})
		if s.initErr != nil {
			s.initErr = fmt.Errorf("init pdfium: %w", s.initErr)
		}
	})
	return s.initErr
}

func (s *Service) ensureRuntimeWASMBytes(ctx context.Context) ([]byte, error) {
	wasmPath := filepath.Join(s.runtimeDir, pdfiumRuntimeFileName)
	if _, err := os.Stat(wasmPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat PDF runtime: %w", err)
		}
		if !s.autoDownload {
			return nil, fmt.Errorf("missing PDF runtime %s", pdfiumRuntimeFileName)
		}
		if err := os.MkdirAll(s.runtimeDir, 0o750); err != nil {
			return nil, fmt.Errorf("create PDF runtime dir: %w", err)
		}
		if err := s.downloadWithFallback(ctx, pdfiumRuntimeURLCandidates(), wasmPath, "PDF runtime"); err != nil {
			return nil, err
		}
	}
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("read PDF runtime: %w", err)
	}
	return wasmBytes, nil
}

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
