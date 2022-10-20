package cover

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/benhoyt/goawk/internal/parseutil"
	"github.com/benhoyt/goawk/parser"
)

func TestAnnotatingLogicCorrectness(t *testing.T) {
	tests := []struct {
		awkPath   string
		checkPath string
	}{
		{"a1.awk", "a1_annotation_data.txt"},
		{"a2.awk", "a2_annotation_data.txt"},
		{"a3.awk", "a3_annotation_data.txt"},
	}

	for _, test := range tests {
		t.Run(test.awkPath, func(t *testing.T) {
			awkPath := "../../testdata/cover/" + test.awkPath
			checkPath := "../../testdata/cover/" + test.checkPath
			f, err := os.Open(awkPath)
			if err != nil {
				panic(err)
			}
			fileReader := &parseutil.FileReader{}
			err = fileReader.AddFile(test.awkPath, f)
			if err != nil {
				panic(err)
			}
			coverage := New(ModeSet, false, fileReader)
			prog, err := parser.ParseProgram(fileReader.Source(), nil)
			if err != nil {
				panic(err)
			}
			coverage.Annotate(&prog.ResolvedProgram.Program)

			var actualAnnotationData strings.Builder

			for i, block := range coverage.trackedBlocks {
				actualAnnotationData.WriteString(fmt.Sprintf("%d %s %s-%s %d\n", i+1,
					block.path, block.start, block.end,
					block.numStmts))
			}

			result := strings.TrimSpace(actualAnnotationData.String())

			expected, err := ioutil.ReadFile(checkPath)
			if err != nil {
				panic(err)
			}

			if strings.TrimSpace(string(normalizeNewlines(expected))) != result {
				t.Errorf("Annotation data is wrong:\n\nactual:\n\n%s\n\nexpected:\n\n%s", result, expected)
			}
		})
	}
}

func normalizeNewlines(b []byte) []byte {
	return bytes.Replace(b, []byte("\r\n"), []byte{'\n'}, -1)
}
