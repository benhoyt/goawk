package parseutil

import (
	"bytes"
	"io"
)

type FileReader struct {
	files  []file
	source bytes.Buffer
}

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

func (fr *FileReader) Source() []byte {
	return fr.source.Bytes()
}

type file struct {
	path  string
	lines int
}
