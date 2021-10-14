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

func _regexMatch(str, pattern string) bool {
	re := regexp.MustCompile(pattern)
	return re.MatchString(str)
}

func _containsNum(array map[string]float64, index string) float64 {
	if _, ok := array[index]; ok {
		return 1
	}
	return 0
}

func _containsStr(array map[string]string, index string) float64 {
	if _, ok := array[index]; ok {
		return 1
	}
	return 0
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
`)
}
