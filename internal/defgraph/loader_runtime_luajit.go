//go:build luajit && !windows

package defgraph

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"sc_cli/internal/collections"

	lua "github.com/aarzilli/golua/lua"
)

type clonedTableMetadata struct {
	ParentDefID Option[DefID]
	Baseline    LuaObject
}

type traversalScratch struct {
	visited *collections.HashSet[uintptr]
}

func newTraversalScratch() traversalScratch {
	return traversalScratch{visited: collections.NewHashSet[uintptr]()}
}

func (scratch *traversalScratch) reset() *collections.HashSet[uintptr] {
	if scratch.visited == nil {
		scratch.visited = collections.NewHashSet[uintptr]()
		return scratch.visited
	}

	scratch.visited.Clear()
	return scratch.visited
}

type runtime struct {
	L                      *lua.State
	repoRoot               string
	compiledRoot           string
	filesLoaded            []ScriptPath
	scriptStack            []ScriptPath
	loadedSet              *collections.OrderedSet[ScriptPath]
	defSourceFile          *collections.HashMap[DefID, ScriptPath]
	preUpgradeRequiredShip map[DefID]LuaValue
	warnings               []string
	tableMetadata          *collections.HashMap[uintptr, clonedTableMetadata]
	defTableIDs            *collections.HashMap[uintptr, DefID]
	traversal              traversalScratch
}

func LoadWorld(repoRoot string, compiledRoot string) (*World, error) {
	repoRoot = NormalizeRepoRoot(repoRoot)
	compiledRoot = NormalizeCompiledRoot(compiledRoot)

	rt := &runtime{
		L:             lua.NewState(),
		repoRoot:      repoRoot,
		compiledRoot:  compiledRoot,
		loadedSet:     collections.NewOrderedSet[ScriptPath](),
		defSourceFile: collections.NewHashMap[DefID, ScriptPath](),
		tableMetadata: collections.NewHashMap[uintptr, clonedTableMetadata](),
		defTableIDs:   collections.NewHashMap[uintptr, DefID](),
		traversal:     newTraversalScratch(),
	}
	defer rt.L.Close()

	rt.L.OpenLibs()
	if err := rt.setupEnvironment(); err != nil {
		return nil, err
	}

	for _, rel := range bootstrapScripts {
		if err := rt.doFile(rel); err != nil {
			return nil, fmt.Errorf("load %s: %w", rel, err)
		}
	}

	if !rt.loadedSet.Contains(ScriptPath(moduleUpgradeChainScript)) {
		if rt.preUpgradeRequiredShip == nil {
			rt.preUpgradeRequiredShip = rt.capturePreUpgradeRequiredShip()
		}
		if err := rt.doFile(moduleUpgradeChainScript); err != nil {
			return nil, fmt.Errorf("load %s: %w", moduleUpgradeChainScript, err)
		}
	}

	defs, err := rt.captureDefs()
	if err != nil {
		return nil, err
	}

	return &World{
		RepoRoot:               repoRoot,
		CompiledRoot:           compiledRoot,
		Loader:                 LoaderNameGoLua,
		LoaderRuntime:          LoaderRuntimeLuaJIT,
		FilesLoaded:            append([]ScriptPath(nil), rt.filesLoaded...),
		Warnings:               append([]string(nil), rt.warnings...),
		Defs:                   defs,
		Enums:                  rt.captureEnums(),
		PreUpgradeRequiredShip: rt.preUpgradeRequiredShip,
	}, nil
}

