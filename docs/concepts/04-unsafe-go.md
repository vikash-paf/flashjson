---
title: Working with unsafe in Go
order: 4
category: concepts
---

# Working with `unsafe` in Go

The `unsafe` package gives you direct memory access, bypassing Go's type safety. It's essential for high-performance code but dangerous if misused.

## Why We Need `unsafe`

Go's safety features have overhead:

```go
// Safe but slow: string conversion copies bytes
str := string(byteSlice)  // Allocates new memory, copies all bytes

// Safe but slow: interface boxing
var v interface{} = 42    // Allocates wrapper on heap
```

For FlashJSON, we need:
1. Zero-copy string extraction
2. Direct struct field access
3. Custom memory layouts

---

## Core `unsafe` Types

### unsafe.Pointer

A pointer that can point to anything and be cast to anything:

```go
import "unsafe"

func example() {
    var x int64 = 42
    
    // Get unsafe pointer to x
    ptr := unsafe.Pointer(&x)
    
    // Cast to different type (dangerous!)
    floatPtr := (*float64)(ptr)
    
    // Now we're interpreting int64 bits as float64
    fmt.Println(*floatPtr)  // Some weird float value
}
```

**Rules:**
- `unsafe.Pointer` can be converted to/from any `*T`
- `unsafe.Pointer` can be converted to/from `uintptr`
- You CANNOT do arithmetic on `unsafe.Pointer` directly

### uintptr

An integer large enough to hold any pointer:

```go
func pointerArithmetic() {
    data := []byte{1, 2, 3, 4, 5}
    
    // Get pointer to first element
    ptr := unsafe.Pointer(&data[0])
    
    // Convert to uintptr for arithmetic
    addr := uintptr(ptr)
    
    // Move to third element (index 2)
    addr += 2
    
    // Convert back to pointer
    thirdPtr := unsafe.Pointer(addr)
    
    // Cast to *byte and read
    value := *(*byte)(thirdPtr)
    fmt.Println(value)  // 3
}
```

**⚠️ WARNING:** Never store `uintptr` in a variable for later use! The GC might move the underlying data.

```go
// DANGEROUS - GC might move data between these lines
addr := uintptr(unsafe.Pointer(&data[0]))
// ... GC runs here, moves data ...
ptr := unsafe.Pointer(addr)  // Now points to garbage!

// SAFE - single expression
ptr := unsafe.Pointer(uintptr(unsafe.Pointer(&data[0])) + offset)
```

---

## unsafe.Sizeof, Alignof, Offsetof

### Sizeof

Returns the size of a type in bytes:

```go
fmt.Println(unsafe.Sizeof(int64(0)))   // 8
fmt.Println(unsafe.Sizeof(int32(0)))   // 4
fmt.Println(unsafe.Sizeof(byte(0)))    // 1
fmt.Println(unsafe.Sizeof("hello"))    // 16 (string header, not content!)
fmt.Println(unsafe.Sizeof([]byte{}))   // 24 (slice header)
```

### Alignof

Returns the required alignment of a type:

```go
fmt.Println(unsafe.Alignof(int64(0)))  // 8 (must be at 8-byte boundary)
fmt.Println(unsafe.Alignof(int32(0)))  // 4
fmt.Println(unsafe.Alignof(byte(0)))   // 1
```

### Offsetof

Returns the offset of a struct field:

```go
type User struct {
    ID   int64
    Name string
    Age  int32
}

fmt.Println(unsafe.Offsetof(User{}.ID))    // 0
fmt.Println(unsafe.Offsetof(User{}.Name))  // 8
fmt.Println(unsafe.Offsetof(User{}.Age))   // 24
```

---

## String and Slice Headers

Go strings and slices are just headers pointing to data:

```go
// From reflect package (conceptual)
type StringHeader struct {
    Data uintptr
    Len  int
}

type SliceHeader struct {
    Data uintptr
    Len  int
    Cap  int
}
```

### Zero-Copy Byte-to-String Conversion

```go
// SLOW: Copies all bytes
func bytesToStringSafe(b []byte) string {
    return string(b)
}

// FAST: Zero-copy, shares memory
func bytesToStringUnsafe(b []byte) string {
    return *(*string)(unsafe.Pointer(&b))
}
```

Visual representation:

```
Safe conversion (copies):
                                              
  []byte:  [ptr] [len] [cap]                  
              │                               
              ▼                               
           [H][e][l][l][o]  (original)        
                                   │          
                                   │ copy     
                                   ▼          
           [H][e][l][l][o]  (new memory)      
              ▲                               
              │                               
  string: [ptr] [len]                         


Unsafe conversion (shares):
                                              
  []byte:  [ptr] [len] [cap]                  
              │                               
              ▼                               
           [H][e][l][l][o]  (original)        
              ▲                               
              │                               
  string: [ptr] [len]    (same memory!)       
```

**⚠️ DANGER:** If the byte slice is modified, the string becomes garbage!

