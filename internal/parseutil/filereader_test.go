package parseutil_test

import (
	. "github.com/benhoyt/goawk/internal/parseutil"
	"strings"
	"testing"
)

func TestLineResolution(t *testing.T) {
	fr := &FileReader{}

	file1 := "file1"
	file2 := "file2"

	addFile := func(fileName string, code string) {
		if nil != fr.AddFile(fileName, strings.NewReader(code)) {
			panic("should not happen")
		}
	}

	addFile(file1, `BEGIN {
print f(1)
}`)
	addFile(file2, `function f(x) {
print x
}`)
	{
		path, line := fr.FileLine(2)
		if path != file1 || line != 2 {
			t.Errorf("wrong path/line")
		}
	}
	{
		path, line := fr.FileLine(5)
		if path != file2 || line != 2 {
			t.Errorf("wrong path/line")
		}
	}
	{
		path, line := fr.FileLine(100)
		if path != "" || line != 0 {
			t.Errorf("wrong path/line")
		}
	}
}