func (rt *runtime) setupEnvironment() error {
	rt.L.NewTable()
	rt.L.SetGlobal("Def")
	rt.L.NewTable()
	rt.L.SetGlobal("Spell")
	rt.L.NewTable()
	rt.L.SetGlobal("AccountAura")
	rt.L.NewTable()
	rt.L.SetGlobal("DefExtension")
	rt.L.NewTable()
	rt.L.SetGlobal("SpellExtension")
	rt.L.NewTable()
	rt.L.SetGlobal("ai")
	rt.L.NewTable()
	rt.L.SetGlobal("designers")
	rt.L.NewTable()
	rt.L.SetGlobal("VesselTree")
	rt.L.NewTable()
	rt.L.SetGlobal("DefItemPrice")
	rt.L.NewTable()
	rt.L.SetGlobal("UI")
	rt.L.NewTable()
	rt.L.SetGlobal("strings")
	rt.pushString("en")
	rt.L.SetGlobal("lang")

	if err := rt.setupDefTracking(); err != nil {
		return err
	}

	rt.setGlobalFunction("dprint", func(L *lua.State) int { return 0 })
	rt.setGlobalFunction("ScriptWriteFile", func(L *lua.State) int {
		L.PushBoolean(false)
		return 1
	})
	rt.setGlobalFunction("SystemGetGlobalObject", func(L *lua.State) int {
		return 0
	})
	rt.setGlobalFunction("IsAppInPrepareSharedDataMode", func(L *lua.State) int {
		L.PushBoolean(false)
		return 1
	})
	rt.setGlobalFunction("GetCvarValue", func(L *lua.State) int {
		switch L.ToString(1) {
		case "r_width":
			L.PushInteger(1920)
		case "r_height":
			L.PushInteger(1080)
		default:
			L.PushNil()
		}
		return 1
	})
	rt.setGlobalFunction("GetU64Time", func(L *lua.State) int {
		L.NewTable()
		L.PushInteger(1)
		L.SetField(-2, "l")
		L.PushInteger(1)
		L.SetField(-2, "h")
		return 1
	})
	rt.setGlobalFunction("IsPC", func(L *lua.State) int {
		L.PushBoolean(true)
		return 1
	})
	rt.setGlobalFunction("IsMac", func(L *lua.State) int {
		L.PushBoolean(false)
		return 1
	})
	rt.setGlobalFunction("IsLinux", func(L *lua.State) int {
		L.PushBoolean(false)
		return 1
	})
	rt.setGlobalFunction("IsX360", func(L *lua.State) int {
		L.PushBoolean(false)
		return 1
	})
	rt.setGlobalFunction("IsPS3", func(L *lua.State) int {
		L.PushBoolean(false)
		return 1
	})
	rt.setGlobalFunction("Vec3", rt.makeVectorFunction("Vec3", "x", "y", "z"))
	rt.setGlobalFunction("Vec2", rt.makeVectorFunction("Vec2", "x", "y"))
	rt.setGlobalFunction("Quat", rt.makeVectorFunction("Quat", "x", "y", "z", "w"))
	rt.setGlobalFunction("Color", rt.makeVectorFunction("Color", "r", "g", "b", "a"))
	rt.setGlobalFunction("u64", func(L *lua.State) int {
		if L.GetTop() == 0 {
			L.PushInteger(0)
			return 1
		}
		L.PushValue(1)
		return 1
	})

	rt.setupBit()
	if err := rt.setupSolid(); err != nil {
		return err
	}
	if err := rt.setupGameStore(); err != nil {
		return err
	}
	if err := rt.setupBootstrapTables(); err != nil {
		return err
	}
	if err := rt.setupSys(); err != nil {
		return err
	}
	if err := rt.installLuaUtilities(); err != nil {
		return err
	}

	rt.L.GetGlobal("ai")
	rt.pushString(DontInheritSentinel)
	rt.L.SetField(-2, "DONT_INHERIT")
	rt.L.Pop(1)

	return nil
}

func (rt *runtime) setupBootstrapTables() error {
	rt.L.GetGlobal("math")
	rt.setFieldFunction(-1, "mod", func(L *lua.State) int {
		L.PushNumber(math.Mod(rt.numberArg(L, 1), rt.numberArg(L, 2)))
		return 1
	})
	rt.L.Pop(1)

	rt.L.GetGlobal("UI")
	rt.L.NewTable()
	rt.L.SetField(-2, "Root")
	rt.setFieldFunction(-1, "GetStringByName", func(L *lua.State) int {
		name := L.ToString(2)

		L.GetGlobal("strings")
		if L.Type(-1) != lua.LUA_TTABLE {
			L.Pop(1)
			L.PushNil()
			return 1
		}

		L.GetField(-1, name)
		L.Remove(-2)
		if L.Type(-1) != lua.LUA_TTABLE {
			L.Pop(1)
			L.PushNil()
			return 1
		}

		L.GetGlobal("lang")
		lang := L.ToString(-1)
		L.Pop(1)
		L.GetField(-1, lang)
		L.Remove(-2)
		return 1
	})
	rt.L.Pop(1)

	rt.setGlobalFunction("GetStoreItemTypeByDef", func(L *lua.State) int {
		defID := DefID(L.ToString(1))
		fields, ok := rt.worldDefFields(defID)
		if !ok {
			L.PushNil()
			return 1
		}

		switch {
		case !fields.MustGet("module_type").IsNull():
			L.PushString("IT_MODULE")
		case boolFieldFromObject(fields, "is_drug"):
			L.PushString("IT_DRUG")
		case boolFieldFromObject(fields, "is_bundle"):
			L.PushString("IT_BUNDLE")
		case boolFieldFromObject(fields, "is_blueprint"):
			L.PushString("IT_BLUEPRINT")
		case boolFieldFromObject(fields, "is_resource"):
			L.PushString("IT_RESOURCE")
		case boolFieldFromObject(fields, "is_avatar"):
			L.PushString("IT_AVATAR")
		case boolFieldFromObject(fields, "is_junk"):
			L.PushString("IT_JUNK")
		default:
			L.PushNil()
		}

		return 1
	})

	return nil
}

