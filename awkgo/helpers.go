package main

func (c *compiler) outputHelpers() {
	c.output(`
func _getField(line string, fields []string, i int) string {
	if i == 0 {
		return line
	}
	if i >= 1 && i <= len(fields) {
        return fields[i-1]
    }
    return ""
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

func _regexMatch(pattern, str string) float64 {
	re := regexp.MustCompile(pattern)
	if re.MatchString(str) {
		return 1
	}
	return 0
}

func _condNum(cond, trueVal, falseVal float64) float64 {
	if cond != 0 {
		return trueVal
	}
	return falseVal
}

func _condStr(cond float64, trueVal, falseVal string) float64 {
	if cond != "" {
		return trueVal
	}
	return falseVal
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
`)
}
