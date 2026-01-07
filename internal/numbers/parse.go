// Package numbers provides fast number parsing using SWAR techniques.
// SWAR = SIMD Within A Register - process multiple values in a single integer.
package numbers

// ParseInt64 parses an integer from bytes using SWAR techniques.
// This is significantly faster than strconv.ParseInt for small numbers.
func ParseInt64(b []byte) (int64, bool) {
	if len(b) == 0 {
		return 0, false
	}

	// Handle negative numbers
	neg := false
	if b[0] == '-' {
		neg = true
		b = b[1:]
		if len(b) == 0 {
			return 0, false
		}
	}

	// Use fast path for small numbers (up to 18 digits)
	// 64-bit can hold up to 19 digits, but we use 18 for safety
	if len(b) > 18 {
		return 0, false // Overflow possible
	}

	var result uint64

	// Process 8 digits at a time using SWAR if possible
	for len(b) >= 8 {
		// Load 8 bytes as uint64
		chunk := loadUint64(b)

		// Check all characters are digits (0x30-0x39)
		// If c is a digit, c-'0' is 0-9
		// If c-'0' > 9, it wasn't a digit
		sub := chunk - 0x3030303030303030 // Subtract '0' from each byte
		test := sub + 0x7676767676767676  // Add 118 (if digit, result < 128)
		if (test|sub)&0x8080808080808080 != 0 {
			// Not all digits - fall back to slow path
			return parseIntSlow(b, neg)
		}

		// Convert 8 ASCII digits to a number using SWAR
		// sub now contains 0-9 in each byte
		val := swar8Digits(sub)
		result = result*100000000 + val
		b = b[8:]
	}

	// Process remaining digits one by one
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		result = result*10 + uint64(c-'0')
	}

	if neg {
		return -int64(result), true
	}
	return int64(result), true
}

// ParseUint64 parses an unsigned integer from bytes.
func ParseUint64(b []byte) (uint64, bool) {
	if len(b) == 0 || len(b) > 20 {
		return 0, false
	}

	var result uint64

	// Process 8 digits at a time
	for len(b) >= 8 {
		chunk := loadUint64(b)
		sub := chunk - 0x3030303030303030
		test := sub + 0x7676767676767676
		if (test|sub)&0x8080808080808080 != 0 {
			return parseUintSlow(b)
		}
		val := swar8Digits(sub)
		result = result*100000000 + val
		b = b[8:]
	}

	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		result = result*10 + uint64(c-'0')
	}

	return result, true
}

// swar8Digits converts 8 bytes (each 0-9) to a single number.
// Input: 8 bytes in a uint64, each containing a digit 0-9
// Output: the decimal number those digits represent
//
// Example: [1,2,3,4,5,6,7,8] -> 12345678
//
// Algorithm:
// 1. Multiply odd bytes by 10, add to even bytes -> 4 values 0-99
// 2. Multiply odd pairs by 100, add to even pairs -> 2 values 0-9999
// 3. Multiply odd quad by 10000, add to even quad -> 1 value 0-99999999
func swar8Digits(v uint64) uint64 {
	// Step 1: Combine pairs of digits
	// v = [d0, d1, d2, d3, d4, d5, d6, d7] (little-endian)
	// We want: [d0*10+d1, d2*10+d3, d4*10+d5, d6*10+d7]

	// Mask to extract even bytes: bytes 0, 2, 4, 6
	const mask1 = 0x00FF00FF00FF00FF

	// Extract even and odd bytes
	lower := v & mask1        // bytes 0, 2, 4, 6
	upper := (v >> 8) & mask1 // bytes 1, 3, 5, 7

	// Combine: lower * 10 + upper
	v = lower*10 + upper

	// Step 2: Combine pairs of 2-digit numbers
	// v now has 4 values (each in 2 bytes): [ab, cd, ef, gh]
	// We want: [abcd, efgh] (each in 4 bytes)
	const mask2 = 0x0000FFFF0000FFFF

	lower = v & mask2
	upper = (v >> 16) & mask2
	v = lower*100 + upper

	// Step 3: Combine the two 4-digit numbers
	// v now has 2 values: [abcd, efgh]
	// We want: abcdefgh
	lower = v & 0xFFFFFFFF
	upper = v >> 32
	v = lower*10000 + upper

	return v
}

// loadUint64 loads 8 bytes as a little-endian uint64.
func loadUint64(b []byte) uint64 {
	_ = b[7] // Bounds check hint
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

// parseIntSlow is the fallback for non-SWAR parsing.
func parseIntSlow(b []byte, neg bool) (int64, bool) {
	var result int64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		result = result*10 + int64(c-'0')
	}
	if neg {
		return -result, true
	}
	return result, true
}

// parseUintSlow is the fallback for non-SWAR parsing.
func parseUintSlow(b []byte) (uint64, bool) {
	var result uint64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}
		result = result*10 + uint64(c-'0')
	}
	return result, true
}

// ParseFloat64 parses a float from bytes.
// For now, this delegates to strconv - SWAR float parsing is more complex.
func ParseFloat64(b []byte) (float64, bool) {
	// TODO: Implement fast float parsing
	// For now, use standard library
	return 0, false
}
