package pdf

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

func TestDirectPDFiumGoogleCXXGuideFixtureShowsMalformedRawText(t *testing.T) {
	wasmBytes, err := os.ReadFile(filepath.Join("testdata", "pdfium.wasm"))
	if err != nil {
		t.Fatalf("read pdfium wasm: %v", err)
	}

	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:      1,
		MaxIdle:      1,
		MaxTotal:     1,
		ReuseWorkers: true,
		Stdout:       io.Discard,
		Stderr:       io.Discard,
		WASM:         wasmBytes,
	})
	if err != nil {
		t.Fatalf("init pdfium webassembly: %v", err)
	}
	defer func() {
		_ = pool.Close()
	}()

	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		t.Fatalf("get pdfium instance: %v", err)
	}
	defer instance.Close()

	path, err := filepath.Abs(filepath.Join("testdata", "Google_C++_Guide.pdf"))
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	doc, err := instance.OpenDocument(&requests.OpenDocument{FilePath: &path})
	if err != nil {
		t.Fatalf("open fixture pdf: %v", err)
	}
	defer func() {
		_, _ = instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	}()

	pageText, err := instance.GetPageText(&requests.GetPageText{
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 1}},
	})
	if err != nil {
		t.Fatalf("extract page text: %v", err)
	}
	if pageText == nil {
		t.Fatal("expected page text response")
	}

	if !strings.Contains(pageText.Text, "智能挃针和其他 C++特性") {
		t.Fatalf("page text = %q, want malformed raw pdfium phrase", pageText.Text)
	}
	if !strings.Contains(pageText.Text, "觃则乊例外") {
		t.Fatalf("page text = %q, want malformed raw pdfium section title", pageText.Text)
	}

	bodyPageText, err := instance.GetPageText(&requests.GetPageText{
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 19}},
	})
	if err != nil {
		t.Fatalf("extract body page text: %v", err)
	}
	if bodyPageText == nil {
		t.Fatal("expected body page text response")
	}
	if !strings.Contains(bodyPageText.Text, "智能指针和其他 C++特性") {
		t.Fatalf("body page text = %q, want correct section heading on later page", bodyPageText.Text)
	}
	if !strings.Contains(bodyPageText.Text, "如果确实需要使用智能挃针的话") {
		t.Fatalf("body page text = %q, want malformed body phrase showing mixed unicode mapping", bodyPageText.Text)
	}
}
