package web

import (
	"encoding/binary"
	"io"
	"os"
)

const (
	peDOSHeaderOffset       = 0x3c
	peSignatureOffset       = 4
	peFileHeaderSize        = 20
	peOptionalMagicPE32     = 0x10b
	peOptionalMagicPE32Plus = 0x20b
	peSecurityDirectoryIdx  = 4
	gzipMagicID1            = 0x1f
	gzipMagicID2            = 0x8b
)

type appendedLayout struct {
	offset  int64
	dataEnd int64
}

func readAppendedLayout(exe string) (appendedLayout, bool) {
	f, err := os.Open(exe)
	if err != nil {
		return appendedLayout{}, false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return appendedLayout{}, false
	}

	return readAppendedLayoutFromReader(f, fi.Size())
}

func readAppendedLayoutFromReader(r io.ReaderAt, size int64) (appendedLayout, bool) {
	logicalEOF := size
	if certOffset, ok := readPESecurityDirectoryOffset(r, size); ok {
		logicalEOF = certOffset
	}

	if logicalEOF < 18 {
		return appendedLayout{}, false
	}

	trailerPos := logicalEOF - 8
	var trailer [8]byte
	if _, err := r.ReadAt(trailer[:], trailerPos); err != nil {
		return appendedLayout{}, false
	}

	offset := int64(binary.LittleEndian.Uint64(trailer[:]))
	if offset <= 0 || offset >= trailerPos || trailerPos-offset < 10 {
		return appendedLayout{}, false
	}

	var magic [2]byte
	if _, err := r.ReadAt(magic[:], offset); err != nil {
		return appendedLayout{}, false
	}
	if magic[0] != gzipMagicID1 || magic[1] != gzipMagicID2 {
		return appendedLayout{}, false
	}

	return appendedLayout{offset: offset, dataEnd: trailerPos}, true
}

func readPESecurityDirectoryOffset(r io.ReaderAt, size int64) (int64, bool) {
	if size < peDOSHeaderOffset+4 {
		return 0, false
	}

	var dos [64]byte
	if _, err := r.ReadAt(dos[:], 0); err != nil {
		return 0, false
	}
	if dos[0] != 'M' || dos[1] != 'Z' {
		return 0, false
	}

	peHeaderOffset := int64(binary.LittleEndian.Uint32(dos[peDOSHeaderOffset : peDOSHeaderOffset+4]))
	if peHeaderOffset <= 0 || peHeaderOffset+peSignatureOffset+peFileHeaderSize+2 > size {
		return 0, false
	}

	var peSignature [4]byte
	if _, err := r.ReadAt(peSignature[:], peHeaderOffset); err != nil {
		return 0, false
	}
	if peSignature != [4]byte{'P', 'E', 0, 0} {
		return 0, false
	}

	var fileHeader [peFileHeaderSize]byte
	if _, err := r.ReadAt(fileHeader[:], peHeaderOffset+peSignatureOffset); err != nil {
		return 0, false
	}

	optionalHeaderOffset := peHeaderOffset + peSignatureOffset + peFileHeaderSize
	optionalHeaderSize := int64(binary.LittleEndian.Uint16(fileHeader[16:18]))
	if optionalHeaderSize < 2 || optionalHeaderOffset+optionalHeaderSize > size {
		return 0, false
	}

	var optionalMagic [2]byte
	if _, err := r.ReadAt(optionalMagic[:], optionalHeaderOffset); err != nil {
		return 0, false
	}

	var dataDirectoryBase int64
	var numberOfRvaAndSizesOffset int64
	switch binary.LittleEndian.Uint16(optionalMagic[:]) {
	case peOptionalMagicPE32:
		numberOfRvaAndSizesOffset = 92
		dataDirectoryBase = 96
	case peOptionalMagicPE32Plus:
		numberOfRvaAndSizesOffset = 108
		dataDirectoryBase = 112
	default:
		return 0, false
	}

	securityDirectoryOffset := dataDirectoryBase + peSecurityDirectoryIdx*8
	if optionalHeaderSize < securityDirectoryOffset+8 {
		return 0, false
	}

	var numberOfRvaAndSizes [4]byte
	if _, err := r.ReadAt(numberOfRvaAndSizes[:], optionalHeaderOffset+numberOfRvaAndSizesOffset); err != nil {
		return 0, false
	}
	if binary.LittleEndian.Uint32(numberOfRvaAndSizes[:]) <= peSecurityDirectoryIdx {
		return 0, false
	}

	var securityDirectory [8]byte
	if _, err := r.ReadAt(securityDirectory[:], optionalHeaderOffset+securityDirectoryOffset); err != nil {
		return 0, false
	}

	certOffset := int64(binary.LittleEndian.Uint32(securityDirectory[0:4]))
	certSize := int64(binary.LittleEndian.Uint32(securityDirectory[4:8]))
	if certOffset <= 0 || certSize <= 0 || certOffset >= size || certOffset+certSize > size {
		return 0, false
	}

	return certOffset, true
}
