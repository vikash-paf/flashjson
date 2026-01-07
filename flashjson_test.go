package flashjson

import (
	"bytes"
	"strings"
	"testing"
)

type SimpleUser struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type ComplexUser struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Email  string   `json:"email"`
	Active bool     `json:"active"`
	Score  float64  `json:"score"`
	Tags   []string `json:"tags,omitempty"`
}

func TestUnmarshalSimple(t *testing.T) {
	input := []byte(`{"name":"Alice","age":30}`)
	var user SimpleUser

	err := Unmarshal(input, &user)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if user.Name != "Alice" {
		t.Errorf("expected Name='Alice', got %q", user.Name)
	}
	if user.Age != 30 {
		t.Errorf("expected Age=30, got %d", user.Age)
	}
}

func TestUnmarshalComplex(t *testing.T) {
	input := []byte(`{
		"id": 12345,
		"name": "Bob",
		"email": "bob@example.com",
		"active": true,
		"score": 95.5
	}`)

	var user ComplexUser
	err := Unmarshal(input, &user)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if user.ID != 12345 {
		t.Errorf("expected ID=12345, got %d", user.ID)
	}
	if user.Name != "Bob" {
		t.Errorf("expected Name='Bob', got %q", user.Name)
	}
	if user.Email != "bob@example.com" {
		t.Errorf("expected Email='bob@example.com', got %q", user.Email)
	}
	if !user.Active {
		t.Error("expected Active=true")
	}
	if user.Score != 95.5 {
		t.Errorf("expected Score=95.5, got %f", user.Score)
	}
}

func TestUnmarshalToInterface(t *testing.T) {
	input := []byte(`{"key":"value","num":42}`)
	var result interface{}

	err := Unmarshal(input, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	if m["key"] != "value" {
		t.Errorf("expected key='value', got %v", m["key"])
	}
}

func TestMarshalSimple(t *testing.T) {
	user := SimpleUser{Name: "Alice", Age: 30}

	data, err := Marshal(user)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Check it contains expected fields
	s := string(data)
	if !strings.Contains(s, `"name":"Alice"`) {
		t.Errorf("expected name field, got %s", s)
	}
	if !strings.Contains(s, `"age":30`) {
		t.Errorf("expected age field, got %s", s)
	}
}

func TestMarshalComplex(t *testing.T) {
	user := ComplexUser{
		ID:     123,
		Name:   "Test",
		Email:  "test@test.com",
		Active: true,
		Score:  99.9,
	}

	data, err := Marshal(user)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	s := string(data)
	if !strings.Contains(s, `"id":123`) {
		t.Errorf("expected id field, got %s", s)
	}
	if !strings.Contains(s, `"active":true`) {
		t.Errorf("expected active field, got %s", s)
	}

	// Tags should be omitted (omitempty and nil)
	if strings.Contains(s, `"tags"`) {
		t.Errorf("tags should be omitted, got %s", s)
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{`{}`, true},
		{`{"key":"value"}`, true},
		{`[1,2,3]`, true},
		{`null`, true},
		{`true`, true},
		{`123`, true},
		{`{invalid}`, false},
		{``, false},
		{`{"key":}`, false},
	}

	for _, tc := range tests {
		got := Valid([]byte(tc.input))
		if got != tc.valid {
			t.Errorf("Valid(%q) = %v, want %v", tc.input, got, tc.valid)
		}
	}
}

func TestDecoder(t *testing.T) {
	input := `{"name":"Stream","age":25}`
	dec := NewDecoder(strings.NewReader(input))

	var user SimpleUser
	err := dec.Decode(&user)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if user.Name != "Stream" {
		t.Errorf("expected Name='Stream', got %q", user.Name)
	}
}

func TestEncoder(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	user := SimpleUser{Name: "Encoded", Age: 100}
	err := enc.Encode(user)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	s := buf.String()
	if !strings.Contains(s, `"name":"Encoded"`) {
		t.Errorf("expected name field, got %s", s)
	}
}

func TestUnmarshalErrors(t *testing.T) {
	// Non-pointer
	var user SimpleUser
	err := Unmarshal([]byte(`{}`), user)
	if err == nil {
		t.Error("should error on non-pointer")
	}

	// Nil pointer
	err = Unmarshal([]byte(`{}`), nil)
	if err == nil {
		t.Error("should error on nil")
	}
}

// --- Benchmarks ---

func BenchmarkUnmarshalSmall(b *testing.B) {
	input := []byte(`{"name":"Alice","age":30}`)
	var user SimpleUser

	b.ResetTimer()
	b.SetBytes(int64(len(input)))

	for i := 0; i < b.N; i++ {
		_ = Unmarshal(input, &user)
	}
}

func BenchmarkMarshalSmall(b *testing.B) {
	user := SimpleUser{Name: "Alice", Age: 30}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = Marshal(user)
	}
}