func (rt *runtime) setupDefTracking() error {
	rt.L.GetGlobal("Def")
	defer rt.L.Pop(1)

	if rt.L.Type(-1) != lua.LUA_TTABLE {
		return fmt.Errorf("Def must be a table during runtime setup")
	}

	absolute := rt.absIndex(-1)
	rt.L.NewTable()
	rt.L.SetMetaMethod("__newindex", func(L *lua.State) int {
		if L.Type(2) == lua.LUA_TSTRING && L.Type(3) == lua.LUA_TTABLE {
			defID := DefID(rt.stackValueString(2))
			if sourceFile, ok := rt.currentScriptSource(); ok && !rt.defSourceFile.Contains(defID) {
				rt.defSourceFile.Put(defID, sourceFile)
			}
			rt.defTableIDs.Put(L.ToPointer(3), defID)
		}

		L.PushValue(2)
		L.PushValue(3)
		L.RawSet(1)
		return 0
	})
	rt.L.SetMetaTable(absolute)
	return nil
}

func (rt *runtime) setupBit() {
	rt.L.NewTable()
	rt.setFieldFunction(-1, "lshift", func(L *lua.State) int {
		left := int64(rt.numberArg(L, 1))
		right := uint(rt.integerArg(L, 2))
		L.PushInteger(left << right)
		return 1
	})
	rt.setFieldFunction(-1, "rshift", func(L *lua.State) int {
		left := int64(rt.numberArg(L, 1))
		right := uint(rt.integerArg(L, 2))
		L.PushInteger(int64(uint64(left) >> right))
		return 1
	})
	rt.setFieldFunction(-1, "band", func(L *lua.State) int {
		left := int64(rt.numberArg(L, 1))
		right := int64(rt.numberArg(L, 2))
		L.PushInteger(left & right)
		return 1
	})
	rt.setFieldFunction(-1, "bor", func(L *lua.State) int {
		left := int64(rt.numberArg(L, 1))
		right := int64(rt.numberArg(L, 2))
		L.PushInteger(left | right)
		return 1
	})
	rt.setFieldFunction(-1, "bxor", func(L *lua.State) int {
		left := int64(rt.numberArg(L, 1))
		right := int64(rt.numberArg(L, 2))
		L.PushInteger(left ^ right)
		return 1
	})
	rt.L.SetGlobal("bit")
}

func (rt *runtime) setupSolid() error {
	rt.L.NewTable()
	if err := rt.setDynamicStringIndex(-1); err != nil {
		return err
	}
	rt.L.SetGlobal("solid")
	return nil
}

func (rt *runtime) setupGameStore() error {
	rt.L.NewTable()
	if err := rt.setDynamicStringIndex(-1); err != nil {
		return err
	}
	rt.L.NewTable()
	if err := rt.setDynamicStringIndex(-1); err != nil {
		return err
	}
	rt.L.SetField(-2, "CreditsType")
	rt.L.NewTable()
	if err := rt.setDynamicStringIndex(-1); err != nil {
		return err
	}
	rt.L.SetField(-2, "ItemType")
	rt.L.SetGlobal("GameStore")
	return nil
}

