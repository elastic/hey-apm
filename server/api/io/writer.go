package io

import (
	"bufio"
	"bytes"
)

type FileWriter interface {
	WriteToFile(string, []byte) error
}

type BufferWriter struct {
	b *bytes.Buffer
	w *bufio.Writer
}

func NewBufferWriter() *BufferWriter {
	var b bytes.Buffer
	return &BufferWriter{&b, bufio.NewWriter(&b)}
}

func (bw *BufferWriter) Write(p []byte) (nn int, err error) {
	return bw.w.Write(p)
}

func (bw *BufferWriter) String() string {
	bw.w.Flush()
	return bw.b.String()
}
