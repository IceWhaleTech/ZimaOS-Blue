package main

import (
  "context"
  "fmt"
  "os"
  "path/filepath"
  "time"

  pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
  "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type stubPDFService struct{}
func (s *stubPDFService) Info(ctx context.Context, path string) (pdfextract.DocumentInfo, error) {
  fmt.Println("INFO_PATH=" + path)
  return pdfextract.DocumentInfo{Path: path, FileName: filepath.Base(path), PageCount: 1, ModifiedAt: time.Now().UTC(), Engine: "stub"}, nil
}
func (s *stubPDFService) Extract(ctx context.Context, req pdfextract.ExtractRequest) (pdfextract.ExtractResult, error) {
  return pdfextract.ExtractResult{Document: pdfextract.DocumentInfo{Path: req.Path, FileName: filepath.Base(req.Path), Engine: "stub"}, Text: "ok"}, nil
}
func (s *stubPDFService) InspectForm(ctx context.Context, path string) (pdfextract.FormInspectResult, error) { return pdfextract.FormInspectResult{}, nil }
func (s *stubPDFService) FillForm(ctx context.Context, req pdfextract.FillFormRequest) (pdfextract.FillFormResult, error) { return pdfextract.FillFormResult{}, nil }

func main() {
  workspace := filepath.Join(os.Getenv("HOME"), ".zimaos-blue", "data", "workspace")
  registry := tools.NewRegistry()
  tools.RegisterBuiltinToolsWithRuntimeConfig(registry, tools.WebSearchConfig{}, tools.WebFetchConfig{}, []string{workspace}, 0, tools.BuiltinRuntimeConfig{})
  tools.RegisterPDFTool(registry, &stubPDFService{})
  tool := registry.Get("pdf")
  if tool == nil { panic("missing pdf tool") }
  _, err := tool.Execute(context.Background(), map[string]interface{}{"action":"info","path":"orca_命理完整报告_杂志风.pdf"})
  if err != nil {
    fmt.Println("ERR=" + err.Error())
    return
  }
  fmt.Println("OK")
}
