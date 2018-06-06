package tests

import (
	"strings"

	"github.com/elastic/hey-apm/server/api/io"
)

func WithoutColors(s string) string {
	for _, c := range []string{io.Magenta, io.Cyan, io.Red, io.Green, io.Yellow, io.Grey} {
		s = strings.Replace(s, c, "", -1)
	}
	return s
}

type MockFileWriter struct {
	name string
	Data string
}

func (mfw *MockFileWriter) WriteToFile(name string, data []byte) error {
	mfw.name = name
	mfw.Data = string(data)
	return nil
}

func (mfw MockFileWriter) HasBeenWritenTo() bool {
	return mfw.name != ""
}
