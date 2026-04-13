package tools

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

func TestOfficeBuildZipPreservesEntryContent(t *testing.T) {
	entries := []officeZipEntry{
		{Name: "doc/alpha.xml", Content: `<root>Alice &amp; Bob</root>`},
		{Name: "", Content: "skip me"},
		{Name: "doc/unicode.txt", Content: "中文\nLine 2\nemoji-like ascii :-)"},
		{
			Name:     "doc/streamed.xml",
			SizeHint: len(`<streamed>from writer path</streamed>`),
			WriteTo: func(w io.Writer) error {
				_, err := officeWriteStringNoCopy(w, `<streamed>from writer path</streamed>`)
				return err
			},
		},
	}

	data, err := officeBuildZip(entries)
	if err != nil {
		t.Fatalf("officeBuildZip() error = %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}

	got := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		got[file.Name] = string(body)
	}

	if len(got) != 3 {
		t.Fatalf("zip entry count = %d, want 3", len(got))
	}
	if got["doc/alpha.xml"] != `<root>Alice &amp; Bob</root>` {
		t.Fatalf("alpha.xml = %q", got["doc/alpha.xml"])
	}
	if got["doc/unicode.txt"] != "中文\nLine 2\nemoji-like ascii :-)" {
		t.Fatalf("unicode.txt = %q", got["doc/unicode.txt"])
	}
	if got["doc/streamed.xml"] != `<streamed>from writer path</streamed>` {
		t.Fatalf("streamed.xml = %q", got["doc/streamed.xml"])
	}
}
