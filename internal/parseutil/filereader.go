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
	path      string
	startLine int // the number of first line of this file in joined source
	lines     int
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
	startLine := 1
	if len(fr.files) > 0 {
		lastFile := fr.files[len(fr.files)-1]
		startLine = lastFile.startLine + lastFile.lines
	}
	fr.files = append(fr.files, file{path, startLine, lines})
	return nil
}

// FileLine resolves global line number in joined source to a local line number in a file (identified by path)
func (fr *FileReader) FileLine(line int) (path string, fileLine int) {
	for _, f := range fr.files {
		if line >= f.startLine && line < f.startLine+f.lines {
			return f.path, line - f.startLine + 1
		}
	}
	return "", 0
}

// Source returns joined source of all input sources
func (fr *FileReader) Source() []byte {
	return fr.source.Bytes()
}
