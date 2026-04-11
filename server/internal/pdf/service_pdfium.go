//go:build !darwin

package pdf

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"

	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
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
