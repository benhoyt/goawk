// Input/output handling for GoAWK interpreter

package interp

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/benhoyt/goawk/internal/resolver"
	. "github.com/benhoyt/goawk/lexer"
)

// Print a line of output followed by a newline
func (p *interp) printLine(writer io.Writer, line string) error {
	err := writeOutput(writer, line)
	if err != nil {
		return err
	}
	return writeOutput(writer, p.outputRecordSep)
}

// Print given arguments followed by a newline (for "print" statement).
func (p *interp) printArgs(writer io.Writer, args []value) error {
	switch p.outputMode {
	case CSVMode, TSVMode:
		fields := make([]string, 0, 7) // up to 7 args won't require a heap allocation
		for _, arg := range args {
			fields = append(fields, arg.str(p.outputFormat))
		}
		err := p.writeCSV(writer, fields)
		if err != nil {
			return err
		}
	default:
		// Print OFS-separated args followed by ORS (usually newline).
		for i, arg := range args {
			if i > 0 {
				err := writeOutput(writer, p.outputFieldSep)
				if err != nil {
					return err
				}
			}
			err := writeOutput(writer, arg.str(p.outputFormat))
			if err != nil {
				return err
			}
		}
		err := writeOutput(writer, p.outputRecordSep)
		if err != nil {
			return err
		}
	}
	return nil
}

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

// Determine the output stream for given redirect token and
// destination (file or pipe name)
func (p *interp) getOutputStream(redirect Token, destValue value) (io.Writer, error) {
	name := p.toString(destValue)
	if _, ok := p.inputStreams[name]; ok {
		return nil, newError("can't write to reader stream")
	}
	if w, ok := p.outputStreams[name]; ok {
		return w, nil
	}

	switch redirect {
	case GREATER, APPEND:
		if name == "-" {
			// filename of "-" means write to stdout, eg: print "x" >"-"
			return p.output, nil
		}
		// Write or append to file
		if p.noFileWrites {
			return nil, newError("can't write to file due to NoFileWrites")
		}
		p.flushOutputAndError() // ensure synchronization
		flags := os.O_CREATE | os.O_WRONLY
		if redirect == GREATER {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_APPEND
		}
		f, err := os.OpenFile(name, flags, 0644)
		if err != nil {
			return nil, newError("output redirection error: %s", err)
		}
		out := newOutFileStream(f, outputBufSize)
		p.outputStreams[name] = out
		return out, nil

	case PIPE:
		// Pipe to command
		if p.noExec {
			return nil, newError("can't write to pipe due to NoExec")
		}
		cmd := p.execShell(name)
		cmd.Stdout = p.output
		cmd.Stderr = p.errorOutput
		p.flushOutputAndError() // ensure synchronization
		out, err := newOutCmdStream(cmd)
		if err != nil {
			p.printErrorf("%s\n", err)
			out = newOutNullStream()
		}
		p.outputStreams[name] = out
		return out, nil

	default:
		// Should never happen
		panic(fmt.Sprintf("unexpected redirect type %s", redirect))
	}
}

// Executes code using configured system shell
func (p *interp) execShell(code string) *exec.Cmd {
	executable := p.shellCommand[0]
	args := p.shellCommand[1:]
	args = append(args, code)
	if p.checkCtx {
		return exec.CommandContext(p.ctx, executable, args...)
	} else {
		return exec.Command(executable, args...)
	}
}

// Get input Scanner to use for "getline" based on file name
func (p *interp) getInputScannerFile(name string) (*bufio.Scanner, error) {
	if _, ok := p.outputStreams[name]; ok {
		return nil, newError("can't read from writer stream")
	}
	if _, ok := p.inputStreams[name]; ok {
		return p.scanners[name], nil
	}
	if name == "-" {
		// filename of "-" means read from stdin, eg: getline <"-"
		if scanner, ok := p.scanners["-"]; ok {
			return scanner, nil
		}
		scanner := p.newScanner(p.stdin, make([]byte, inputBufSize))
		p.scanners[name] = scanner
		return scanner, nil
	}
	if p.noFileReads {
		return nil, newError("can't read from file due to NoFileReads")
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err // *os.PathError is handled by caller (getline returns -1)
	}
	in := newInFileStream(f)
	scanner := p.newScanner(in, make([]byte, inputBufSize))
	p.scanners[name] = scanner
	p.inputStreams[name] = in
	return scanner, nil
}

