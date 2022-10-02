package main

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func TestCover(t *testing.T) {
	if runtime.GOOS == "windows" {
		// we use *nix-specific tools for this test
		return
	}

	tests := []struct {
		name                string
		mode                string
		runs                [][]string
		expectedCoverReport string
	}{
		{"1File,set", "set", [][]string{{"a1.awk"}}, "test_set.cov"},
		{"1File,count", "count", [][]string{{"a1.awk"}}, "test_count.cov"},
		{"2Files,set", "set", [][]string{{"a2.awk", "a1.awk"}}, "test_a2a1_set.cov"},
		{"2Files,count", "count", [][]string{{"a2.awk", "a1.awk"}}, "test_a2a1_count.cov"},
		{"1File2Runs,set", "set", [][]string{{"a1.awk"}, {"a1.awk"}}, "test_1file2runs_set.cov"},
		{"2Files2Runs,set", "count", [][]string{{"a2.awk", "a1.awk"}, {"a2.awk", "a1.awk"}}, "test_2file2runs_count.cov"},
	}

	coverprofile := "/tmp/testCov.txt"
	coverprofileFixed := "/tmp/testCov_fixed.txt"

	for _, test := range tests {
		t.Run("TestCover"+test.name, func(t *testing.T) {

			// make sure file doesn't exist
			if _, err := os.Stat(coverprofile); os.IsNotExist(err) {

			} else if err == nil {
				// file exists
				err := os.Remove(coverprofile)
				if err != nil {
					panic(err)
				}
			} else {
				panic(err)
			}
			for _, run := range test.runs {
				var args []string
				args = append(args, "goawk")
				for _, file := range run {
					args = append(args, "-f", "testdata/cover/"+file)
				}
				args = append(args, "-coverprofile", coverprofile)
				args = append(args, "-covermode", test.mode)
				os.Args = args
				status := mainLogic()
				if status != 0 {
					t.Fatalf("expected exit status 0, got: %d", status)
				}
			}

			{
				err := exec.Command("awk", "-v", "OUT="+coverprofileFixed,
					"-f", "testdata/cover/_fixForCompareWithExpected.awk", coverprofile).Run()
				if err != nil {
					panic(err)
				}
			}
			{
				expected := "testdata/cover/" + test.expectedCoverReport
				diff, err := exec.Command("diff", coverprofileFixed, expected).CombinedOutput()
				if err != nil {
					t.Fatalf("Coverage (%s) differs from expected (%s):\n%s\n", coverprofile, expected, string(diff))
				}
			}
		})
	}
}
