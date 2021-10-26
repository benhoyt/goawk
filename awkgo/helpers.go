package main

// TODO: we should include only the functions we use in the output

func (c *compiler) outputHelpers() {
	c.output(`
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
		_fields = strings.Fields(_line) // TODO: use FS
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

func _regexMatches(str, pattern string) bool {
	// TODO: cache these or pre-compile literal regexes
	re := regexp.MustCompile(pattern)
	return re.MatchString(str)
}

func _match(str, pattern string) float64 {
	re := regexp.MustCompile(pattern)
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

func _containsNum(array map[string]float64, index string) bool {
	_, ok := array[index]
	return ok
}

func _containsStr(array map[string]string, index string) bool {
	_, ok := array[index]
	return ok
}

// TODO: do these with inline func literal
func _assignNum(p *float64, v float64) float64 {
	*p = v
	return v
}

func _assignStr(p *string, v string) string {
	*p = v
	return v
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

func _split(s string, a map[string]string, fs string) float64 {
	var parts []string
	if fs == " " {
		parts = strings.Fields(s)
	} else if s == "" {
		// NF should be 0 on empty line
	} else if utf8.RuneCountInString(fs) <= 1 {
		parts = strings.Split(s, fs)
	} else {
		re := regexp.MustCompile(fs)
		parts = re.Split(s, -1)
	}
	for k := range a {
		delete(a, k)
	}
	for i, part := range parts {
		a[strconv.Itoa(i+1)] = part // TODO: should be a numeric string
	}
	return float64(len(a))
}

func _sub(regex, repl, in string, global bool) (out string, num int) {
	re := regexp.MustCompile(regex)
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
`)
}
