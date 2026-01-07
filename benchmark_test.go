package flashjson

import (
	"encoding/json"
	"testing"
)

// Benchmark comparison between FlashJSON and encoding/json

type BenchUser struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Age     int     `json:"age"`
	Active  bool    `json:"active"`
	Balance float64 `json:"balance"`
}

type BenchComplex struct {
	Users    []BenchUser `json:"users"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	Metadata struct {
		Created string `json:"created"`
		Version string `json:"version"`
	} `json:"metadata"`
}

var smallJSON = []byte(`{"id":12345,"name":"Alice Smith","email":"alice@example.com","age":30,"active":true,"balance":1234.56}`)

var mediumJSON = []byte(`{
	"users": [
		{"id":1,"name":"Alice","email":"alice@example.com","age":30,"active":true,"balance":100.50},
		{"id":2,"name":"Bob","email":"bob@example.com","age":25,"active":false,"balance":200.75},
		{"id":3,"name":"Charlie","email":"charlie@example.com","age":35,"active":true,"balance":300.25}
	],
	"total": 3,
	"page": 1,
	"metadata": {"created":"2024-01-01","version":"1.0"}
}`)

// Unmarshal benchmarks

func BenchmarkUnmarshalSmall_FlashJSON(b *testing.B) {
	var user BenchUser
	b.SetBytes(int64(len(smallJSON)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Unmarshal(smallJSON, &user)
	}
}

func BenchmarkUnmarshalSmall_StdLib(b *testing.B) {
	var user BenchUser
	b.SetBytes(int64(len(smallJSON)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = json.Unmarshal(smallJSON, &user)
	}
}

func BenchmarkUnmarshalMedium_FlashJSON(b *testing.B) {
	var data BenchComplex
	b.SetBytes(int64(len(mediumJSON)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Unmarshal(mediumJSON, &data)
	}
}

func BenchmarkUnmarshalMedium_StdLib(b *testing.B) {
	var data BenchComplex
	b.SetBytes(int64(len(mediumJSON)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = json.Unmarshal(mediumJSON, &data)
	}
}

// Marshal benchmarks

func BenchmarkMarshalSmall_FlashJSON(b *testing.B) {
	user := BenchUser{
		ID:      12345,
		Name:    "Alice Smith",
		Email:   "alice@example.com",
		Age:     30,
		Active:  true,
		Balance: 1234.56,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Marshal(user)
	}
}

func BenchmarkMarshalSmall_StdLib(b *testing.B) {
	user := BenchUser{
		ID:      12345,
		Name:    "Alice Smith",
		Email:   "alice@example.com",
		Age:     30,
		Active:  true,
		Balance: 1234.56,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(user)
	}
}

// Valid benchmark

func BenchmarkValid_FlashJSON(b *testing.B) {
	b.SetBytes(int64(len(mediumJSON)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Valid(mediumJSON)
	}
}

func BenchmarkValid_StdLib(b *testing.B) {
	b.SetBytes(int64(len(mediumJSON)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = json.Valid(mediumJSON)
	}
}