func (rt *runtime) setupSys() error {
	rt.L.NewTable()
	rt.setFieldFunction(-1, "execscript", func(L *lua.State) int {
		path := L.ToString(1)
		normalized := ScriptPath(normalizeLogicalPath(path))
		if normalized == ScriptPath(moduleUpgradeChainScript) && rt.preUpgradeRequiredShip == nil {
			rt.preUpgradeRequiredShip = rt.capturePreUpgradeRequiredShip()
		}
		if err := rt.doFile(normalized.String()); err != nil {
			panic(fmt.Errorf("execscript %s: %w", normalized, err))
		}
		return 0
	})
	rt.setFieldFunction(-1, "new", func(L *lua.State) int {
		parent := rt.parentDefIDFromValue(1)
		if parent.IsAbsent() {
			L.GetGlobal("__defgraph_clone_table")
			L.PushValue(1)
			L.Call(1, 1)
			return 1
		}

		if err := rt.cloneValue(1, rt.beginTraversal()); err != nil {
			panic(err)
		}
		if L.IsTable(-1) {
			ptr := L.ToPointer(-1)
			meta := clonedTableMetadata{
				ParentDefID: parent,
				Baseline:    rt.tableToObject(-1, rt.beginTraversal()),
			}
			rt.tableMetadata.Put(ptr, meta)
		}
		return 1
	})
	rt.setFieldFunction(-1, "mergeTable", func(L *lua.State) int {
		if !L.IsTable(1) || !L.IsTable(2) {
			panic(fmt.Errorf("sys.mergeTable expects two tables"))
		}
		if err := rt.mergeTables(1, 2); err != nil {
			panic(err)
		}
		L.PushValue(1)
		return 1
	})
	rt.setFieldFunction(-1, "logError", func(L *lua.State) int {
		message := L.ToString(1)
		if extra := L.GetTop(); extra > 1 {
			values := make([]string, 0, extra)
			for index := 2; index <= extra; index++ {
				values = append(values, L.ToString(index))
			}
			message = fmt.Sprintf("%s %s", message, strings.Join(values, " "))
		}
		rt.warnings = append(rt.warnings, strings.TrimSpace(message))
		return 0
	})
	rt.setFieldFunction(-1, "log", func(L *lua.State) int {
		return 0
	})
	rt.setFieldFunction(-1, "out", func(L *lua.State) int {
		return 0
	})
	rt.L.SetGlobal("sys")
	return nil
}

func (rt *runtime) installLuaUtilities() error {
	return rt.L.DoString(`
local function defgraph_clone_table_impl(value, seen)
	if type(value) ~= "table" then
		return value
	end

	if seen[value] ~= nil then
		return seen[value]
	end

	local copy = {}
	seen[value] = copy

	for key, item in pairs(value) do
		copy[defgraph_clone_table_impl(key, seen)] = defgraph_clone_table_impl(item, seen)
	end

	local mt = getmetatable(value)
	if mt ~= nil then
		setmetatable(copy, mt)
	end

	return copy
end

function __defgraph_clone_table(value)
	return defgraph_clone_table_impl(value, {})
end

function pairsSorted(t, cmp)
	local keys = {}
	for key in pairs(t) do
		table.insert(keys, key)
	end

	if cmp == nil then
		cmp = function(a, b)
			local ta = type(a)
			local tb = type(b)
			if ta ~= tb then
				return ta < tb
			end
			if ta == "string" or ta == "number" then
				return a < b
			end
			return tostring(a) < tostring(b)
		end
	end

	table.sort(keys, cmp)

	local index = 0
	return function()
		index = index + 1
		local key = keys[index]
		if key == nil then
			return nil
		end
		return key, t[key]
	end
end
`)
}

func (rt *runtime) setDynamicStringIndex(index int) error {
	absolute := rt.absIndex(index)
	rt.L.NewTable()
	rt.L.SetMetaMethod("__index", func(L *lua.State) int {
		key := L.ToString(2)
		L.PushString(key)
		L.PushString(key)
		L.SetTable(1)
		L.PushString(key)
		return 1
	})
	rt.L.SetMetaTable(absolute)
	return nil
}

func (rt *runtime) makeVectorFunction(kind string, keys ...string) lua.LuaGoFunction {
	return func(L *lua.State) int {
		L.NewTable()
		rt.pushString(kind)
		L.SetField(-2, "__type")
		for index, key := range keys {
			rt.pushLuaValue(index + 1)
			L.SetField(-2, key)
		}
		return 1
	}
}

