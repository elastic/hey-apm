package out

import (
	"bufio"
	"bytes"
	"io/ioutil"
)

type FileWriter struct {
	Filename string
}

func (fw FileWriter) Write(content []byte) (int, error) {
	return len(content), ioutil.WriteFile(fw.Filename, content, 0644)
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