```go
b := []byte("hello")
s := bytesToStringUnsafe(b)
fmt.Println(s)  // "hello"

b[0] = 'X'
fmt.Println(s)  // "Xello" - string mysteriously changed!
```

### Safe Usage Pattern

```go
// For FlashJSON: input JSON is immutable during parse
func (d *Decoder) extractString(start, end int) string {
    if d.config.CopyStrings {
        // Safe: copy bytes
        return string(d.input[start:end])
    }
    // Fast: zero-copy (only if user knows input won't change)
    return bytesToStringUnsafe(d.input[start:end])
}
```

---

## Direct Struct Field Access

Instead of using reflection at runtime, we can compute field offsets once and use them directly:

```go
type User struct {
    Name string
    Age  int
}

// Computed once during "compilation" phase
var nameOffset = unsafe.Offsetof(User{}.Name)  // 0
var ageOffset = unsafe.Offsetof(User{}.Age)    // 16

func setUserName(ptr unsafe.Pointer, name string) {
    // Get pointer to Name field
    fieldPtr := unsafe.Pointer(uintptr(ptr) + nameOffset)
    
    // Cast to *string and set
    *(*string)(fieldPtr) = name
}

func setUserAge(ptr unsafe.Pointer, age int) {
    fieldPtr := unsafe.Pointer(uintptr(ptr) + ageOffset)
    *(*int)(fieldPtr) = age
}

func main() {
    user := User{}
    ptr := unsafe.Pointer(&user)
    
    setUserName(ptr, "Alice")
    setUserAge(ptr, 30)
    
    fmt.Printf("%+v\n", user)  // {Name:Alice Age:30}
}
```

**Why this matters for FlashJSON:**
- Reflection's `field.Set()` is slow
- We compute offsets once per type (cached)
- At parse time: direct pointer math

---

## Reading Values from Bytes

Parse JSON values directly from byte slice:

```go
// Read a uint64 from bytes (assuming little-endian)
func readUint64(b []byte) uint64 {
    return *(*uint64)(unsafe.Pointer(&b[0]))
}

// But JSON numbers are ASCII, so we need conversion
func parseIntFast(b []byte) int64 {
    // SWAR (SIMD Within A Register) technique
    // Process 8 ASCII digits in parallel
    // See implementation docs for details
}
```

---

## Safety Rules Summary

### ✅ Safe Patterns

```go
// 1. Type punning within single expression
str := *(*string)(unsafe.Pointer(&bytes))

// 2. Struct field access with known offsets
*(*int)(unsafe.Pointer(uintptr(ptr) + offset)) = value

// 3. Slice to array pointer (Go 1.17+)
arr := (*[4]byte)(slice)
```

### ❌ Dangerous Patterns

```go
// 1. Storing uintptr
addr := uintptr(ptr)  // GC can invalidate this!
// ... later ...
ptr = unsafe.Pointer(addr)  // Potentially garbage

// 2. Pointer to stack variable that escapes
func bad() unsafe.Pointer {
    x := 42
    return unsafe.Pointer(&x)  // x is on stack, will be garbage
}

// 3. Assuming struct layout
// Field order and padding can change between Go versions
```

---

## FlashJSON's unsafe Usage

### 1. Zero-Copy String Extraction

```go
func extractStringZeroCopy(data []byte, start, end int) string {
    slice := data[start:end]
    return *(*string)(unsafe.Pointer(&slice))
}
```

### 2. Direct Field Writing

```go
type fieldOp struct {
    offset uintptr
    typ    reflect.Kind
}

func (op *fieldOp) setString(structPtr unsafe.Pointer, value string) {
    *(*string)(unsafe.Pointer(uintptr(structPtr) + op.offset)) = value
}

func (op *fieldOp) setInt(structPtr unsafe.Pointer, value int64) {
    *(*int64)(unsafe.Pointer(uintptr(structPtr) + op.offset)) = value
}
```

### 3. Arena Allocation

```go
func (a *Arena) allocString(s string) string {
    b := a.Alloc(len(s))
    copy(b, s)
    return *(*string)(unsafe.Pointer(&b))
}
```

---

## Testing unsafe Code

Always test unsafe code thoroughly:

```go
func TestZeroCopyString(t *testing.T) {
    data := []byte(`"hello"`)
    
    // Extract without quotes
    s := extractStringZeroCopy(data, 1, 6)
    
    if s != "hello" {
        t.Errorf("expected 'hello', got '%s'", s)
    }
    
    // Verify it really shares memory (for debugging)
    dataPtr := uintptr(unsafe.Pointer(&data[1]))
    strHeader := (*reflect.StringHeader)(unsafe.Pointer(&s))
    
    if strHeader.Data != dataPtr {
        t.Error("string should share memory with slice")
    }
}
```

---

## Next Steps

- [../architecture/02-tape-design.md](../architecture/02-tape-design.md) - How we structure parsed JSON
- [../implementation/01-arena.md](../implementation/01-arena.md) - Build the arena allocator
