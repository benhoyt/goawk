package interp

import (
	"bufio"
	"encoding/csv"
	"io"
	"runtime"
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
		// TODO: could attach to interp and reuse this with bw.Reset
		bw := bufio.NewWriterSize(output, 4096)
		output = bw
		flush = bw.Flush
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
