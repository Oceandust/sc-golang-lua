package defgraph

import (
	"testing"
)

func TestDetectVariantKind(t *testing.T) {
	fields := NewLuaObject()
	fields.Set("mark", LuaInt(7))

	value := detectVariantKind(DefID("Weapon_Test_T4_Epic"), fields)
	if value.OrElse("") != VariantKindEpic {
		t.Fatalf("variant kind = %q", value.OrElse(""))
	}
}

func TestMergeObjectsHonorsDontInherit(t *testing.T) {
	parent := NewLuaObject()
	parent.Set("material", LuaString("Metal"))
	parent.Set("tier", LuaInt(5))

	child := NewLuaObject()
	child.Set("material", LuaString(DontInheritSentinel))
	child.Set("rank", LuaInt(12))

	merged := mergeObjects(parent, child)

	if _, ok := merged.Get("material"); ok {
		t.Fatal("expected dont-inherit field to be removed")
	}
	if rank, ok := merged.Get("rank"); !ok {
		t.Fatal("expected child field to be present")
	} else if value, ok := rank.AsInt64(); !ok || value != 12 {
		t.Fatalf("rank = %#v", rank)
	}
	if tier, ok := merged.Get("tier"); !ok {
		t.Fatal("expected inherited field to remain")
	} else if value, ok := tier.AsInt64(); !ok || value != 5 {
		t.Fatalf("tier = %#v", tier)
	}
}
