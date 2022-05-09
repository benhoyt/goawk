// Tests copied from encoding/csv to ensure we pass all the relevant cases.

// These tests are a subset of those in encoding/csv used to test Reader.
// However, the §, ¶ and ∑ special characters (for error positions) have been
// removed, and some tests have been removed or tweaked slightly because we
// don't support all the encoding/csv features (FieldsPerRecord is not
// supported, LazyQuotes is always on, and TrimLeadingSpace is always off).

package interp

import (
	"bufio"
	"encoding/csv"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

type readTest struct {
	Name   string
	Input  string
	Output [][]string
	Error  string

	// These fields are copied into the CSVInputConfig
	Comma   rune
	Comment rune
}

var readTests = []readTest{{
	Name:   "Simple",
	Input:  "a,b,c\n",
	Output: [][]string{{"a", "b", "c"}},
}, {
	Name:   "CRLF",
	Input:  "a,b\r\nc,d\r\n",
	Output: [][]string{{"a", "b"}, {"c", "d"}},
}, {
	Name:   "BareCR",
	Input:  "a,b\rc,d\r\n",
	Output: [][]string{{"a", "b\rc", "d"}},
}, {
	Name: "RFC4180test",
	Input: `#field1,field2,field3
"aaa","bb
b","ccc"
"a,a","b""bb","ccc"
zzz,yyy,xxx
`,
	Output: [][]string{
		{"#field1", "field2", "field3"},
		{"aaa", "bb\nb", "ccc"},
		{"a,a", `b"bb`, "ccc"},
		{"zzz", "yyy", "xxx"},
	},
}, {
	Name:   "NoEOLTest",
	Input:  "a,b,c",
	Output: [][]string{{"a", "b", "c"}},
}, {
	Name:   "Semicolon",
	Input:  "a;b;c\n",
	Output: [][]string{{"a", "b", "c"}},
	Comma:  ';',
}, {
	Name: "MultiLine",
	Input: `"two
line","one line","three
line
field"`,
	Output: [][]string{{"two\nline", "one line", "three\nline\nfield"}},
}, {
	Name:  "BlankLine",
	Input: "a,b,c\n\nd,e,f\n\n",
	Output: [][]string{
		{"a", "b", "c"},
		{"d", "e", "f"},
	},
}, {
	Name:  "BlankLineFieldCount",
	Input: "a,b,c\n\nd,e,f\n\n",
	Output: [][]string{
		{"a", "b", "c"},
		{"d", "e", "f"},
	},
}, {
	Name:   "LeadingSpace",
	Input:  " a,  b,   c\n",
	Output: [][]string{{" a", "  b", "   c"}},
}, {
	Name:    "Comment",
	Input:   "#1,2,3\na,b,c\n#comment",
	Output:  [][]string{{"a", "b", "c"}},
	Comment: '#',
}, {
	Name:   "NoComment",
	Input:  "#1,2,3\na,b,c",
	Output: [][]string{{"#1", "2", "3"}, {"a", "b", "c"}},
}, {
	Name:   "LazyQuotes",
	Input:  `a "word","1"2",a","b`,
	Output: [][]string{{`a "word"`, `1"2`, `a"`, `b`}},
}, {
	Name:   "BareQuotes",
	Input:  `a "word","1"2",a"`,
	Output: [][]string{{`a "word"`, `1"2`, `a"`}},
}, {
	Name:   "BareDoubleQuotes",
	Input:  `a""b,c`,
	Output: [][]string{{`a""b`, `c`}},
}, {
	Name:   "TrimQuote",
	Input:  `"a"," b",c`,
	Output: [][]string{{"a", " b", "c"}},
}, {
	Name:   "FieldCount",
	Input:  "a,b,c\nd,e",
	Output: [][]string{{"a", "b", "c"}, {"d", "e"}},
}, {
	Name:   "TrailingCommaEOF",
	Input:  "a,b,c,",
	Output: [][]string{{"a", "b", "c", ""}},
}, {
	Name:   "TrailingCommaEOL",
	Input:  "a,b,c,\n",
	Output: [][]string{{"a", "b", "c", ""}},
}, {
	Name:   "TrailingCommaSpaceEOF",
	Input:  "a,b,c, ",
	Output: [][]string{{"a", "b", "c", " "}},
}, {
	Name:   "TrailingCommaSpaceEOL",
	Input:  "a,b,c, \n",
	Output: [][]string{{"a", "b", "c", " "}},
}, {
	Name:   "TrailingCommaLine3",
	Input:  "a,b,c\nd,e,f\ng,hi,",
	Output: [][]string{{"a", "b", "c"}, {"d", "e", "f"}, {"g", "hi", ""}},
}, {
	Name:   "NotTrailingComma3",
	Input:  "a,b,c, \n",
	Output: [][]string{{"a", "b", "c", " "}},
}, {
	Name: "CommaFieldTest",
	Input: `x,y,z,w
x,y,z,
x,y,,
x,,,
,,,
"x","y","z","w"
"x","y","z",""
"x","y","",""
"x","","",""
"","","",""
`,
	Output: [][]string{
		{"x", "y", "z", "w"},
		{"x", "y", "z", ""},
		{"x", "y", "", ""},
		{"x", "", "", ""},
		{"", "", "", ""},
		{"x", "y", "z", "w"},
		{"x", "y", "z", ""},
		{"x", "y", "", ""},
		{"x", "", "", ""},
		{"", "", "", ""},
	},
}, {
	Name:  "TrailingCommaIneffective1",
	Input: "a,b,\nc,d,e",
	Output: [][]string{
		{"a", "b", ""},
		{"c", "d", "e"},
	},
}, {
	Name:  "ReadAllReuseRecord",
	Input: "a,b\nc,d",
	Output: [][]string{
		{"a", "b"},
		{"c", "d"},
	},
}, {
	Name:  "CRLFInQuotedField", // Issue 21201
	Input: "A,\"Hello\r\nHi\",B\r\n",
	Output: [][]string{
		{"A", "Hello\nHi", "B"},
	},
}, {
	Name:   "BinaryBlobField", // Issue 19410
	Input:  "x09\x41\xb4\x1c,aktau",
	Output: [][]string{{"x09A\xb4\x1c", "aktau"}},
}, {
	Name:   "TrailingCR",
	Input:  "field1,field2\r",
	Output: [][]string{{"field1", "field2"}},
}, {
	Name:   "QuotedTrailingCR",
	Input:  "\"field\"\r",
	Output: [][]string{{"field"}},
}, {
	Name:   "FieldCR",
	Input:  "field\rfield\r",
	Output: [][]string{{"field\rfield"}},
}, {
	Name:   "FieldCRCR",
	Input:  "field\r\rfield\r\r",
	Output: [][]string{{"field\r\rfield\r"}},
}, {
	Name:   "FieldCRCRLF",
	Input:  "field\r\r\nfield\r\r\n",
	Output: [][]string{{"field\r"}, {"field\r"}},
}, {
	Name:   "FieldCRCRLFCR",
	Input:  "field\r\r\n\rfield\r\r\n\r",
	Output: [][]string{{"field\r"}, {"\rfield\r"}},
}, {
	Name:   "FieldCRCRLFCRCR",
	Input:  "field\r\r\n\r\rfield\r\r\n\r\r",
	Output: [][]string{{"field\r"}, {"\r\rfield\r"}, {"\r"}},
}, {
	Name:  "MultiFieldCRCRLFCRCR",
	Input: "field1,field2\r\r\n\r\rfield1,field2\r\r\n\r\r,",
	Output: [][]string{
		{"field1", "field2\r"},
		{"\r\rfield1", "field2\r"},
		{"\r\r", ""},
	},
}, {
	Name:    "NonASCIICommaAndComment",
	Input:   "a£b,c£ \td,e\n€ comment\n",
	Output:  [][]string{{"a", "b,c", " \td,e"}},
	Comma:   '£',
	Comment: '€',
}, {
	Name:    "NonASCIICommaAndCommentWithQuotes",
	Input:   "a€\"  b,\"€ c\nλ comment\n",
	Output:  [][]string{{"a", "  b,", " c"}},
	Comma:   '€',
	Comment: 'λ',
}, {
	// λ and θ start with the same byte.
	// This tests that the parser doesn't confuse such characters.
	Name:    "NonASCIICommaConfusion",
	Input:   "\"abθcd\"λefθgh",
	Output:  [][]string{{"abθcd", "efθgh"}},
	Comma:   'λ',
	Comment: '€',
}, {
	Name:    "NonASCIICommentConfusion",
	Input:   "λ\nλ\nθ\nλ\n",
	Output:  [][]string{{"λ"}, {"λ"}, {"λ"}},
	Comment: 'θ',
}, {
	Name:   "QuotedFieldMultipleLF",
	Input:  "\"\n\n\n\n\"",
	Output: [][]string{{"\n\n\n\n"}},
}, {
	Name:  "MultipleCRLF",
	Input: "\r\n\r\n\r\n\r\n",
}, {
	// The implementation may read each line in several chunks if it doesn't fit entirely
	// in the read buffer, so we should test the code to handle that condition.
	Name:    "HugeLines",
	Input:   strings.Repeat("#ignore\n", 10000) + "" + strings.Repeat("@", 5000) + "," + strings.Repeat("*", 5000),
	Output:  [][]string{{strings.Repeat("@", 5000), strings.Repeat("*", 5000)}},
	Comment: '#',
}, {
	Name:   "LazyQuoteWithTrailingCRLF",
	Input:  "\"foo\"bar\"\r\n",
	Output: [][]string{{`foo"bar`}},
}, {
	Name:   "DoubleQuoteWithTrailingCRLF",
	Input:  "\"foo\"\"bar\"\r\n",
	Output: [][]string{{`foo"bar`}},
}, {
	Name:   "EvenQuotes",
	Input:  `""""""""`,
	Output: [][]string{{`"""`}},
}, {
	Name:   "LazyOddQuotes",
	Input:  `"""""""`,
	Output: [][]string{{`"""`}},
}, {
	Name:  "BadComma1",
	Comma: '\n',
	Error: "invalid CSV field separator or comment delimiter",
}, {
	Name:  "BadComma2",
	Comma: '\r',
	Error: "invalid CSV field separator or comment delimiter",
}, {
	Name:  "BadComma3",
	Comma: '"',
	Error: "invalid CSV field separator or comment delimiter",
}, {
	Name:  "BadComma4",
	Comma: utf8.RuneError,
	Error: "invalid CSV field separator or comment delimiter",
}, {
	Name:    "BadComment1",
	Comment: '\n',
	Error:   "invalid CSV field separator or comment delimiter",
}, {
	Name:    "BadComment2",
	Comment: '\r',
	Error:   "invalid CSV field separator or comment delimiter",
}, {
	Name:    "BadComment3",
	Comment: utf8.RuneError,
	Error:   "invalid CSV field separator or comment delimiter",
}, {
	Name:    "BadCommaComment",
	Comma:   'X',
	Comment: 'X',
	Error:   "invalid CSV field separator or comment delimiter",
}}

func TestCSVReader(t *testing.T) {
	for _, tt := range readTests {
		t.Run(tt.Name, func(t *testing.T) {
			inputConfig := CSVInputConfig{
				Separator: tt.Comma,
				Comment:   tt.Comment,
			}
			if inputConfig.Separator == 0 {
				inputConfig.Separator = ','
			}

			var out [][]string
			err := validateCSVInputConfig(CSVMode, inputConfig)
			if err == nil {
				var fields []string
				splitter := csvSplitter{
					separator: inputConfig.Separator,
					sepLen:    utf8.RuneLen(inputConfig.Separator),
					comment:   inputConfig.Comment,
					fields:    &fields,
				}
				scanner := bufio.NewScanner(strings.NewReader(tt.Input))
				scanner.Split(splitter.scan)
				scanner.Buffer(make([]byte, inputBufSize), maxRecordLength)

				for scanner.Scan() {
					row := make([]string, len(fields))
					copy(row, fields)
					out = append(out, row)

					// We don't explicitly check the returned token, but at
					// least check it parses to the same row.
					if strings.ContainsRune(tt.Input, '\r') {
						// But FieldCRCRLF and similar tests don't round-trip
						continue
					}
					token := scanner.Text()
					reader := csv.NewReader(strings.NewReader(token))
					reader.Comma = inputConfig.Separator
					reader.Comment = inputConfig.Comment
					reader.FieldsPerRecord = -1
					reader.LazyQuotes = true
					tokenRow, err := reader.Read()
					if err != nil {
						t.Fatalf("error reparsing token: %v", err)
					}
					if !reflect.DeepEqual(tokenRow, row) {
						t.Fatalf("token mismatch:\ngot  %q\nwant %q", tokenRow, row)
					}
				}
				err = scanner.Err()
			}

			if tt.Error != "" {
				if err == nil {
					t.Fatalf("error mismatch:\ngot  nil\nwant %q", tt.Error)
				}
				if err.Error() != tt.Error {
					t.Fatalf("error mismatch:\ngot  %q\nwant %q", err.Error(), tt.Error)
				}
				if out != nil {
					t.Fatalf("output mismatch:\ngot  %q\nwant nil", out)
				}
			} else {
				if err != nil {
					t.Fatalf("error mismatch:\ngot  %q\nwant nil", err.Error())
				}
				if !reflect.DeepEqual(out, tt.Output) {
					t.Fatalf("output mismatch:\ngot  %q\nwant %q", out, tt.Output)
				}
			}
		})
	}
}
