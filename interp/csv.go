package interp

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"runtime"
	"unicode/utf8"
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

var errCSVSeparator = errors.New("invalid CSV field separator or comment delimiter")

func validateCSVInputConfig(mode IOMode, config CSVInputConfig) error {
	if mode != CSVMode && mode != TSVMode {
		return nil
	}
	if config.Separator == config.Comment || !validCSVSeparator(config.Separator) ||
		(config.Comment != 0 && !validCSVSeparator(config.Comment)) {
		return errCSVSeparator
	}
	return nil
}

func validateCSVOutputConfig(mode IOMode, config CSVOutputConfig) error {
	if mode != CSVMode && mode != TSVMode {
		return nil
	}
	if !validCSVSeparator(config.Separator) {
		return errCSVSeparator
	}
	return nil
}

func validCSVSeparator(r rune) bool {
	return r != 0 && r != '"' && r != '\r' && r != '\n' && utf8.ValidRune(r) && r != utf8.RuneError
}

// Splitter that splits records in CSV or TSV format.
type csvSplitter struct {
	separator rune
	comment   rune
	noHeader  bool

	inQuote      bool
	resetBuffer  bool
	recordBuffer []byte
	fieldIndexes []int

	fields     *[]string
	fieldNames *[]string
	row        int
}

// Much of this code is taken from the stdlib encoding/csv reader code (which
// is licensed under a compatible BSD-style license).
func (s *csvSplitter) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		// No more data, tell Scanner to stop.
		return 0, nil, nil
	}

	// TODO: explicit args? pull out to method?
	readLine := func() []byte {
		newline := bytes.IndexByte(data, '\n')
		var line []byte
		switch {
		case newline >= 0:
			// Process a single line (including newline).
			line = data[:newline+1]
			data = data[newline+1:]
		case atEOF:
			// If at EOF, we have a final record without a newline.
			line = data
			data = data[len(data):]
		default:
			// Need more data
			return nil
		}

		// For backwards compatibility, drop trailing \r before EOF.
		if len(line) > 0 && atEOF && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
			advance++
		}

		return line
	}

	// Read line (automatically skipping past empty lines and any comments).
	var line []byte
	for {
		line = readLine()
		if len(line) == 0 {
			return advance, nil, nil // Request more data
		}
		if s.comment != 0 && nextRune(line) == s.comment {
			advance += len(line)
			continue // Skip comment lines
		}
		if len(line) == lengthNL(line) {
			advance += len(line)
			continue // Skip empty lines
		}
		break
	}

	// Parse each field in the record.
	const quoteLen = len(`"`)
	sepLen := utf8.RuneLen(s.separator) // TODO: could cache this
	if s.resetBuffer {
		s.recordBuffer = s.recordBuffer[:0]
		s.fieldIndexes = s.fieldIndexes[:0]
	}
parseField:
	for {
		if !s.inQuote && (len(line) == 0 || line[0] != '"') {
			// Non-quoted string field
			i := bytes.IndexRune(line, s.separator)
			field := line
			if i >= 0 {
				field = field[:i]
			} else {
				field = field[:len(field)-lengthNL(field)]
			}
			s.recordBuffer = append(s.recordBuffer, field...)
			s.fieldIndexes = append(s.fieldIndexes, len(s.recordBuffer))
			if i >= 0 {
				line = line[i+sepLen:]
				advance += i + sepLen
				continue parseField
			}
			advance += len(field)
			break parseField
		} else {
			// Quoted string field
			if !s.inQuote {
				line = line[quoteLen:]
				advance += quoteLen
			}
			s.inQuote = true
			for {
				i := bytes.IndexByte(line, '"')
				if i >= 0 {
					// Hit next quote.
					s.recordBuffer = append(s.recordBuffer, line[:i]...)
					line = line[i+quoteLen:]
					advance += i + quoteLen
					switch rn := nextRune(line); {
					case rn == '"':
						// `""` sequence (append quote).
						s.recordBuffer = append(s.recordBuffer, '"')
						line = line[quoteLen:]
						advance += quoteLen
					case rn == s.separator:
						// `",` sequence (end of field).
						line = line[sepLen:]
						s.fieldIndexes = append(s.fieldIndexes, len(s.recordBuffer))
						advance += sepLen
						s.inQuote = false
						continue parseField
					case lengthNL(line) == len(line):
						// `"\n` sequence (end of line).
						s.fieldIndexes = append(s.fieldIndexes, len(s.recordBuffer))
						advance += len(line)
						break parseField
					default:
						// `"` sequence (bare quote).
						s.recordBuffer = append(s.recordBuffer, '"')
					}
				} else if len(line) > 0 {
					// Hit end of line (copy all data so far).
					advance += len(line)
					newlineLen := lengthNL(line)
					if newlineLen == 2 {
						s.recordBuffer = append(s.recordBuffer, line[:len(line)-2]...)
						s.recordBuffer = append(s.recordBuffer, '\n')
					} else {
						s.recordBuffer = append(s.recordBuffer, line...)
					}
					line = readLine()
					if line == nil {
						return advance, nil, nil // Request more data
					}
				} else {
					// Abrupt end of file.
					s.fieldIndexes = append(s.fieldIndexes, len(s.recordBuffer))
					advance += len(line)
					break parseField
				}
			}
		}
	}

	s.inQuote = false
	s.resetBuffer = true

	// Create a single string and create slices out of it.
	// This pins the memory of the fields together, but allocates once.
	str := string(s.recordBuffer) // Convert to string once to batch allocations
	dst := make([]string, len(s.fieldIndexes))
	//TODO: reuse dst like so:
	//dst = dst[:0]
	//if cap(dst) < len(s.fieldIndexes) {
	//	dst = make([]string, len(s.fieldIndexes))
	//}
	//dst = dst[:len(s.fieldIndexes)]
	var preIdx int
	for i, idx := range s.fieldIndexes {
		dst[i] = str[preIdx:idx]
		preIdx = idx
	}

	if s.row == 0 && !s.noHeader {
		// Set header field names and advance, but don't return a line (token).
		s.row++
		*s.fieldNames = dst
		return advance, nil, nil
	}

	// Normal row, set fields and return a line (token).
	s.row++
	*s.fields = dst
	return advance, data[:advance], nil
}

// lengthNL reports the number of bytes for the trailing \n.
func lengthNL(b []byte) int {
	if len(b) > 1 && b[len(b)-2] == '\r' && b[len(b)-1] == '\n' {
		return 2
	}
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return 1
	}
	return 0
}

// nextRune returns the next rune in b or utf8.RuneError.
func nextRune(b []byte) rune {
	r, _ := utf8.DecodeRune(b)
	return r
}
