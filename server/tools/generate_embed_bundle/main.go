package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var zeroTime = time.Unix(0, 0).UTC()

func main() {
	sourceDir := flag.String("source", "", "source directory to bundle")
	outputPath := flag.String("output", "", "output .tar.gz path")
	prefix := flag.String("prefix", "", "path prefix inside the bundle")
	flag.Parse()

	if strings.TrimSpace(*sourceDir) == "" {
		fatalf("missing -source")
	}
	if strings.TrimSpace(*outputPath) == "" {
		fatalf("missing -output")
	}
	if strings.TrimSpace(*prefix) == "" {
		fatalf("missing -prefix")
	}

	entries, err := collectFiles(*sourceDir)
	if err != nil {
		fatalf("collect files: %v", err)
	}
	if err := writeBundle(*outputPath, *sourceDir, strings.TrimSpace(*prefix), entries); err != nil {
		fatalf("write bundle: %v", err)
	}

	fmt.Printf("wrote %s with %d files\n", *outputPath, len(entries))
}

func collectFiles(sourceDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func writeBundle(outputPath, sourceDir, prefix string, files []string) error {
	tmpPath := outputPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	success := false
	defer func() {
		_ = out.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	gz := gzip.NewWriter(out)
	gz.Name = ""
	gz.Comment = ""
	gz.ModTime = zeroTime

	tw := tar.NewWriter(gz)
	for _, rel := range files {
		fullPath := filepath.Join(sourceDir, rel)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return err
		}
		header := &tar.Header{
			Name:     filepath.ToSlash(filepath.Join(prefix, rel)),
			Mode:     0o644,
			Size:     int64(len(data)),
			ModTime:  zeroTime,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	success = true
	return os.Rename(tmpPath, outputPath)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
