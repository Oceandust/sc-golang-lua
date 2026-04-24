package defgraph

import (
	"testing"
)

func TestDiffObjectFields(t *testing.T) {
	baseline := NewLuaObject()
	baselinePhysics := NewLuaObject()
	baselinePhysics.Set("static", LuaBool(true))
	baselinePhysics.Set("material", LuaString("Metal"))
	baseline.Set("inherit", LuaString("Weapon_Base"))
	baseline.Set("physics", LuaObjectValue(baselinePhysics))
	baseline.Set("tier", LuaInt(5))

	current := NewLuaObject()
	currentPhysics := NewLuaObject()
	currentPhysics.Set("static", LuaBool(true))
	currentPhysics.Set("material", LuaString("Metal"))
	currentPhysics.Set("solid_type", LuaString("compound"))
	current.Set("inherit", LuaString("Weapon_Base"))
	current.Set("physics", LuaObjectValue(currentPhysics))
	current.Set("tier", LuaInt(5))
	current.Set("requiredRole", LuaString("SNIPER"))

	diff := diffObjectFields(baseline, current)
	if diff.Len() != 2 {
		t.Fatalf("diff size = %d, want 2", diff.Len())
	}

	roleValue, ok := diff.Get("requiredRole")
	if !ok {
		t.Fatalf("requiredRole diff missing")
	}
	if roleText, ok := roleValue.AsString(); !ok || roleText != "SNIPER" {
		t.Fatalf("requiredRole diff = %#v", roleValue)
	}

	physicsValue, ok := diff.Get("physics")
	if !ok {
		t.Fatalf("physics diff missing")
	}

	physics, ok := physicsValue.AsObject()
	if !ok {
		t.Fatalf("physics diff missing object: %#v", physicsValue)
	}

	solidType, ok := physics.Get("solid_type")
	if !ok {
		t.Fatalf("physics diff missing solid_type")
	}
	if text, ok := solidType.AsString(); !ok || text != "compound" {
		t.Fatalf("physics solid_type = %#v", solidType)
	}
	if _, ok := physics.Get("material"); ok {
		t.Fatalf("unexpected inherited material in diff")
	}
}
