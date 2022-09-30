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
	fileSetNoNewline := []testFile{
		{"file1", `BEGIN {
print f(1)
}`},
		{"file2", `function f(x) {
print x
}`},
	}
	fileSetWithNewline := []testFile{
		{"file1", `BEGIN {
print f(1)
}
`},
		{"file2", `function f(x) {
print x
}
`},
	}
	tests := []test{
		{
			"TestInFirstFile",
			fileSetNoNewline,
			2,
			"file1",
			2,
		},
		{
			"TestInSecondFile",
			fileSetNoNewline,
			5,
			"file2",
			2,
		},
		{
			"TestInFirstFileWithNewline",
			fileSetWithNewline,
			2,
			"file1",
			2,
		},
		{
			"TestInSecondFileWithNewline",
			fileSetWithNewline,
			5,
			"file2",
			2,
		},
		{
			"TestOutside",
			fileSetNoNewline,
			100,
			"",
			0,
		},
		{
			"TestOutsideNegative",
			fileSetNoNewline,
			-100,
			"",
			0,
		},
		{
			"TestNoFiles",
			[]testFile{},
			1,
			"",
			0,
		},
		{
			"TestZeroLenFiles",
			[]testFile{
				{"file1", ""},
				{"file2", ""},
			},
			1,
			"file1",
			1,
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

			path, fileLine := fr.FileLine(tst.line)
			if path != tst.path {
				t.Errorf("expected path: %v, got: %v", tst.path, path)
			}
			if fileLine != tst.fileLine {
				t.Errorf("expected fileLine: %v, got: %v", tst.fileLine, fileLine)
			}
		})
	}
}
