package defgraph

import "testing"

func BenchmarkDiffObjects(b *testing.B) {
	baseline := benchmarkObject(3, 6)
	current := baseline.Clone()

	nested, _ := current.MustGet("node_0").AsObject()
	nested.Set("extra_flag", LuaBool(true))
	current.Set("new_string", LuaString("SNIPER"))

	b.ReportAllocs()
	for b.Loop() {
		result := DiffLuaObjects(baseline, current)
		if result.Len() == 0 {
			b.Fatal("unexpected empty diff")
		}
	}
}

func BenchmarkMergeObjects(b *testing.B) {
	parent := benchmarkObject(3, 6)
	child := parent.Clone()

	nested, _ := child.MustGet("node_1").AsObject()
	nested.Set("damage_bonus", LuaFloat(7.5))
	child.Set("inherit_only", LuaString("DONT_INHERIT"))

	b.ReportAllocs()
	for b.Loop() {
		result := MergeLuaObjects(parent, child, func(value LuaValue) bool {
			text, ok := value.AsString()
			return ok && text == "DONT_INHERIT"
		})
		if result.Len() == 0 {
			b.Fatal("unexpected empty merge")
		}
	}
}

func benchmarkObject(depth int, width int) LuaObject {
	object := NewLuaObject()
	for index := 0; index < width; index++ {
		object.Set("value_"+string(rune('a'+index)), LuaInt(int64(index)))
	}

	if depth == 0 {
		return object
	}

	for index := 0; index < width; index++ {
		object.Set("node_"+string(rune('0'+index)), LuaObjectValue(benchmarkObject(depth-1, width)))
	}

	return object
}
