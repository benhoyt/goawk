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
		_fields = strings.Fields(_line)
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

func _numToStr(n float64) string {
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
		return fmt.Sprintf("%.6g", n)
	}
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

func _assignNum(p *float64, v float64) float64 {
	*p = v
	return v
}

func _assignStr(p *string, v string) string {
	*p = v
	return v
}

func _preIncr(p *float64) float64 {
	*p++
	return *p
}

func _preDecr(p *float64) float64 {
	*p--
	return *p
}

func _postIncr(p *float64) float64 {
	x := *p
	*p++
	return x
}

func _postDecr(p *float64) float64 {
	x := *p
	*p--
	return x
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
`)
}
