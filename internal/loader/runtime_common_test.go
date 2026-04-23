package loader

import (
	"path/filepath"
	"reflect"
	"testing"

	"defgraph/internal/luavalue"
)

func TestLoadedManifest(t *testing.T) {
	got := LoadedManifest()
	want := []string{
		"scripts/ai/constants.lua",
		"scripts/ai/spellconstants.lua",
		"scripts/ai/cosmos_constants.lua",
		"scripts/ai/achievementconstants.lua",
		"scripts/ai/questconstants.lua",
		"scripts/ai/situation_globals.lua",
		"scripts/ai/adventure_globals.lua",
		"scripts/masterserver.lua",
		"gamedata/shared/item_subtypes.lua",
		"gamedata/def/designersdefs.lua",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadedManifest() = %#v, want %#v", got, want)
	}
}

func TestResolveCompiledPath(t *testing.T) {
	compiledRoot := filepath.Join("tmp", "compiled", "21_04_2026_snapshot")
	got := ResolveCompiledPath(compiledRoot, "gamedata/def/ex/laser.lua")
	want := filepath.Join(compiledRoot, "gamedata", "def", "ex", "laser.lua")
	if got != want {
		t.Fatalf("ResolveCompiledPath() = %q, want %q", got, want)
	}
}

func TestDiffObjectFields(t *testing.T) {
	baseline := luavalue.NewObject()
	baselinePhysics := luavalue.NewObject()
	baselinePhysics.Set("static", luavalue.Bool(true))
	baselinePhysics.Set("material", luavalue.String("Metal"))
	baseline.Set("inherit", luavalue.String("Weapon_Base"))
	baseline.Set("physics", luavalue.ObjectValue(baselinePhysics))
	baseline.Set("tier", luavalue.Int(5))

	current := luavalue.NewObject()
	currentPhysics := luavalue.NewObject()
	currentPhysics.Set("static", luavalue.Bool(true))
	currentPhysics.Set("material", luavalue.String("Metal"))
	currentPhysics.Set("solid_type", luavalue.String("compound"))
	current.Set("inherit", luavalue.String("Weapon_Base"))
	current.Set("physics", luavalue.ObjectValue(currentPhysics))
	current.Set("tier", luavalue.Int(5))
	current.Set("requiredRole", luavalue.String("SNIPER"))

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
