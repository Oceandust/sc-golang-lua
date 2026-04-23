package luavalue

import (
	"encoding/json"
	"testing"
)

func TestObjectMarshalPreservesInsertionOrder(t *testing.T) {
	object := NewObject()
	object.Set("b", Int(1))
	object.Set("a", String("x"))

	data, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("marshal object: %v", err)
	}

	if string(data) != `{"b":1,"a":"x"}` {
		t.Fatalf("marshal object = %s", data)
	}
}

func TestValueFromAnyPreservesNumbers(t *testing.T) {
	intValue, err := FromAny(float64(7))
	if err != nil {
		t.Fatalf("from any int-like float: %v", err)
	}
	if value, ok := intValue.AsInt64(); !ok || value != 7 {
		t.Fatalf("int-like float converted to (%d, %v)", value, ok)
	}

	floatValue, err := FromAny(float64(7.5))
	if err != nil {
		t.Fatalf("from any float: %v", err)
	}
	if value, ok := floatValue.AsFloat64(); !ok || value != 7.5 {
		t.Fatalf("float converted to (%f, %v)", value, ok)
	}
}

func TestObjectCloneIsIndependent(t *testing.T) {
	original := NewObject()
	original.Set("tier", Int(5))

	cloned := original.Clone()
	cloned.Set("tier", Int(6))

	originalValue, _ := original.Get("tier")
	clonedValue, _ := cloned.Get("tier")

	originalTier, _ := originalValue.AsInt64()
	clonedTier, _ := clonedValue.AsInt64()

	if originalTier != 5 {
		t.Fatalf("original tier = %d", originalTier)
	}
	if clonedTier != 6 {
		t.Fatalf("cloned tier = %d", clonedTier)
	}
}
