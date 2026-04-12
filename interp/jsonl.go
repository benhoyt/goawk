// JSON Lines input parsing for GoAWK interpreter.

package interp

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// jsonlSplitter is a bufio.Scanner split function for JSON Lines input.
// It splits on newlines, skipping empty lines, and pre-parses each JSON line
// into fields (like csvSplitter does for CSV). This ensures that FIELDS and
// other per-record state are populated before each action body runs.
type jsonlSplitter struct {
	fields        *[]string
	setFieldNames func(names []string)
	interp        *interp // for error reporting
}

func (s *jsonlSplitter) scan(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Find and skip empty lines; stop at first non-empty line.
	skip := 0
	var line []byte
	for {
		newline := bytes.IndexByte(data, '\n')
		var lineEnd int
		if newline >= 0 {
			lineEnd = newline + 1
		} else if atEOF {
			lineEnd = len(data)
		} else {
			return 0, nil, nil // need more data
		}

		candidate := dropCR(data[:lineEnd-lenNewline(data[:lineEnd])])
		if len(bytes.TrimSpace(candidate)) > 0 {
			line = candidate
			advance += lineEnd
			break
		}
		// Empty line: skip it
		advance += lineEnd
		skip += lineEnd
		data = data[lineEnd:]
		if atEOF && len(data) == 0 {
			return advance, nil, nil
		}
	}

	// Parse the JSON line and populate fields / field names.
	fields, names, parseErr := parseJSONLineToFields(line)
	if parseErr != nil {
		if s.interp != nil {
			fmt.Fprintf(s.interp.errorOutput, "goawk: %s\n", parseErr)
		}
		fields = nil
		names = nil
	}
	*s.fields = fields
	s.setFieldNames(names)

	return advance, line, nil
}

// parseJSONLine parses a JSON line and populates p.fields and p.fieldNames.
// Called by ensureFields() when $0 is reassigned in JSONL mode.
func (p *interp) parseJSONLine(line string) error {
	fields, names, err := parseJSONLineToFields([]byte(line))
	if err != nil {
		return err
	}
	p.fields = fields
	p.setFieldNames(names)
	return nil
}

// parseJSONLineToFields parses a JSON line and returns the field values and
// (for objects) the field names. For arrays, names is nil.
func parseJSONLineToFields(line []byte) (fields []string, names []string, err error) {
	if len(bytes.TrimSpace(line)) == 0 {
		return nil, nil, nil
	}

	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()

	token, err := dec.Token()
	if err != nil {
		return nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}

	switch t := token.(type) {
	case json.Delim:
		switch t {
		case '[':
			fields, err = parseJSONArrayFields(dec)
			return fields, nil, err
		case '{':
			return parseJSONObjectFields(dec)
		default:
			return nil, nil, fmt.Errorf("unexpected JSON delimiter %q", t)
		}
	case nil: // JSON null
		return []string{""}, nil, nil
	case bool:
		if t {
			return []string{"1"}, nil, nil
		}
		return []string{"0"}, nil, nil
	case json.Number:
		return []string{t.String()}, nil, nil
	case string:
		return []string{t}, nil, nil
	default:
		return nil, nil, fmt.Errorf("unexpected JSON token type %T", token)
	}
}

func parseJSONArrayFields(dec *json.Decoder) (fields []string, err error) {
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		fields = append(fields, jsonRawToString(raw))
	}
	// consume the closing ']'
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return fields, nil
}

func parseJSONObjectFields(dec *json.Decoder) (fields []string, names []string, err error) {
	for dec.More() {
		// Read the object key
		keyToken, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, nil, fmt.Errorf("expected string key in JSON object, got %T", keyToken)
		}

		// Read the value as raw JSON to preserve document order and type
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, err
		}

		fields = append(fields, jsonRawToString(raw))
		names = append(names, key)
	}
	// consume the closing '}'
	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}
	return fields, names, nil
}

// jsonRawToValue converts a raw JSON value to an AWK value:
//   - null      → numStr("")
//   - true      → numStr("1")
//   - false     → numStr("0")
//   - number    → numStr(<decimal string>)
//   - string    → numStr(<unquoted string>)
//   - array/obj → numStr(<JSON representation>)
func jsonRawToValue(raw json.RawMessage) value {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return numStr("")
	}
	switch raw[0] {
	case 'n': // null
		return numStr("")
	case 't': // true
		return numStr("1")
	case 'f': // false
		return numStr("0")
	case '"': // string
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return numStr(s)
		}
		return numStr("")
	case '[', '{': // array or object – return JSON representation
		return numStr(string(raw))
	default: // number
		var n json.Number
		if err := json.Unmarshal(raw, &n); err == nil {
			return numStr(n.String())
		}
		return numStr("")
	}
}

// jsonRawToString returns the AWK string representation of a raw JSON value.
func jsonRawToString(raw json.RawMessage) string {
	return jsonRawToValue(raw).s
}
