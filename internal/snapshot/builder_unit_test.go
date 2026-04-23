package snapshot

import (
	"testing"

	"defgraph/internal/luavalue"
	"defgraph/internal/types"
)

func TestDetectVariantKind(t *testing.T) {
	fields := luavalue.NewObject()
	fields.Set("mark", luavalue.Int(7))

	value := detectVariantKind(types.DefID("Weapon_Test_T4_Epic"), fields)
	if value.OrElse("") != types.VariantKindEpic {
		t.Fatalf("variant kind = %q", value.OrElse(""))
	}
}

func TestMergeObjectsHonorsDontInherit(t *testing.T) {
	parent := luavalue.NewObject()
	parent.Set("material", luavalue.String("Metal"))
	parent.Set("tier", luavalue.Int(5))

	child := luavalue.NewObject()
	child.Set("material", luavalue.String(types.DontInheritSentinel))
	child.Set("rank", luavalue.Int(12))

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
