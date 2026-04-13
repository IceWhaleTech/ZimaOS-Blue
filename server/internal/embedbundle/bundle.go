package embedbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"testing/fstest"
)

// MustLoadTarGzFS loads a tar.gz bundle into an in-memory fs.FS and panics on error.
func MustLoadTarGzFS(bundle []byte, label, requiredPrefix string, requiredPaths ...string) fs.FS {
	fsys, err := LoadTarGzFS(bundle, requiredPrefix, requiredPaths...)
	if err != nil {
		panic(fmt.Sprintf("%s: load embedded bundle: %v", strings.TrimSpace(label), err))
	}
	return fsys
}

// LoadTarGzFS loads a tar.gz bundle into an in-memory fs.FS.
func LoadTarGzFS(bundle []byte, requiredPrefix string, requiredPaths ...string) (fs.FS, error) {
	requiredPrefix = normalizePrefix(requiredPrefix)
	if len(bundle) == 0 {
		return nil, fmt.Errorf("empty bundle")
	}

	gz, err := gzip.NewReader(bytes.NewReader(bundle))
	if err != nil {
		return nil, fmt.Errorf("open gzip bundle: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	files := make(fstest.MapFS)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("unsupported tar entry %q type %d", hdr.Name, hdr.Typeflag)
		}

		name := normalizeBundlePath(hdr.Name, requiredPrefix)
		if name == "" {
			return nil, fmt.Errorf("invalid tar entry path %q", hdr.Name)
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		files[name] = &fstest.MapFile{
			Data: data,
			Mode: fs.FileMode(hdr.Mode),
		}
	}

	for _, required := range requiredPaths {
		required = normalizeBundlePath(required, requiredPrefix)
		if required == "" {
			return nil, fmt.Errorf("invalid required path %q", required)
		}
		if _, ok := files[required]; !ok {
			return nil, fmt.Errorf("bundle missing %s", required)
		}
	}

	return files, nil
}

func normalizePrefix(prefix string) string {
	prefix = path.Clean(strings.TrimSpace(prefix))
	if prefix == "." || prefix == "" {
		return ""
	}
	return prefix + "/"
}

func normalizeBundlePath(name, requiredPrefix string) string {
	clean := path.Clean(strings.TrimSpace(name))
	if clean == "." || clean == "" {
		return ""
	}
	if strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	if requiredPrefix != "" && !strings.HasPrefix(clean, requiredPrefix) {
		return ""
	}
	return clean
}
