# FlashJSON

**The world's fastest JSON encoder/decoder for Go** with near-zero memory allocations.

> ⚠️ **Work in Progress** - This library is being built as an educational project to learn systems programming concepts.

> ⚠️ *Coded with Claude Opus 4.5*

## Features

- **Blazing Fast**: SIMD-accelerated parsing (AVX2/NEON)
- **Low Memory**: Near-zero GC pressure with arena allocators
- **Compatible**: Drop-in replacement for `encoding/json`
- **Stable**: Extensively fuzzed and tested
- **Educational**: Comprehensive documentation explaining every concept

## Installation

```bash
go get github.com/vikash-paf/flashjson
```

## Usage

```go
package main

import (
    "fmt"
    
    json "github.com/vikash-paf/flashjson"
)

type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

func main() {
    // Marshal
    user := User{Name: "Alice", Age: 30}
    data, err := json.Marshal(user)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(data))
    
    // Unmarshal
    var result User
    err = json.Unmarshal(data, &result)
    if err != nil {
        panic(err)
    }
    fmt.Printf("%+v\n", result)
}
```

## Benchmarks

```sh
go test -v -bench=. -benchmem -run=^$ . 2>&1 | tee bench.txt

cat bench.txt
```

## Performance Targets

| Operation | encoding/json | FlashJSON | Speedup |
|-----------|--------------|-----------|---------|
| Unmarshal (small) | 500ns | 50ns | 10x |
| Marshal (small) | 300ns | 40ns | 7x |
| Unmarshal (1KB) | 5μs | 500ns | 10x |
| [Done] Allocations | 50-200/op | 0-2/op | 100x |

## Architecture

FlashJSON uses a four-layer architecture:

```
Layer 0: SIMD Indexer     → Process 32-64 bytes per cycle
Layer 1: Tape (Index)     → Flat structural representation
Layer 2: OpCode VM        → Cached type-to-struct mapping
Layer 3: Public API       → encoding/json compatible interface
```

See [Architecture Documentation](./docs/architecture/01-overview.md) for details.

## Learning Resources

This project includes extensive documentation for learning systems programming:

- [CPU Fundamentals](./docs/concepts/01-cpu-fundamentals.md) - Pipeline, branch prediction, SIMD
- [Memory Management](./docs/concepts/02-memory-management.md) - GC, arena allocators
- [SIMD Deep Dive](./docs/concepts/03-simd-deep-dive.md) - AVX2/NEON programming

## Development Status

- [x] Phase 1: Foundation (Arena, Tape, Generic Indexer)
- [x] Phase 2: SIMD (AVX2, NEON)
- [x] Phase 3: Compatibility (Full encoding/json API)
- [x] Phase 4: Verification (Benchmarks, Fuzzing)

## Contributing

This is primarily an educational project. Contributions welcome, especially:
- Documentation improvements
- Test cases
- Benchmark scenarios

## License

MIT

## Acknowledgments

Inspired by:
- [simdjson](https://simdjson.org/) - SIMD JSON parsing concepts
- [Sonic](https://github.com/bytedance/sonic) - Go JSON performance
- [goccy/go-json](https://github.com/goccy/go-json) - OpCode approach