// Get input Scanner to use for "getline" based on pipe name
func (p *interp) getInputScannerPipe(name string) (*bufio.Scanner, error) {
	if _, ok := p.outputStreams[name]; ok {
		return nil, newError("can't read from writer stream")
	}
	if _, ok := p.inputStreams[name]; ok {
		return p.scanners[name], nil
	}
	if p.noExec {
		return nil, newError("can't read from pipe due to NoExec")
	}
	cmd := p.execShell(name)
	cmd.Stdin = p.stdin
	cmd.Stderr = p.errorOutput
	p.flushOutputAndError() // ensure synchronization
	in, err := newInCmdStream(cmd)
	if err != nil {
		p.printErrorf("%s\n", err)
		return bufio.NewScanner(strings.NewReader("")), nil
	}

	scanner := p.newScanner(in, make([]byte, inputBufSize))
	p.inputStreams[name] = in
	p.scanners[name] = scanner
	return scanner, nil
}

// Create a new buffered Scanner for reading input records
func (p *interp) newScanner(input io.Reader, buffer []byte) *bufio.Scanner {
	scanner := bufio.NewScanner(input)
	switch {
	case p.inputMode == CSVMode || p.inputMode == TSVMode:
		splitter := csvSplitter{
			separator:     p.csvInputConfig.Separator,
			sepLen:        utf8.RuneLen(p.csvInputConfig.Separator),
			comment:       p.csvInputConfig.Comment,
			header:        p.csvInputConfig.Header,
			fields:        &p.fields,
			setFieldNames: p.setFieldNames,
		}
		scanner.Split(splitter.scan)
	case p.recordSep == "\n":
		// Scanner default is to split on newlines
	case p.recordSep == "":
		// Empty string for RS means split on \n\n (blank lines)
		splitter := blankLineSplitter{terminator: &p.recordTerminator}
		scanner.Split(splitter.scan)
	case len(p.recordSep) == 1:
		splitter := byteSplitter{sep: p.recordSep[0]}
		scanner.Split(splitter.scan)
	case utf8.RuneCountInString(p.recordSep) >= 1:
		// Multi-byte and single char but multi-byte RS use regex
		splitter := regexSplitter{re: p.recordSepRegex, terminator: &p.recordTerminator}
		scanner.Split(splitter.scan)
	}
	scanner.Buffer(buffer, maxRecordLength)
	return scanner
}

// setFieldNames is called by csvSplitter.scan on the first row (if the
// "header" option is specified).
func (p *interp) setFieldNames(names []string) {
	p.fieldNames = names
	p.fieldIndexes = nil // clear name-to-index cache

	// Populate FIELDS array (mapping of field indexes to field names).
	fieldsArray := p.array(resolver.Global, p.arrayIndexes["FIELDS"])
	for k := range fieldsArray {
		delete(fieldsArray, k)
	}
	for i, name := range names {
		fieldsArray[strconv.Itoa(i+1)] = str(name)
	}
}

// Copied from bufio/scan.go in the stdlib: I guess it's a bit more
// efficient than bytes.TrimSuffix(data, []byte("\r"))
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[:len(data)-1]
	}
	return data
}

func dropLF(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\n' {
		return data[:len(data)-1]
	}
	return data
}

type blankLineSplitter struct {
	terminator *string
}

func (s blankLineSplitter) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Skip newlines at beginning of data
	i := 0
	for i < len(data) && (data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) {
		// At end of data after newlines, skip entire data block
		return i, nil, nil
	}
	start := i

	// Try to find two consecutive newlines (or \n\r\n for Windows)
	for ; i < len(data); i++ {
		if data[i] != '\n' {
			continue
		}
		end := i
		if i+1 < len(data) && data[i+1] == '\n' {
			i += 2
			for i < len(data) && (data[i] == '\n' || data[i] == '\r') {
				i++ // Skip newlines at end of record
			}
			*s.terminator = string(data[end:i])
			return i, dropCR(data[start:end]), nil
		}
		if i+2 < len(data) && data[i+1] == '\r' && data[i+2] == '\n' {
			i += 3
			for i < len(data) && (data[i] == '\n' || data[i] == '\r') {
				i++ // Skip newlines at end of record
			}
			*s.terminator = string(data[end:i])
			return i, dropCR(data[start:end]), nil
		}
	}

	// If we're at EOF, we have one final record; return it
	if atEOF {
		token = dropCR(dropLF(data[start:]))
		*s.terminator = string(data[len(token):])
		return len(data), token, nil
	}

	// Request more data
	return 0, nil, nil
}

// Splitter that splits records on the given separator byte
type byteSplitter struct {
	sep byte
}

func (s byteSplitter) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, s.sep); i >= 0 {
		// We have a full sep-terminated record
		return i + 1, data[:i], nil
	}
	// If at EOF, we have a final, non-terminated record; return it
	if atEOF {
		return len(data), data, nil
	}
	// Request more data
	return 0, nil, nil
}

