package numbers

import (
	"strconv"
	"testing"
)

func TestParseInt64(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		ok       bool
	}{
		{"0", 0, true},
		{"1", 1, true},
		{"123", 123, true},
		{"12345678", 12345678, true},
		{"123456789012345678", 123456789012345678, true},
		{"-1", -1, true},
		{"-123", -123, true},
		{"-12345678", -12345678, true},
		{"", 0, false},
		{"abc", 0, false},
		{"12a34", 0, false},
		{"-", 0, false},
	}

	for _, tc := range tests {
		result, ok := ParseInt64([]byte(tc.input))
		if ok != tc.ok {
			t.Errorf("ParseInt64(%q): ok=%v, expected ok=%v", tc.input, ok, tc.ok)
			continue
		}
		if ok && result != tc.expected {
			t.Errorf("ParseInt64(%q) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

func TestParseUint64(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
		ok       bool
	}{
		{"0", 0, true},
		{"1", 1, true},
		{"12345678", 12345678, true},
		{"18446744073709551615", 0, false}, // Max uint64, but 20 digits
		{"", 0, false},
		{"abc", 0, false},
	}

	for _, tc := range tests {
		result, ok := ParseUint64([]byte(tc.input))
		if ok != tc.ok {
			t.Errorf("ParseUint64(%q): ok=%v, expected ok=%v", tc.input, ok, tc.ok)
			continue
		}
		if ok && result != tc.expected {
			t.Errorf("ParseUint64(%q) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

func TestSwar8Digits(t *testing.T) {
	// Test the SWAR algorithm directly
	// Input: bytes representing "12345678"
	input := uint64(0x3837363534333231) // "12345678" in little-endian
	sub := input - 0x3030303030303030   // Subtract '0' from each byte

	result := swar8Digits(sub)
	expected := uint64(12345678)

	if result != expected {
		t.Errorf("swar8Digits: got %d, expected %d", result, expected)
	}
}

// Benchmarks comparing to strconv

func BenchmarkParseInt64(b *testing.B) {
	inputs := []string{
		"0",
		"123",
		"12345678",
		"1234567890123456",
	}

	for _, input := range inputs {
		bytes := []byte(input)

		b.Run("SWAR/"+input, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = ParseInt64(bytes)
			}
		})

		b.Run("strconv/"+input, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = strconv.ParseInt(input, 10, 64)
			}
		})
	}
}

func BenchmarkParseUint64(b *testing.B) {
	input := []byte("12345678")

	b.Run("SWAR", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = ParseUint64(input)
		}
	})

	b.Run("strconv", func(b *testing.B) {
		s := string(input)
		for i := 0; i < b.N; i++ {
			_, _ = strconv.ParseUint(s, 10, 64)
		}
	})
}
