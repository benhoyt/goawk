package cover

import (
	"fmt"
	"github.com/benhoyt/goawk/internal/parseutil"
	"github.com/benhoyt/goawk/parser"
	"io/ioutil"
	"os"
	"strings"
	"testing"
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
			coverage := New("set", false, fileReader)
			prog, err := parser.ParseProgram(fileReader.Source(), nil)
			if err != nil {
				panic(err)
			}
			coverage.Annotate(&prog.ResolvedProgram.Program)

			var actualAnnotationData strings.Builder

			for i := 1; i <= coverage.annotationIdx; i++ {
				boundary := coverage.boundaries[i]
				actualAnnotationData.WriteString(fmt.Sprintf("%d %s %s-%s %d\n", i,
					boundary.fileName, boundary.start, boundary.end,
					coverage.stmtsCnt[i]))
			}

			result := strings.TrimSpace(actualAnnotationData.String())

			expected, err := ioutil.ReadFile(checkPath)
			if err != nil {
				panic(err)
			}

			if strings.TrimSpace(string(expected)) != result {
				t.Fatalf("Annotation data is wrong:\n%s", result)
			}
		})
	}
}
