package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func main() {
	check := flag.Bool("check", false, "verify files match generated catalog instead of writing them")
	flag.Parse()

	content, err := providerpool.GenerateOfficialProviderCatalogJSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate provider catalog: %v\n", err)
		os.Exit(1)
	}

	rootDir, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve repo root: %v\n", err)
		os.Exit(1)
	}

	targets := []string{
		filepath.Join(rootDir, "docs", "provider_catalog.json"),
		filepath.Join(rootDir, "server", "internal", "providerpool", "embedded", "provider_catalog.json"),
	}

	for _, path := range targets {
		if *check {
			if err := verifyFileMatches(path, content); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Printf("verified %s\n", path)
			continue
		}

		if err := os.WriteFile(path, content, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s\n", path)
	}
}

func verifyFileMatches(path string, expected []byte) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("%s does not match generated provider catalog; run `make provider-catalog`", path)
	}
	return nil
}

func repoRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..")), nil
}