func (rt *runtime) doFile(rel string) error {
	normalized := ScriptPath(normalizeLogicalPath(rel))
	if rt.loadedSet.Contains(normalized) {
		return nil
	}

	path := ResolveCompiledPath(rt.compiledRoot, normalized.String())
	if _, err := os.Stat(path); err != nil {
		return err
	}

	rt.scriptStack = append(rt.scriptStack, normalized)
	defer func() {
		rt.scriptStack = rt.scriptStack[:len(rt.scriptStack)-1]
	}()

	if err := rt.L.DoFile(path); err != nil {
		if strings.Contains(err.Error(), "cannot load incompatible bytecode") {
			return fmt.Errorf("%w (logical path: %s, runtime: %s). the linked LuaJIT build cannot execute this chunk; for this project you need the exact game-era LuaJIT runtime, not an arbitrary distro package", err, normalized, rt.runtimeDescriptor())
		}
		return err
	}

	rt.loadedSet.Add(normalized)
	rt.filesLoaded = append(rt.filesLoaded, normalized)
	return nil
}

func (rt *runtime) runtimeDescriptor() string {
	rt.L.GetGlobal("jit")
	defer rt.L.Pop(1)

	if rt.L.Type(-1) != lua.LUA_TTABLE {
		return "unknown"
	}

	version := rt.tableStringField(-1, "version")
	osName := rt.tableStringField(-1, "os")
	arch := rt.tableStringField(-1, "arch")

	parts := make([]string, 0, 3)
	if version != "" {
		parts = append(parts, version)
	}
	if osName != "" {
		parts = append(parts, osName)
	}
	if arch != "" {
		parts = append(parts, arch)
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, "/")
}

func (rt *runtime) tableStringField(index int, field string) string {
	absolute := rt.absIndex(index)
	rt.L.GetField(absolute, field)
	defer rt.L.Pop(1)

	if rt.L.IsNoneOrNil(-1) {
		return ""
	}

	return strings.TrimSpace(rt.L.ToString(-1))
}

func (rt *runtime) capturePreUpgradeRequiredShip() map[DefID]LuaValue {
	out := map[DefID]LuaValue{}
	rt.withGlobalTable("Def", func(index int) {
		rt.forEachTable(index, func(keyIndex int, valueIndex int) {
			if rt.L.Type(keyIndex) != lua.LUA_TSTRING || rt.L.Type(valueIndex) != lua.LUA_TTABLE {
				return
			}
			defID := DefID(rt.stackValueString(keyIndex))
			rt.L.GetField(valueIndex, "required_ship")
			defer rt.L.Pop(1)
			if rt.L.IsNoneOrNil(-1) {
				return
			}
			out[defID] = rt.luaValue(-1, rt.beginTraversal())
		})
	})
	return out
}

func (rt *runtime) captureDefs() (map[DefID]RawDef, error) {
	out := map[DefID]RawDef{}
	rt.withGlobalTable("Def", func(index int) {
		rt.forEachTable(index, func(keyIndex int, valueIndex int) {
			if rt.L.Type(keyIndex) != lua.LUA_TSTRING || rt.L.Type(valueIndex) != lua.LUA_TTABLE {
				return
			}

			id := DefID(rt.stackValueString(keyIndex))
			fields := rt.tableToObject(valueIndex, rt.beginTraversal())
			ptr := rt.L.ToPointer(valueIndex)
			meta, hasMeta := rt.tableMetadata.Get(ptr)

			inherit := None[DefID]()
			localFields := fields
			if hasMeta {
				localFields = diffObjectFields(meta.Baseline, fields)
				inherit = meta.ParentDefID
			}
			if rawInherit, ok := fields.Get("inherit"); ok {
				if inherit.IsAbsent() {
					if value, present := rawInherit.AsString(); present && value != "" {
						inherit = Some(DefID(value))
					}
				}
			}

			sourceFile, _ := rt.defSourceFile.Get(id)
			out[id] = RawDef{
				ID:            id,
				SourceFile:    sourceFile,
				LocalFields:   localFields,
				InheritParent: inherit,
			}
		})
	})
	return out, nil
}

