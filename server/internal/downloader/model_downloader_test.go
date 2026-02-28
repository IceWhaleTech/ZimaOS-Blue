package downloader

import "testing"

func TestBuildURLList_PriorityOrder(t *testing.T) {
	f := ModelFile{
		Filename: "model.onnx",
		URL:      "https://huggingface.co/a/b/resolve/main/model.onnx",
		Mirrors: []string{
			"https://hf-mirror.com/a/b/resolve/main/model.onnx",
			"https://modelscope.cn/models/a/b/resolve/master/model.onnx",
			"https://example.com/mirror/model.onnx",
		},
	}
	urls := buildURLList(f)
	if len(urls) < 3 {
		t.Fatalf("expected >=3 urls, got %d", len(urls))
	}
	if urls[0] != "https://modelscope.cn/models/a/b/resolve/master/model.onnx" {
		t.Fatalf("first url should be modelscope, got %q", urls[0])
	}
	if urls[1] != "https://huggingface.co/a/b/resolve/main/model.onnx" {
		t.Fatalf("second url should be hf.co primary, got %q", urls[1])
	}
	if urls[2] != "https://hf-mirror.com/a/b/resolve/main/model.onnx" {
		t.Fatalf("third url should be hf-mirror, got %q", urls[2])
	}
}
