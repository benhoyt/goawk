//go:build awkgoexample
// +build awkgoexample

package main

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	_output  *bufio.Writer
	_scanner *bufio.Scanner
	_line    string
	_fields  []string
	_lineNum int
	_seed    float64
	_rand    *rand.Rand

	CONVFMT string
	FS      string
	OFMT    string
	OFS     string
	ORS     string
	RLENGTH float64
	RSTART  float64
	SUBSEP  string
	n       float64
	x       float64
)

func main() {
	_output = bufio.NewWriter(os.Stdout)
	defer _output.Flush()

	_scanner = bufio.NewScanner(os.Stdin)
	_seed = 1.0
	_rand = rand.New(rand.NewSource(int64(math.Float64bits(_seed))))

	FS = " "
	OFS = " "
	ORS = "\n"
	OFMT = "%.6g"
	CONVFMT = "%.6g"
	SUBSEP = "\x1c"

	for _scanner.Scan() {
		_lineNum++
		_line = _scanner.Text()
		_fields = _splitHelper(_line, FS)

		if _numToStr(_strToNum(_getField(1))+0.0) == _getField(2) {
			x += func() float64 { n++; return n }()
		}
	}

	if _scanner.Err() != nil {
		fmt.Fprintln(os.Stderr, _scanner.Err())
		os.Exit(1)
	}

	fmt.Fprintln(_output, _formatNum(x))
}

func _getField(i int) string {
	if i < 0 || i > len(_fields) {
		return ""
	}
	if i == 0 {
		return _line
	}
	return _fields[i-1]
}

func _setField(i int, s string) {
	if i == 0 {
		_line = s
		_fields = _splitHelper(_line, FS)
		return
	}
	for j := len(_fields); j < i; j++ {
		_fields = append(_fields, "")
	}
	_fields[i-1] = s
	_line = strings.Join(_fields, OFS)
}

func _boolToNum(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func _numToStrFormat(format string, n float64) string {
	switch {
	case math.IsNaN(n):
		return "nan"
	case math.IsInf(n, 0):
		if n < 0 {
			return "-inf"
		} else {
			return "inf"
		}
	case n == float64(int(n)):
		return strconv.Itoa(int(n))
	default:
		return fmt.Sprintf(format, n)
	}
}

func _numToStr(n float64) string {
	return _numToStrFormat(CONVFMT, n)
}

func _formatNum(n float64) string {
	return _numToStrFormat(OFMT, n)
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

// Like strconv.ParseFloat, but parses at the start of string and
// allows things like "1.5foo".
func _strToNum(s string) float64 {
	// Skip whitespace at start
	i := 0
	for i < len(s) && asciiSpace[s[i]] != 0 {
		i++
	}
	start := i

	// Parse mantissa: optional sign, initial digit(s), optional '.',
	// then more digits
	gotDigit := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		gotDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		gotDigit = true
		i++
	}
	if !gotDigit {
		return 0
	}

	// Parse exponent ("1e" and similar are allowed, but ParseFloat
	// rejects them)
	end := i
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			end = i
		}
	}

	floatStr := s[start:end]
	f, _ := strconv.ParseFloat(floatStr, 64)
	return f // Returns infinity in case of "value out of range" error
}

func _isFieldTrue(s string) bool {
	if s == "" {
		return false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return true
	}
	return f != 0
}

var _reCache = make(map[string]*regexp.Regexp)

func _reCompile(pattern string) *regexp.Regexp {
	if re, ok := _reCache[pattern]; ok {
		return re
	}
	re := regexp.MustCompile(pattern)
	// Dumb, non-LRU cache: just cache the first N regexes
	if len(_reCache) < 100 {
		_reCache[pattern] = re
	}
	return re
}

func _match(str string, re *regexp.Regexp) float64 {
	loc := re.FindStringIndex(str)
	if len(loc) > 0 {
		RSTART = float64(loc[0] + 1)
		RLENGTH = float64(loc[1] - loc[0])
	} else {
		RSTART = 0
		RLENGTH = -1
	}
	return RSTART
}

func _srand(seed float64) float64 {
	prev := _seed
	_seed = seed
	_rand.Seed(int64(math.Float64bits(seed)))
	return prev
}

func _srandNow() float64 {
	prev := _seed
	_rand.Seed(time.Now().UnixNano())
	return prev
}

func _substrLength(s string, pos, length int) string {
	if pos > len(s) {
		pos = len(s) + 1
	}
	if pos < 1 {
		pos = 1
	}
	if length < 0 {
		length = 0
	}
	maxLength := len(s) - pos + 1
	if length > maxLength {
		length = maxLength
	}
	return s[pos-1 : pos-1+length]
}

func _substr(s string, pos int) string {
	if pos > len(s) {
		pos = len(s) + 1
	}
	if pos < 1 {
		pos = 1
	}
	length := len(s) - pos + 1
	return s[pos-1 : pos-1+length]
}

func _firstRune(s string) int {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return 0
	}
	return int(r)
}

func _splitHelper(s, fs string) []string {
	var parts []string
	if fs == " " {
		parts = strings.Fields(s)
	} else if s == "" {
		// NF should be 0 on empty line
	} else if utf8.RuneCountInString(fs) <= 1 {
		parts = strings.Split(s, fs)
	} else {
		parts = _reCompile(fs).Split(s, -1)
	}
	return parts
}

func _split(s string, a map[string]string, fs string) float64 {
	parts := _splitHelper(s, fs)
	for k := range a {
		delete(a, k)
	}
	for i, part := range parts {
		a[strconv.Itoa(i+1)] = part
	}
	return float64(len(a))
}

func _sub(re *regexp.Regexp, repl, in string, global bool) (out string, num int) {
	count := 0
	out = re.ReplaceAllStringFunc(in, func(s string) string {
		// Only do the first replacement for sub(), or all for gsub()
		if !global && count > 0 {
			return s
		}
		count++
		// Handle & (ampersand) properly in replacement string
		r := make([]byte, 0, 64) // Up to 64 byte replacement won't require heap allocation
		for i := 0; i < len(repl); i++ {
			switch repl[i] {
			case '&':
				r = append(r, s...)
			case '\\':
				i++
				if i < len(repl) {
					switch repl[i] {
					case '&':
						r = append(r, '&')
					case '\\':
						r = append(r, '\\')
					default:
						r = append(r, '\\', repl[i])
					}
				} else {
					r = append(r, '\\')
				}
			default:
				r = append(r, repl[i])
			}
		}
		return string(r)
	})
	return out, count
}

func _system(command string) float64 {
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Stdout = _output
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return -1
	}
	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return float64(exitErr.ProcessState.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "unexpected error running command %q: %v\n", command, err)
		return -1
	}
	return 0
}

func _fflush() float64 {
	err := _output.Flush()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error flushing output: %v\n", err)
		return -1
	}
	return 0
}