func (rt *runtime) captureEnums() EnumRegistry {
	registry := EnumRegistry{}

	rt.withGlobal("ItemSubtype", func() {
		registry.ItemSubtype = rt.enumFromStackTop()
	})

	rt.withGlobalTable("ai", func(_ int) {
		registry.ModuleType = rt.enumField("ModuleType")
		registry.ShipClass = rt.enumField("ShipClass")
		registry.ShipRoles = rt.enumField("ShipRoles")
		registry.WeaponClass = rt.enumField("WeaponClass")
		registry.Race = rt.enumField("Race")
		registry.RaceMask = rt.enumField("RaceMask")
		registry.SpaceShipModuleSlot = rt.enumField("SpaceShipModuleSlot")
	})

	return registry
}

func (rt *runtime) enumField(field string) EnumTable {
	rt.L.GetField(-1, field)
	defer rt.L.Pop(1)
	return rt.enumFromStackTop()
}

func (rt *runtime) enumFromStackTop() EnumTable {
	enum := EnumTable{
		ByName:  collections.NewHashMap[string, int64](),
		ByValue: collections.NewHashMap[int64, string](),
	}
	if rt.L.Type(-1) != lua.LUA_TTABLE {
		return enum
	}

	type entry struct {
		name  string
		value int64
	}
	entries := make([]entry, 0)
	rt.forEachTable(-1, func(keyIndex int, valueIndex int) {
		if rt.L.Type(keyIndex) != lua.LUA_TSTRING {
			return
		}

		value, ok := rt.luaInt64Value(valueIndex)
		if !ok {
			return
		}
		name := rt.stackValueString(keyIndex)
		enum.ByName.Put(name, value)
		entries = append(entries, entry{name: name, value: value})
	})

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].value == entries[j].value {
			return entries[i].name < entries[j].name
		}
		return entries[i].value < entries[j].value
	})

	for _, item := range entries {
		if _, ok := enum.ByValue.Get(item.value); ok {
			continue
		}
		enum.ByValue.Put(item.value, item.name)
	}

	return enum
}

func (rt *runtime) mergeTables(dstIndex int, srcIndex int) error {
	absDst := rt.absIndex(dstIndex)
	absSrc := rt.absIndex(srcIndex)
	rt.L.PushNil()
	for rt.L.Next(absSrc) != 0 {
		valueIndex := rt.absIndex(-1)
		keyIndex := rt.absIndex(-2)

		if rt.L.Type(valueIndex) == lua.LUA_TTABLE {
			rt.L.PushValue(keyIndex)
			rt.L.GetTable(absDst)
			if rt.L.Type(-1) == lua.LUA_TTABLE {
				if err := rt.mergeTables(-1, valueIndex); err != nil {
					rt.L.Pop(2)
					return err
				}
				rt.L.Pop(1)
				rt.L.Pop(1)
				continue
			}
			rt.L.Pop(1)
		}

		if err := rt.cloneValue(keyIndex, rt.beginTraversal()); err != nil {
			rt.L.Pop(1)
			return err
		}
		if err := rt.cloneValue(valueIndex, rt.beginTraversal()); err != nil {
			rt.L.Pop(2)
			return err
		}
		rt.L.SetTable(absDst)
		rt.L.Pop(1)
	}
	return nil
}

func (rt *runtime) cloneValue(index int, seen *collections.HashSet[uintptr]) error {
	absolute := rt.absIndex(index)
	switch rt.L.Type(absolute) {
	case lua.LUA_TNIL:
		rt.L.PushNil()
	case lua.LUA_TBOOLEAN:
		rt.L.PushBoolean(rt.L.ToBoolean(absolute))
	case lua.LUA_TNUMBER:
		text := rt.stackValueString(absolute)
		number, isInteger, ok := parseLuaNumber(text)
		if !ok {
			rt.L.PushString(text)
			return nil
		}
		if isInteger {
			rt.L.PushInteger(int64(number))
		} else {
			rt.L.PushNumber(number)
		}
	case lua.LUA_TSTRING:
		rt.L.PushString(rt.stackValueString(absolute))
	case lua.LUA_TTABLE:
		ptr := rt.L.ToPointer(absolute)
		if seen.Contains(ptr) {
			rt.L.PushNil()
			return nil
		}
		seen.Add(ptr)
		defer seen.Remove(ptr)

		rt.L.NewTable()
		destIndex := rt.absIndex(-1)
		rt.L.PushNil()
		for rt.L.Next(absolute) != 0 {
			if err := rt.cloneValue(-2, seen); err != nil {
				rt.L.Pop(1)
				return err
			}
			if err := rt.cloneValue(-1, seen); err != nil {
				rt.L.Pop(2)
				return err
			}
			rt.L.SetTable(destIndex)
			rt.L.Pop(1)
		}

		if rt.L.GetMetaTable(absolute) {
			if err := rt.cloneValue(-1, seen); err != nil {
				rt.L.Pop(1)
				return err
			}
			rt.L.SetMetaTable(destIndex)
			rt.L.Pop(1)
		}
	default:
		rt.L.PushValue(absolute)
	}

	return nil
}

