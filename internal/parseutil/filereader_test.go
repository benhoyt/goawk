package parseutil

import (
	"strings"
	"testing"
)

func addFile(fr *FileReader, fileName string, code string) {
	if nil != fr.AddFile(fileName, strings.NewReader(code)) {
		panic("should not happen")
	}
}

func TestLineResolution(t *testing.T) {
	fr := &FileReader{}

	file1 := "file1"
	file2 := "file2"

	addFile(fr, file1, `BEGIN {
print f(1)
}`)
	addFile(fr, file2, `function f(x) {
print x
}`)
	if len(fr.files) != 2 {
		t.Errorf("must be 2 files")
	}

	{
		path, l := fr.FileLine(2)
		if path != file1 || l != 2 {
			t.Errorf("wrong file/path")
		}
	}
	{
		path, l := fr.FileLine(5)
		if path != file2 || l != 2 {
			t.Errorf("wrong file/path")
		}
	}
	{
		path, l := fr.FileLine(100)
		if path != "" || l != 0 {
			t.Errorf("wrong file/path")
		}
	}
}
