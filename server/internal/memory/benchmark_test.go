package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkVectorStoreSearch benchmarks the current in-memory vector search
func BenchmarkVectorStoreSearch(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-vector")
	defer os.RemoveAll(tmpDir)

	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:    filepath.Join(tmpDir, "test.db"),
		EnableFTS: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// Insert test data
	for i := 0; i < 1000; i++ {
		content := fmt.Sprintf("This is test content number %d about AI and machine learning", i)
		store.Store(ctx, content, nil, nil)
	}

	b.ResetTimer()

	// Benchmark keyword search (FTS5)
	b.Run("FTS5_Search_1K", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			store.SearchKeyword(ctx, "machine learning", 10)
		}
	})
}

// BenchmarkMarkdownSearch benchmarks Markdown file search
func BenchmarkMarkdownSearch(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-markdown")
	defer os.RemoveAll(tmpDir)

	// Create test files
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("# Document %d\n\nThis is content about AI and machine learning.\n\n## Section\n\nMore details here.", i)
		os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("doc-%d.md", i)), []byte(content), 0644)
	}

	store := NewMarkdownMemoryStore(tmpDir)
	ctx := context.Background()

	b.ResetTimer()

	b.Run("Markdown_Search_100files", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			store.Search(ctx, "machine learning", 10)
		}
	})
}