func (rt *runtime) luaValue(index int, visited *collections.HashSet[uintptr]) LuaValue {
	switch rt.L.Type(index) {
	case lua.LUA_TNIL:
		return LuaNull()
	case lua.LUA_TBOOLEAN:
		return LuaBool(rt.L.ToBoolean(index))
	case lua.LUA_TSTRING:
		return LuaString(rt.stackValueString(index))
	case lua.LUA_TNUMBER:
		number, isInteger, ok := rt.luaNumericValue(index)
		if !ok {
			return LuaString(rt.stackValueString(index))
		}
		if isInteger {
			return LuaInt(int64(number))
		}
		return LuaFloat(number)
	case lua.LUA_TTABLE:
		return LuaObjectValue(rt.tableToObject(index, visited))
	case lua.LUA_TUSERDATA:
		number, isInteger, ok := rt.luaNumericValue(index)
		if !ok {
			return LuaString(rt.stackValueString(index))
		}
		if isInteger {
			return LuaInt(int64(number))
		}
		return LuaFloat(number)
	default:
		return LuaString(rt.stackValueString(index))
	}
}

func (rt *runtime) tableToObject(index int, visited *collections.HashSet[uintptr]) LuaObject {
	absolute := rt.absIndex(index)
	ptr := rt.L.ToPointer(absolute)
	if visited.Contains(ptr) {
		return NewLuaObject()
	}
	visited.Add(ptr)
	defer visited.Remove(ptr)

	out := NewLuaObject()
	rt.forEachTable(absolute, func(keyIndex int, valueIndex int) {
		out.Set(rt.luaKeyToString(keyIndex), rt.luaValue(valueIndex, visited))
	})
	return out
}

func (rt *runtime) luaKeyToString(index int) string {
	absolute := rt.absIndex(index)
	switch rt.L.Type(absolute) {
	case lua.LUA_TSTRING:
		return rt.stackValueString(absolute)
	case lua.LUA_TNUMBER:
		number, isInteger, ok := rt.luaNumericValue(absolute)
		if !ok {
			return rt.stackValueString(absolute)
		}
		if isInteger {
			return strconv.FormatInt(int64(number), 10)
		}
		return strconv.FormatFloat(number, 'f', -1, 64)
	case lua.LUA_TUSERDATA:
		number, isInteger, ok := rt.luaNumericValue(absolute)
		if !ok {
			return rt.stackValueString(absolute)
		}
		if isInteger {
			return strconv.FormatInt(int64(number), 10)
		}
		return strconv.FormatFloat(number, 'f', -1, 64)
	default:
		return rt.stackValueString(absolute)
	}
}

func (rt *runtime) parentDefIDFromValue(index int) Option[DefID] {
	if rt.L.Type(index) != lua.LUA_TTABLE {
		return None[DefID]()
	}
	if defID, ok := rt.defTableIDs.Get(rt.L.ToPointer(index)); ok {
		return Some(defID)
	}
	return None[DefID]()
}

func (rt *runtime) withGlobal(name string, fn func()) {
	rt.L.GetGlobal(name)
	defer rt.L.Pop(1)
	fn()
}

func (rt *runtime) withGlobalTable(name string, fn func(index int)) {
	rt.L.GetGlobal(name)
	defer rt.L.Pop(1)
	if rt.L.Type(-1) != lua.LUA_TTABLE {
		return
	}
	fn(rt.absIndex(-1))
}

func (rt *runtime) forEachTable(index int, fn func(keyIndex int, valueIndex int)) {
	absolute := rt.absIndex(index)
	rt.L.PushNil()
	for rt.L.Next(absolute) != 0 {
		stackTop := rt.L.GetTop()
		keyIndex := rt.absIndex(-2)
		valueIndex := rt.absIndex(-1)
		rt.L.PushValue(keyIndex)
		rt.L.PushValue(valueIndex)
		fn(rt.absIndex(-2), rt.absIndex(-1))
		rt.L.SetTop(stackTop - 1)
	}
}

