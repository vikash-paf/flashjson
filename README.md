# FlashJSON

A simple JSON encoding/decoding library wrapper for Go.

## Installation

```bash
go get github.com/vikash-paf/flashjson
```

## Usage

```go
package main

import (
    "fmt"
    "github.com/vikash-paf/flashjson"
)

func main() {
    // Marshal
    data := map[string]interface{}{
        "name": "John",
        "age":  30,
    }
    
    jsonBytes, err := flashjson.Marshal(data)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(jsonBytes))
    
    // Unmarshal
    var result map[string]interface{}
    err = flashjson.Unmarshal(jsonBytes, &result)
    if err != nil {
        panic(err)
    }
    fmt.Printf("%+v\n", result)
}
```

## API

- `Marshal(v interface{}) ([]byte, error)` - Encode Go value to JSON
- `Unmarshal(data []byte, v interface{}) error` - Decode JSON to Go value
- `NewEncoder(w io.Writer) *Encoder` - Create a JSON encoder
- `NewDecoder(r io.Reader) *Decoder` - Create a JSON decoder

## Testing

```bash
go test -v
```

## License

MIT
