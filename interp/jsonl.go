// JSON Lines input parsing for GoAWK interpreter.

package interp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
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

// parseJSONLine calls parseJSONLineToFields and updates p.fields and p.fieldNames.
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
// (for objects) the field names. For JSON objects, nested structures are
// flattened using dot notation: object keys use @"parent.child" and array
// indexes use @"parent.0", @"parent.1", etc.
// For top-level JSON arrays, names is nil and elements map to $1, $2, etc.
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
			if err := flattenObject(dec, "", &fields, &names); err != nil {
				return nil, nil, err
			}
			return fields, names, nil
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

// parseJSONArrayFields reads JSON array elements ('{' already consumed) and
// returns them as positional fields. Non-scalar elements are returned as their
// JSON string representation.
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

// flattenJSONValue recursively flattens a raw JSON value into fields/names
// using dot notation for objects and numeric indexes for arrays.
// path is the dot-separated key path so far (empty at the top level).
func flattenJSONValue(raw json.RawMessage, path string, fields *[]string, names *[]string) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	switch raw[0] {
	case '{':
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if _, err := dec.Token(); err != nil { // consume '{'
			return err
		}
		return flattenObject(dec, path, fields, names)
	case '[':
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if _, err := dec.Token(); err != nil { // consume '['
			return err
		}
		return flattenArray(dec, path, fields, names)
	default:
		// Scalar value: add to fields with its path as the name.
		*fields = append(*fields, jsonRawToString(raw))
		*names = append(*names, path)
		return nil
	}
}

// flattenObject processes a JSON object ('{' already consumed) and flattens
// its key-value pairs into fields/names using dot-notation paths.
func flattenObject(dec *json.Decoder, prefix string, fields *[]string, names *[]string) error {
	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("expected string key in JSON object, got %T", keyToken)
		}

		var path string
		if prefix == "" {
			path = key
		} else {
			path = prefix + "." + key
		}

		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}

		if err := flattenJSONValue(raw, path, fields, names); err != nil {
			return err
		}
	}
	_, err := dec.Token() // consume '}'
	return err
}

// flattenArray processes a JSON array ('[' already consumed) and flattens
// its elements into fields/names using numeric-index paths (prefix.0, prefix.1, ...).
func flattenArray(dec *json.Decoder, prefix string, fields *[]string, names *[]string) error {
	i := 0
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		path := prefix + "." + strconv.Itoa(i)
		if err := flattenJSONValue(raw, path, fields, names); err != nil {
			return err
		}
		i++
	}
	_, err := dec.Token() // consume ']'
	return err
}

// jsonRawToString returns the AWK string representation of a scalar JSON value.
// For non-scalar values (arrays and objects), the raw JSON is returned as-is
// (used when a top-level JSON array contains nested structures).
func jsonRawToString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case 'n': // null
		return ""
	case 't': // true
		return "1"
	case 'f': // false
		return "0"
	case '"': // string
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
		return ""
	case '[', '{': // array or object – return JSON representation
		return string(raw)
	default: // number
		var n json.Number
		if err := json.Unmarshal(raw, &n); err == nil {
			return n.String()
		}
		return ""
	}
}