func (rt *runtime) absIndex(index int) int {
	if index > 0 {
		return index
	}
	return rt.L.GetTop() + index + 1
}

func (rt *runtime) beginTraversal() *collections.HashSet[uintptr] {
	return rt.traversal.reset()
}

func (rt *runtime) currentScriptSource() (ScriptPath, bool) {
	if len(rt.scriptStack) == 0 {
		return "", false
	}

	return rt.scriptStack[len(rt.scriptStack)-1], true
}

func (rt *runtime) setGlobalFunction(name string, fn lua.LuaGoFunction) {
	rt.L.PushGoClosure(fn)
	rt.L.SetGlobal(name)
}

func (rt *runtime) setFieldFunction(index int, name string, fn lua.LuaGoFunction) {
	absolute := rt.absIndex(index)
	rt.L.PushGoClosure(fn)
	rt.L.SetField(absolute, name)
}

func (rt *runtime) pushString(value string) {
	rt.L.PushString(value)
}

func (rt *runtime) pushLuaValue(index int) {
	rt.L.PushValue(index)
}

func (rt *runtime) stackValueString(index int) string {
	absolute := rt.absIndex(index)
	rt.L.PushValue(absolute)
	defer rt.L.Pop(1)
	return rt.L.ToString(-1)
}

func (rt *runtime) numberArg(L *lua.State, index int) float64 {
	number, _, ok := rt.luaNumericValue(index)
	if !ok {
		return 0
	}
	return number
}

func (rt *runtime) integerArg(L *lua.State, index int) int64 {
	number, _, ok := rt.luaNumericValue(index)
	if !ok {
		return 0
	}
	return int64(number)
}

func (rt *runtime) luaNumericValue(index int) (float64, bool, bool) {
	absolute := rt.absIndex(index)

	switch rt.L.Type(absolute) {
	case lua.LUA_TNUMBER:
		return parseLuaNumber(rt.stackValueString(absolute))
	case lua.LUA_TUSERDATA:
		number, ok := rt.luaU64Value(absolute)
		if !ok {
			return 0, false, false
		}
		return number, math.Trunc(number) == number, true
	default:
		return 0, false, false
	}
}

func (rt *runtime) worldDefFields(id DefID) (LuaObject, bool) {
	rt.L.GetGlobal("Def")
	defer rt.L.Pop(1)

	if rt.L.Type(-1) != lua.LUA_TTABLE {
		return LuaObject{}, false
	}

	rt.L.GetField(-1, id.String())
	defer rt.L.Pop(1)

	if rt.L.Type(-1) != lua.LUA_TTABLE {
		return LuaObject{}, false
	}

	return rt.tableToObject(-1, rt.beginTraversal()), true
}

func boolFieldFromObject(object LuaObject, key string) bool {
	value, ok := object.Get(key)
	if !ok {
		return false
	}

	flag, ok := value.AsBool()
	return ok && flag
}

func (rt *runtime) luaInt64Value(index int) (int64, bool) {
	number, _, ok := rt.luaNumericValue(index)
	if !ok {
		return 0, false
	}
	return int64(number), true
}

func (rt *runtime) luaU64Value(index int) (float64, bool) {
	absolute := rt.absIndex(index)
	if rt.L.Type(absolute) != lua.LUA_TUSERDATA {
		return 0, false
	}
	if !rt.luaMetatableBooleanField(absolute, "__defgraph_u64") {
		return 0, false
	}

	rt.L.GetField(absolute, "value")
	defer rt.L.Pop(1)

	value, _, ok := parseLuaNumber(rt.L.ToString(-1))
	if !ok {
		return 0, false
	}
	return value, true
}

func (rt *runtime) luaMetatableBooleanField(index int, field string) bool {
	absolute := rt.absIndex(index)
	if !rt.L.GetMetaTable(absolute) {
		return false
	}
	defer rt.L.Pop(1)

	rt.L.GetField(-1, field)
	defer rt.L.Pop(1)

	return rt.L.ToBoolean(-1)
}

func parseLuaNumber(text string) (float64, bool, bool) {
	if strings.TrimSpace(text) == "" {
		return 0, false, false
	}

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, false, false
	}

	if math.Trunc(value) == value {
		return value, true, true
	}

	return value, false, true
}
