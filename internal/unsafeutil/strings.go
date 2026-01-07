// Package unsafe provides zero-copy string operations for high-performance JSON parsing.
// These functions bypass Go's type safety for maximum speed but require careful usage.
package unsafeutil

import (
	"unsafe"
)

// BytesToString converts a byte slice to a string without copying.
// WARNING: The returned string shares memory with the input slice.
// Modifying the slice will corrupt the string.
// Use only when:
// 1. The byte slice is immutable (e.g., from mmap)
// 2. The string will not outlive the slice
// 3. You need maximum performance
func BytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// Convert slice header to string header
	// SliceHeader: Data, Len, Cap
	// StringHeader: Data, Len
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// StringToBytes converts a string to a byte slice without copying.
// WARNING: The returned slice shares memory with the input string.
// Modifying the slice will cause undefined behavior (strings are immutable).
// Use only for reading operations.
func StringToBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// BytesToStringUnsafe is the legacy version using old-style header manipulation.
// Prefer BytesToString which uses Go 1.20+ safe-ish APIs.
func BytesToStringUnsafe(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringPtrOffset returns a string that points to a substring without copying.
// This is useful for extracting JSON values from a buffer.
func StringPtrOffset(base unsafe.Pointer, offset, length int) string {
	if length == 0 {
		return ""
	}
	ptr := unsafe.Pointer(uintptr(base) + uintptr(offset))
	return unsafe.String((*byte)(ptr), length)
}

// BytesPtrOffset returns a byte slice that points to a portion of memory.
func BytesPtrOffset(base unsafe.Pointer, offset, length int) []byte {
	if length == 0 {
		return nil
	}
	ptr := (*byte)(unsafe.Pointer(uintptr(base) + uintptr(offset)))
	return unsafe.Slice(ptr, length)
}