// Splitter that splits records on the given regular expression
type regexSplitter struct {
	re         *regexp.Regexp
	terminator *string
}

func (s regexSplitter) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	loc := s.re.FindIndex(data)
	// Note: for a regex such as "()", loc[0]==loc[1]. Gawk behavior for this
	// case is to match the entire input.
	if loc != nil && loc[0] != loc[1] {
		*s.terminator = string(data[loc[0]:loc[1]]) // set RT special variable
		return loc[1], data[:loc[0]], nil
	}
	// If at EOF, we have a final, non-terminated record; return it
	if atEOF {
		*s.terminator = ""
		return len(data), data, nil
	}
	// Request more data
	return 0, nil, nil
}

// Splitter that splits records in CSV or TSV format.
type csvSplitter struct {
	separator rune
	sepLen    int
	comment   rune
	header    bool

	recordBuffer []byte
	fieldIndexes []int
	noBOMCheck   bool

	fields        *[]string
	setFieldNames func(names []string)
	rowNum        int
}

// The structure of this code is taken from the stdlib encoding/csv Reader
// code, which is licensed under a compatible BSD-style license.
//
// We don't support all encoding/csv features: FieldsPerRecord is not
// supported, LazyQuotes is always on, and TrimLeadingSpace is always off.
func (s *csvSplitter) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// Some CSV files are saved with a UTF-8 BOM at the start; skip it.
	if !s.noBOMCheck && len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
		advance = 3
		s.noBOMCheck = true
	}

	origData := data
	if atEOF && len(data) == 0 {
		// No more data, tell Scanner to stop.
		return 0, nil, nil
	}

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
	skip := 0
	var line []byte
	for {
		line = readLine()
		if len(line) == 0 {
			return 0, nil, nil // Request more data
		}
		if s.comment != 0 && nextRune(line) == s.comment {
			advance += len(line)
			skip += len(line)
			continue // Skip comment lines
		}
		if len(line) == lenNewline(line) {
			advance += len(line)
			skip += len(line)
			continue // Skip empty lines
		}
		break
	}

	// Parse each field in the record.
	const quoteLen = len(`"`)
	tokenHasCR := false
	s.recordBuffer = s.recordBuffer[:0]
	s.fieldIndexes = s.fieldIndexes[:0]
