package web

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestReadAppendedLayoutUnsigned(t *testing.T) {
	t.Parallel()

	payload := buildTestTarGz(t, "index.html", "hello")
	file, wantOffset := buildUnsignedAppendedFile([]byte("linux-binary"), payload)

	layout, ok := readAppendedLayoutFromReader(bytes.NewReader(file), int64(len(file)))
	if !ok {
		t.Fatal("expected appended layout")
	}
	if layout.offset != wantOffset {
		t.Fatalf("offset = %d, want %d", layout.offset, wantOffset)
	}
	if layout.dataEnd != int64(len(file)-8) {
		t.Fatalf("dataEnd = %d, want %d", layout.dataEnd, len(file)-8)
	}
}

func TestReadAppendedLayoutSignedPEUsesSecurityDirectoryBoundary(t *testing.T) {
	t.Parallel()

	payload := buildTestTarGz(t, "index.html", "signed")
	file, wantOffset, certOffset := buildSignedPEAppendedFile(t, payload, 32)

	if got, ok := readPESecurityDirectoryOffset(bytes.NewReader(file), int64(len(file))); !ok || got != certOffset {
		t.Fatalf("security directory offset = %d, %v; want %d, true", got, ok, certOffset)
	}

	layout, ok := readAppendedLayoutFromReader(bytes.NewReader(file), int64(len(file)))
	if !ok {
		t.Fatal("expected appended layout for signed PE")
	}
	if layout.offset != wantOffset {
		t.Fatalf("offset = %d, want %d", layout.offset, wantOffset)
	}
	if layout.dataEnd != certOffset-8 {
		t.Fatalf("dataEnd = %d, want %d", layout.dataEnd, certOffset-8)
	}
}

func TestReadAppendedLayoutSignedPEWithoutPayloadReturnsFalse(t *testing.T) {
	t.Parallel()

	file := buildSignedPEWithoutPayload(t, 32)
	if _, ok := readAppendedLayoutFromReader(bytes.NewReader(file), int64(len(file))); ok {
		t.Fatal("expected no appended layout")
	}
}

func TestExtractTarGzFromSignedLayoutSection(t *testing.T) {
	t.Parallel()

	payload := buildTestTarGz(t, "nested/index.html", "ok")
	file, _, _ := buildSignedPEAppendedFile(t, payload, 48)

	layout, ok := readAppendedLayoutFromReader(bytes.NewReader(file), int64(len(file)))
	if !ok {
		t.Fatal("expected appended layout for signed PE")
	}

	dst := t.TempDir()
	section := io.NewSectionReader(bytes.NewReader(file), layout.offset, layout.dataEnd-layout.offset)
	if err := extractTarGzFromReader(section, dst); err != nil {
		t.Fatalf("extractTarGzFromReader: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dst, "nested", "index.html"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(content) != "ok" {
		t.Fatalf("content = %q, want %q", string(content), "ok")
	}
}

func buildTestTarGz(t *testing.T, name, content string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatalf("write tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}

func buildUnsignedAppendedFile(prefix, payload []byte) ([]byte, int64) {
	file := append([]byte{}, prefix...)
	offset := int64(len(file))
	file = append(file, payload...)
	var trailer [8]byte
	binary.LittleEndian.PutUint64(trailer[:], uint64(offset))
	file = append(file, trailer[:]...)
	return file, offset
}

func buildSignedPEAppendedFile(t *testing.T, payload []byte, certSize int) ([]byte, int64, int64) {
	t.Helper()

	const (
		peHeaderOffset              = 0x80
		optionalHeaderOffset        = peHeaderOffset + 4 + peFileHeaderSize
		optionalHeaderSize   uint16 = 0x00f0
		headerSize                  = 0x200
	)

	file := make([]byte, headerSize)
	file[0] = 'M'
	file[1] = 'Z'
	binary.LittleEndian.PutUint32(file[peDOSHeaderOffset:peDOSHeaderOffset+4], peHeaderOffset)
	copy(file[peHeaderOffset:], []byte{'P', 'E', 0, 0})
	binary.LittleEndian.PutUint16(file[peHeaderOffset+4+16:peHeaderOffset+4+18], optionalHeaderSize)
	binary.LittleEndian.PutUint16(file[optionalHeaderOffset:optionalHeaderOffset+2], peOptionalMagicPE32Plus)
	binary.LittleEndian.PutUint32(file[optionalHeaderOffset+108:optionalHeaderOffset+112], 16)

	offset := int64(len(file))
	file = append(file, payload...)
	var trailer [8]byte
	binary.LittleEndian.PutUint64(trailer[:], uint64(offset))
	file = append(file, trailer[:]...)

	certOffset := int64(len(file))
	securityDirOffset := optionalHeaderOffset + 112 + peSecurityDirectoryIdx*8
	binary.LittleEndian.PutUint32(file[securityDirOffset:securityDirOffset+4], uint32(certOffset))
	binary.LittleEndian.PutUint32(file[securityDirOffset+4:securityDirOffset+8], uint32(certSize))

	file = append(file, bytes.Repeat([]byte{0xa5}, certSize)...)
	return file, offset, certOffset
}

func buildSignedPEWithoutPayload(t *testing.T, certSize int) []byte {
	t.Helper()

	file, _, _ := buildSignedPEAppendedFile(t, nil, certSize)
	return file
}
