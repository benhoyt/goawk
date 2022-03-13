package interp

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"io"
	"runtime"
	"strings"
)

func (p *interp) writeCSV(output io.Writer, fields []string) error {
	// If output is already a *bufio.Writer (the common case), csv.NewWriter
	// will use it directly. This is not explicitly documented, but
	// csv.NewWriter calls bufio.NewWriter which calls bufio.NewWriterSize
	// with a 4KB buffer, and bufio.NewWriterSize is documented as returning
	// the underlying bufio.Writer if it's passed a large enough one.
	var flush func() error
	_, isBuffered := output.(*bufio.Writer)
	if !isBuffered {
		// Otherwise create a new buffered writer and flush after writing.
		if p.csvOutput == nil {
			p.csvOutput = bufio.NewWriterSize(output, 4096)
		} else {
			p.csvOutput.Reset(output)
		}
		output = p.csvOutput
		flush = p.csvOutput.Flush
	}

	// Given the above, creating a new one of these is cheap.
	writer := csv.NewWriter(output)
	writer.Comma = p.csvOutputConfig.Separator
	writer.UseCRLF = runtime.GOOS == "windows"
	err := writer.Write(fields)
	if err != nil {
		return err
	}
	if flush != nil {
		return flush()
	}
	return nil
}

// Splitter that splits records in CSV or TSV format.
type csvSplitter struct {
	separator rune
	comment   rune
	noHeader  bool

	fields     *[]string
	fieldNames *[]string
}

// TODO: this is a hacked-together PoC right now
func (s csvSplitter) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		// No more data, tell Scanner to stop.
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full sep-terminated record
		fields := strings.Split(string(data[:i]), string([]rune{s.separator})) // TODO: proper CSV parsing
		if !s.noHeader && *s.fieldNames == nil {
			*s.fieldNames = fields
			return i + 1, nil, nil
		} else {
			*s.fields = fields
			return i + 1, data[:i], nil
		}
	}
	// If at EOF, we have a final, non-terminated record; return it
	if atEOF {
		fields := strings.Split(string(data), string([]rune{s.separator})) // TODO: proper CSV parsing
		if !s.noHeader && *s.fieldNames == nil {
			*s.fieldNames = fields
			return len(data), nil, nil
		} else {
			*s.fields = fields
			return len(data), data, nil
		}
	}
	// Request more data
	return 0, nil, nil
}
