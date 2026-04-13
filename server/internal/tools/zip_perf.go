package tools

import (
	"archive/zip"
	"compress/flate"
	"io"
	"sync"
)

const fastZipCompressionLevel = flate.BestSpeed

var fastZipCompressorPool = sync.Pool{
	New: func() interface{} {
		writer, _ := flate.NewWriter(io.Discard, fastZipCompressionLevel)
		return writer
	},
}

type pooledFastZipWriter struct {
	*flate.Writer
}

func (w *pooledFastZipWriter) Close() error {
	err := w.Writer.Close()
	fastZipCompressorPool.Put(w.Writer)
	return err
}

func newFastZipWriter(out io.Writer) *zip.Writer {
	writer := zip.NewWriter(out)
	writer.RegisterCompressor(zip.Deflate, func(dst io.Writer) (io.WriteCloser, error) {
		flateWriter := fastZipCompressorPool.Get().(*flate.Writer)
		flateWriter.Reset(dst)
		return &pooledFastZipWriter{Writer: flateWriter}, nil
	})
	return writer
}
