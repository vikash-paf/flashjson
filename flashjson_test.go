package flashjson

import (
	"bytes"
	"strings"
	"testing"
)

func TestMarshal(t *testing.T) {
	data := map[string]interface{}{
		"name": "test",
		"age":  25,
	}

	result, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("Marshal returned empty result")
	}
}

func TestUnmarshal(t *testing.T) {
	input := []byte(`{"name":"test","age":25}`)
	var result map[string]interface{}

	err := Unmarshal(input, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("Expected name=test, got %v", result["name"])
	}
}

func TestEncoder(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)

	data := map[string]string{"key": "value"}
	err := enc.Encode(data)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("Encoder wrote nothing")
	}
}

func TestDecoder(t *testing.T) {
	input := `{"key":"value"}`
	dec := NewDecoder(strings.NewReader(input))

	var result map[string]string
	err := dec.Decode(&result)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected key=value, got %v", result["key"])
	}
}
