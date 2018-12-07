// Tests for the strutil package

package strutil_test

import (
	"strings"
	"testing"

	"github.com/benhoyt/goawk/internal/strutil"
)

const space = "\t\v\r\f\n\u0085\u00a0\u2000\u3000"

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		in, out string
	}{
		{"", ""},
		{"abc", "abc"},
		{space + "abc" + space, "abc"},
		{" ", ""},
		{" \t\r\n \t\t\r\r\n\n ", ""},
		{" \t\r\n x\t\t\r\r\n\n ", "x"},
		{" \u2000\t\r\n x\t\t\r\r\ny\n \u3000", "x\t\t\r\r\ny"},
		{"1 \t\r\n2", "1 \t\r\n2"},
		{" x\x80", "x\x80"},
		{" x\xc0", "x\xc0"},
		{"x \xc0\xc0 ", "x \xc0\xc0"},
		{"x \xc0", "x \xc0"},
		{"x \xc0 ", "x \xc0"},
		{"x \xc0\xc0 ", "x \xc0\xc0"},
		{"x ☺\xc0\xc0 ", "x ☺\xc0\xc0"},
		{"x ☺ ", "x ☺"},
		{" ☺ x ", "☺ x"},
		{" x ", "x"},
		{" xy ", "xy"},
		{"xy", "xy"},
		{" ☺ ", "☺"},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			out := strutil.TrimSpace(test.in)
			if out != test.out {
				t.Errorf("expected %q, got %q", test.out, out)
			}
		})
	}
}

func BenchmarkStrutilTrimNoTrim(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strutil.TrimSpace("typical")
	}
}

func BenchmarkStdlibTrimNoTrim(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strings.TrimSpace("typical")
	}
}

func BenchmarkStrutilTrimASCII(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strutil.TrimSpace("  foo bar  ")
	}
}

func BenchmarkStdlibTrimASCII(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strings.TrimSpace("  foo bar  ")
	}
}

func BenchmarkStrutilTrimSomeUnicode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strutil.TrimSpace("    \u2000\t\r\n x\t\t\r\r\ny\n \u3000    ")
	}
}

func BenchmarkStdlibTrimSomeUnicode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strings.TrimSpace("    \u2000\t\r\n x\t\t\r\r\ny\n \u3000    ")
	}
}

func BenchmarkStrutilTrimJustUnicode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strutil.TrimSpace("\u2000\t\r\n x\t\t\r\r\ny\n \u3000")
	}
}

func BenchmarkStdlibTrimJustUnicode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strings.TrimSpace("\u2000\t\r\n x\t\t\r\r\ny\n \u3000")
	}
}
