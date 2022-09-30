package parseutil_test

import (
	. "github.com/benhoyt/goawk/internal/parseutil"
	"strings"
	"testing"
)

type testFile struct{ name, source string }

type test struct {
	name string
	// input:
	files []testFile
	line  int
	// expected:
	path     string
	fileLine int
}

func TestFileReader(t *testing.T) {
	fileSet1 := []testFile{
		{"file1", `BEGIN {
print f(1)
}`},
		{"file2", `function f(x) {
print x
}`},
	}
	tests := []test{
		{
			"TestInFirstFile",
			fileSet1,
			2,
			"file1",
			2,
		},
		{
			"TestInSecondFile",
			fileSet1,
			5,
			"file2",
			2,
		},
		{
			"TestOutside",
			fileSet1,
			100,
			"",
			0,
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {

			fr := &FileReader{}

			for _, file := range tst.files {
				if nil != fr.AddFile(file.name, strings.NewReader(file.source)) {
					panic("should not happen")
				}
			}

			{
				path, fileLine := fr.FileLine(tst.line)
				if path != tst.path || fileLine != tst.fileLine {
					t.Errorf("wrong path/line")
				}
			}
		})
	}
}