parseField:
	for {
		if len(line) == 0 || line[0] != '"' {
			// Non-quoted string field
			i := bytes.IndexRune(line, s.separator)
			field := line
			if i >= 0 {
				advance += i + s.sepLen
				field = field[:i]
			} else {
				advance += len(field)
				field = field[:len(field)-lenNewline(field)]
			}
			s.recordBuffer = append(s.recordBuffer, field...)
			s.fieldIndexes = append(s.fieldIndexes, len(s.recordBuffer))
			if i >= 0 {
				line = line[i+s.sepLen:]
				continue parseField
			}
			break parseField
		} else {
			// Quoted string field
			line = line[quoteLen:]
			advance += quoteLen
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
						line = line[s.sepLen:]
						s.fieldIndexes = append(s.fieldIndexes, len(s.recordBuffer))
						advance += s.sepLen
						continue parseField
					case lenNewline(line) == len(line):
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
					newlineLen := lenNewline(line)
					if newlineLen == 2 {
						tokenHasCR = true
						s.recordBuffer = append(s.recordBuffer, line[:len(line)-2]...)
						s.recordBuffer = append(s.recordBuffer, '\n')
					} else {
						s.recordBuffer = append(s.recordBuffer, line...)
					}
					line = readLine()
					if line == nil {
						return 0, nil, nil // Request more data
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

	// Create a single string and create slices out of it.
	// This pins the memory of the fields together, but allocates once.
	strBuf := string(s.recordBuffer) // Convert to string once to batch allocations
	fields := make([]string, len(s.fieldIndexes))
	preIdx := 0
	for i, idx := range s.fieldIndexes {
		fields[i] = strBuf[preIdx:idx]
		preIdx = idx
	}

	s.noBOMCheck = true

	if s.rowNum == 0 && s.header {
		// Set header field names and advance, but don't return a line (token).
		s.rowNum++
		s.setFieldNames(fields)
		return advance, nil, nil
	}

	// Normal row, set fields and return a line (token).
	s.rowNum++
	*s.fields = fields
	token = origData[skip:advance]
	token = token[:len(token)-lenNewline(token)]
	if tokenHasCR {
		token = bytes.ReplaceAll(token, []byte{'\r'}, nil)
	}
	return advance, token, nil
}

// lenNewline reports the number of bytes for the trailing \n.
func lenNewline(b []byte) int {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		if len(b) > 1 && b[len(b)-2] == '\r' {
			return 2
		}
		return 1
	}
	return 0
}

// nextRune returns the next rune in b or utf8.RuneError.
func nextRune(b []byte) rune {
	r, _ := utf8.DecodeRune(b)
	return r
}

// Setup for a new input file with given name (empty string if stdin)
func (p *interp) setFile(filename string) {
	p.filename = numStr(filename)
	p.fileLineNum = 0
	p.hadFiles = true
}

// Setup for a new input line (but don't parse it into fields till we
// need to)
func (p *interp) setLine(line string, isTrueStr bool) {
	p.line = line
	p.lineIsTrueStr = isTrueStr
	p.haveFields = false
	p.reparseCSV = true
}

// Splits on FS as a regex, appending each field to fields and returning the
// new slice (for efficiency).
func (p *interp) splitOnFieldSepRegex(fields []string, line string) []string {
	indices := p.fieldSepRegex.FindAllStringIndex(line, -1)
	prevIndex := 0
	for _, match := range indices {
		start, end := match[0], match[1]
		// skip empty matches (https://www.austingroupbugs.net/view.php?id=1468)
		if start == end {
			continue
		}
		fields = append(fields, line[prevIndex:start])
		prevIndex = end
	}
	fields = append(fields, line[prevIndex:])
	return fields
}

// Ensure that the current line is parsed into fields, splitting it
// into fields if it hasn't been already
func (p *interp) ensureFields() {
	if p.haveFields {
		return
	}
	p.haveFields = true

	switch {
	case p.inputMode == CSVMode || p.inputMode == TSVMode:
		if p.reparseCSV {
			scanner := bufio.NewScanner(strings.NewReader(p.line))
			scanner.Buffer(nil, maxRecordLength)
			splitter := csvSplitter{
				separator: p.csvInputConfig.Separator,
				sepLen:    utf8.RuneLen(p.csvInputConfig.Separator),
				comment:   p.csvInputConfig.Comment,
				fields:    &p.fields,
			}
			scanner.Split(splitter.scan)
			if !scanner.Scan() {
				p.fields = nil
			}
		} else {
			// Normally fields have already been parsed by csvSplitter
		}
	case p.fieldSep == " ":
		// FS space (default) means split fields on any whitespace
		p.fields = strings.Fields(p.line)
	case p.line == "":
		p.fields = nil
	case utf8.RuneCountInString(p.fieldSep) <= 1:
		// 1-char FS is handled as plain split (not regex)
		p.fields = strings.Split(p.line, p.fieldSep)
	default:
		// Split on FS as a regex
		p.fields = p.splitOnFieldSepRegex(p.fields[:0], p.line)
	}

	// Special case for when RS=="" and FS is single character,
	// split on newline in addition to FS. See more here:
	// https://www.gnu.org/software/gawk/manual/html_node/Multiple-Line.html
	if p.inputMode == DefaultMode && p.recordSep == "" && utf8.RuneCountInString(p.fieldSep) == 1 {
		fields := make([]string, 0, len(p.fields))
		for _, field := range p.fields {
			lines := strings.Split(field, "\n")
			for _, line := range lines {
				trimmed := strings.TrimSuffix(line, "\r")
				fields = append(fields, trimmed)
			}
		}
		p.fields = fields
	}

	p.fieldsIsTrueStr = p.fieldsIsTrueStr[:0] // avoid allocation most of the time
	for range p.fields {
		p.fieldsIsTrueStr = append(p.fieldsIsTrueStr, false)
	}
	p.numFields = len(p.fields)
}

// Fetch next line (record) of input from current input file, opening
// next input file if done with previous one
func (p *interp) nextLine() (string, error) {
	for {
		if p.scanner == nil {
			if prevInput, ok := p.input.(io.Closer); ok && p.input != p.stdin {
				// Previous input is file, close it
				_ = prevInput.Close()
			}
			if p.filenameIndex >= p.argc && !p.hadFiles {
				// Moved past number of ARGV args and haven't seen
				// any files yet, use stdin
				p.input = p.stdin
				p.setFile("-")
			} else {
				if p.filenameIndex >= p.argc {
					// Done with ARGV args, all done with input
					return "", io.EOF
				}
				// Fetch next filename from ARGV. Can't use
				// getArrayValue() here as it would set the value if
				// not present
				index := strconv.Itoa(p.filenameIndex)
				argvIndex := p.arrayIndexes["ARGV"]
				argvArray := p.array(resolver.Global, argvIndex)
				filename := p.toString(argvArray[index])
				p.filenameIndex++

				// Is it actually a var=value assignment?
				var matches []string
				if !p.noArgVars {
					matches = varRegex.FindStringSubmatch(filename)
				}
				if len(matches) >= 3 {
					// Yep, set variable to value and keep going
					name, val := matches[1], matches[2]
					// Oddly, var=value args must interpret escapes (issue #129)
					unescaped, err := Unescape(val)
					if err == nil {
						val = unescaped
					}
					err = p.setVarByName(name, val)
					if err != nil {
						return "", err
					}
					continue
				} else if filename == "" {
					// ARGV arg is empty string, skip
					p.input = nil
					continue
				} else if filename == "-" {
					// ARGV arg is "-" meaning stdin
					p.input = p.stdin
					p.setFile("-")
				} else {
					// A regular file name, open it
					if p.noFileReads {
						return "", newError("can't read from file due to NoFileReads")
					}
					input, err := os.Open(filename)
					if err != nil {
						return "", err
					}
					p.input = input
					p.setFile(filename)
				}
			}
			if p.inputBuffer == nil { // reuse buffer from last input file
				p.inputBuffer = make([]byte, inputBufSize)
			}
			p.scanner = p.newScanner(p.input, p.inputBuffer)
		}
		p.recordTerminator = p.recordSep // will be overridden if RS is "" or multiple chars
		if p.scanner.Scan() {
			// We scanned some input, break and return it
			break
		}
		err := p.scanner.Err()
		if err != nil {
			return "", fmt.Errorf("error reading from input: %s", err)
		}
		// Signal loop to move onto next file
		p.scanner = nil
	}

	// Got a line (record) of input, return it
	p.lineNum++
	p.fileLineNum++
	return p.scanner.Text(), nil
}

// Write output string to given writer, producing correct line endings
// on Windows (CR LF).
func writeOutput(w io.Writer, s string) error {
	if crlfNewline {
		// First normalize to \n, then convert all newlines to \r\n
		// (on Windows). NOTE: creating two new strings is almost
		// certainly slow; would be better to create a custom Writer.
		s = strings.Replace(s, "\r\n", "\n", -1)
		s = strings.Replace(s, "\n", "\r\n", -1)
	}
	_, err := io.WriteString(w, s)
	return err
}

// Close all streams and so on (after program execution).
func (p *interp) closeAll() {
	if prevInput, ok := p.input.(io.Closer); ok {
		_ = prevInput.Close()
	}
	for _, r := range p.inputStreams {
		_ = r.Close()
	}
	for _, w := range p.outputStreams {
		_ = w.Close()
	}
	if f, ok := p.output.(flusher); ok {
		_ = f.Flush()
	}
	if f, ok := p.errorOutput.(flusher); ok {
		_ = f.Flush()
	}
}

// Flush all output streams as well as standard output. Report whether all
// streams were flushed successfully (logging error(s) if not).
func (p *interp) flushAll() bool {
	allGood := true
	for name, writer := range p.outputStreams {
		if !p.flushWriter(name, writer) {
			allGood = false
		}
	}
	if !p.flushWriter("stdout", p.output) {
		allGood = false
	}
	return allGood
}

// Flush a single, named output stream, and report whether it was flushed
// successfully (logging an error if not).
func (p *interp) flushStream(name string) bool {
	writer := p.outputStreams[name]
	if writer == nil {
		p.printErrorf("error flushing %q: not an output file or pipe\n", name)
		return false
	}
	return p.flushWriter(name, writer)
}

type flusher interface {
	Flush() error
}

// Flush given output writer, and report whether it was flushed successfully
// (logging an error if not).
func (p *interp) flushWriter(name string, writer io.Writer) bool {
	flusher, ok := writer.(flusher)
	if !ok {
		return true // not a flusher, don't error
	}
	err := flusher.Flush()
	if err != nil {
		p.printErrorf("error flushing %q: %v\n", name, err)
		return false
	}
	return true
}

// Flush output and error streams.
func (p *interp) flushOutputAndError() {
	if flusher, ok := p.output.(flusher); ok {
		_ = flusher.Flush()
	}
	if flusher, ok := p.errorOutput.(flusher); ok {
		_ = flusher.Flush()
	}
}

// Print a message to the error output stream, flushing as necessary.
func (p *interp) printErrorf(format string, args ...interface{}) {
	if flusher, ok := p.output.(flusher); ok {
		_ = flusher.Flush() // ensure synchronization
	}
	fmt.Fprintf(p.errorOutput, format, args...)
	if flusher, ok := p.errorOutput.(flusher); ok {
		_ = flusher.Flush()
	}
}
