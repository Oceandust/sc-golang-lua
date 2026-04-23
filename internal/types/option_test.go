package types

import (
	"encoding/json"
	"testing"
)

func TestOptionStringJSONRoundTrip(t *testing.T) {
	source := Some("laser")

	data, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("marshal option: %v", err)
	}
	if string(data) != `"laser"` {
		t.Fatalf("marshal option = %s", data)
	}

	var target Option[string]
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("unmarshal option: %v", err)
	}

	value, ok := target.Get()
	if !ok || value != "laser" {
		t.Fatalf("round-trip option = (%q, %v)", value, ok)
	}
}

func TestOptionInt64JSONRoundTrip(t *testing.T) {
	source := Some(int64(42))

	data, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("marshal option: %v", err)
	}
	if string(data) != "42" {
		t.Fatalf("marshal option = %s", data)
	}

	var target Option[int64]
	if err := json.Unmarshal(data, &target); err != nil {
		t.Fatalf("unmarshal option: %v", err)
	}

	value, ok := target.Get()
	if !ok || value != 42 {
		t.Fatalf("round-trip option = (%d, %v)", value, ok)
	}
}

func TestOptionNullJSONRoundTrip(t *testing.T) {
	var target Option[string]
	if err := json.Unmarshal([]byte("null"), &target); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if target.IsPresent() {
		t.Fatal("expected null option to be absent")
	}
}

func TestVariantKindJSONValidation(t *testing.T) {
	value := VariantKindEpic
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal variant kind: %v", err)
	}

	var decoded VariantKind
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal variant kind: %v", err)
	}
	if decoded != VariantKindEpic {
		t.Fatalf("variant kind = %q", decoded)
	}

	if err := json.Unmarshal([]byte(`"broken"`), &decoded); err == nil {
		t.Fatal("expected invalid variant kind to fail")
	}
}

func TestRecordKindJSONValidation(t *testing.T) {
	var kind RecordKind
	if err := json.Unmarshal([]byte(`"module"`), &kind); err != nil {
		t.Fatalf("unmarshal record kind: %v", err)
	}
	if kind != RecordKindModule {
		t.Fatalf("record kind = %q", kind)
	}

	if err := json.Unmarshal([]byte(`"wat"`), &kind); err == nil {
		t.Fatal("expected invalid record kind to fail")
	}
}
