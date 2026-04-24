package defgraph

import (
	"encoding/json"
	"testing"
)

func TestObjectMarshalPreservesInsertionOrder(t *testing.T) {
	object := NewLuaObject()
	object.Set("b", LuaInt(1))
	object.Set("a", LuaString("x"))

	data, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("marshal LuaObject: %v", err)
	}

	if string(data) != `{"b":1,"a":"x"}` {
		t.Fatalf("marshal LuaObject = %s", data)
	}
}

func TestValueFromAnyPreservesNumbers(t *testing.T) {
	intValue, err := LuaValueFromAny(float64(7))
	if err != nil {
		t.Fatalf("from any int-like float: %v", err)
	}
	if value, ok := intValue.AsInt64(); !ok || value != 7 {
		t.Fatalf("int-like float converted to (%d, %v)", value, ok)
	}

	floatValue, err := LuaValueFromAny(float64(7.5))
	if err != nil {
		t.Fatalf("from any float: %v", err)
	}
	if value, ok := floatValue.AsFloat64(); !ok || value != 7.5 {
		t.Fatalf("float converted to (%f, %v)", value, ok)
	}
}

func TestObjectCloneIsIndependent(t *testing.T) {
	original := NewLuaObject()
	original.Set("tier", LuaInt(5))

	cloned := original.Clone()
	cloned.Set("tier", LuaInt(6))

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

func TestBorrowedObjectAccessSharesNestedState(t *testing.T) {
	nested := NewLuaObject()
	nested.Set("tier", LuaInt(5))

	original := NewLuaObject()
	original.Set("nested", LuaObjectValue(nested))

	value, ok := original.Get("nested")
	if !ok {
		t.Fatal("missing nested LuaValue")
	}

	borrowed, ok := value.AsObject()
	if !ok {
		t.Fatalf("nested LuaValue is not LuaObject: %#v", value)
	}
	borrowed.Set("tier", LuaInt(6))

	nestedValue, ok := original.MustGet("nested").AsObject()
	if !ok {
		t.Fatal("missing nested LuaObject after mutation")
	}

	tier, ok := nestedValue.MustGet("tier").AsInt64()
	if !ok || tier != 6 {
		t.Fatalf("nested tier = %d, %v", tier, ok)
	}
}

func TestMergeDiffAndEqualHelpers(t *testing.T) {
	baseline := NewLuaObject()
	baselinePhysics := NewLuaObject()
	baselinePhysics.Set("static", LuaBool(true))
	baselinePhysics.Set("material", LuaString("Metal"))
	baseline.Set("physics", LuaObjectValue(baselinePhysics))
	baseline.Set("tier", LuaInt(5))

	current := baseline.Clone()
	currentPhysics, _ := current.MustGet("physics").AsObject()
	currentPhysics.Set("solid_type", LuaString("compound"))
	current.Set("role", LuaString("SNIPER"))

	diff := DiffLuaObjects(baseline, current)
	if diff.Len() != 2 {
		t.Fatalf("diff size = %d", diff.Len())
	}

	if !LuaValuesEqual(current.MustGet("role"), LuaString("SNIPER")) {
		t.Fatalf("role helper equality failed")
	}

	merged := MergeLuaObjects(baseline, diff, nil)
	if !LuaObjectsEqual(merged, current) {
		t.Fatalf("merged LuaObject does not match current")
	}
}
