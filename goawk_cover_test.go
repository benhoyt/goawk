package main

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestCover(t *testing.T) {
	tests := []struct {
		mode                string
		coverappend         bool
		runs                [][]string
		expectedCoverReport string
	}{
		{"set", true, [][]string{{"a1.awk"}}, "test_set.cov"},
		{"count", true, [][]string{{"a1.awk"}}, "test_count.cov"},
		{"set", true, [][]string{{"a2.awk", "a1.awk"}}, "test_a2a1_set.cov"},
		{"count", true, [][]string{{"a2.awk", "a1.awk"}}, "test_a2a1_count.cov"},
		{"set", true, [][]string{{"a1.awk"}, {"a1.awk"}}, "test_1file2runs_set.cov"},
		{"count", true, [][]string{{"a2.awk", "a1.awk"}, {"a2.awk", "a1.awk"}}, "test_2file2runs_count.cov"},
		{"set", false, [][]string{{"a1.awk"}, {"a1.awk"}}, "test_1file2runs_set_truncated.cov"},
		{"count", false, [][]string{{"a2.awk", "a1.awk"}, {"a2.awk", "a1.awk"}}, "test_2file2runs_count_truncated.cov"},
	}

	coverprofile := "/tmp/testCov.txt"

	for _, test := range tests {
		t.Run(test.expectedCoverReport, func(t *testing.T) {
			// make sure file doesn't exist
			if _, err := os.Stat(coverprofile); os.IsNotExist(err) {
				// file already doesn't exist
			} else if err == nil {
				// file exists
				err := os.Remove(coverprofile)
				if err != nil {
					t.Fatalf("%v", err)
				}
			} else {
				t.Fatalf("%v", err)
			}
			for _, run := range test.runs {
				var args []string
				args = append(args, "goawk")
				for _, file := range run {
					args = append(args, "-f", "testdata/cover/"+file)
				}
				args = append(args, "-coverprofile", coverprofile)
				args = append(args, "-covermode", test.mode)
				if test.coverappend {
					args = append(args, "-coverappend")
				}
				os.Args = args
				status := mainLogic()
				if status != 0 {
					t.Fatalf("expected exit status 0, got: %d", status)
				}
			}

			result, err := ioutil.ReadFile(coverprofile)
			if err != nil {
				t.Fatalf("%v", err)
			}
			resultStr := string(result)
			resultStr = strings.TrimSpace(convertPathsToFilenames(t, resultStr))
			expected, err := ioutil.ReadFile("testdata/cover/" + test.expectedCoverReport)
			if err != nil {
				t.Fatalf("%v", err)
			}
			expectedStr := strings.TrimSpace(string(expected))
			if resultStr != expectedStr {
				t.Fatalf("wrong coverage report, expected:\n%s\n\nactual:\n\n%s", expectedStr, resultStr)
			}
		})
	}
}

func convertPathsToFilenames(t *testing.T, str string) string {
	lines := strings.Split(str, "\n")
	for i, line := range lines {
		if i == 0 {
			continue // skip mode line
		}
		if line == "" {
			continue // skip empty line
		}
		if !strings.HasPrefix(line, "/") {
			t.Fatalf("must be absolute path in coverage report: %s", line)
		}
		parts := strings.Split(line, "/")
		lines[i] = parts[len(parts)-1] // leave only the part with name
	}
	return strings.Join(lines, "\n")
}
