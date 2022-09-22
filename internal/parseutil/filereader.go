package parseutil

import (
	"bytes"
	"io"
)

// FileReader serves two purposes:
// 1. read input sources and join them for single source
// 2. track the lines counts of each input source
type FileReader struct {
	files  []file
	source bytes.Buffer
}

type file struct {
	path  string
	lines int
}

// AddFile adds input source
func (fr *FileReader) AddFile(path string, source io.Reader) error {
	curLen := len(fr.source.Bytes())
	_, err := fr.source.ReadFrom(source)
	if err != nil {
		return err
	}
	if !bytes.HasSuffix(fr.source.Bytes(), []byte("\n")) {
		fr.source.WriteByte('\n')
	}
	content := fr.source.Bytes()[curLen:]
	lines := bytes.Count(content, []byte("\n"))
	fr.files = append(fr.files, file{path, lines})
	return nil
}

// FileLine resolves global line number in "joined" source to a local line number in a file (identified by path)
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

// Source returns "joined" source of all input sources
func (fr *FileReader) Source() []byte {
	return fr.source.Bytes()
}
