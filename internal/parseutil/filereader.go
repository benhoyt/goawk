// Package parseutil contains various utilities for parsing GoAWK source code.
package parseutil

import (
	"bytes"
	"io"
)

// FileReader serves two purposes:
// 1. read input sources and join them into a single source (slice of bytes)
// 2. track the lines counts of each input source
type FileReader struct {
	files  []file
	source bytes.Buffer
}

type file struct {
	path  string
	lines int
}

// AddFile adds a single source file.
func (fr *FileReader) AddFile(path string, source io.Reader) error {
	curLen := fr.source.Len()
	_, err := fr.source.ReadFrom(source)
	if err != nil {
		return err
	}
	if !bytes.HasSuffix(fr.source.Bytes(), []byte("\n")) {
		// Append newline to file in case it doesn't end with one
		fr.source.WriteByte('\n')
	}
	content := fr.source.Bytes()[curLen:]
	lines := bytes.Count(content, []byte("\n"))
	fr.files = append(fr.files, file{path, lines})
	return nil
}

// FileLine resolves an overall line number from the concatenated source code
// to the local line number in that source file (identified by path).
func (fr *FileReader) FileLine(line int) (path string, fileLine int) {
	startLine := 1
	for _, f := range fr.files {
		if line >= startLine && line < startLine+f.lines {
			return f.path, line - startLine + 1
		}
		startLine += f.lines
	}
	return "", 0
}

// Source returns the concatenated source code from all files added.
func (fr *FileReader) Source() []byte {
	return fr.source.Bytes()
}
