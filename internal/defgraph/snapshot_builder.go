package defgraph

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/emirpasic/gods/sets/linkedhashset"
)

type resolvedDef struct {
	ID             DefID
	SourceFile     ScriptPath
	LocalFields    LuaObject
	ResolvedFields LuaObject
	InheritParent  Option[DefID]
	InheritChain   []DefID
	Kind           RecordKind
}

type resolver struct {
	world     *World
	resolved  map[DefID]resolvedDef
	resolving map[DefID]struct{}
}

func BuildSnapshot(world *World) (*Snapshot, error) {
	ids := make([]DefID, 0, len(world.Defs))
	for id := range world.Defs {
		ids = append(ids, id)
	}
	sortDefIDs(ids)

	resolver := resolver{
		world:     world,
		resolved:  make(map[DefID]resolvedDef, len(ids)),
		resolving: make(map[DefID]struct{}, len(ids)),
	}

	defs := make([]resolvedDef, 0, len(ids))
	for _, id := range ids {
		item, err := resolver.resolve(id)
		if err != nil {
			return nil, err
		}
		defs = append(defs, item)
	}

	defRecords := make([]DefRecord, 0, len(defs))
	for _, item := range defs {
		defRecords = append(defRecords, DefRecord{
			ID:             item.ID,
			Kind:           item.Kind,
			SourceFile:     item.SourceFile,
			InheritParent:  item.InheritParent,
			InheritChain:   append([]DefID(nil), item.InheritChain...),
			LocalFields:    item.LocalFields,
			ResolvedFields: item.ResolvedFields,
		})
	}

	ships := buildShips(world, defs)
	blueprints := buildBlueprints(defs, ships)
	blueprintRecipes := groupBlueprintsByCraftResult(blueprints)
	resources := buildResources(world, defs)
	modules := buildModules(world, defs, ships, blueprintRecipes)

	return &Snapshot{
		Meta: Meta{
			SchemaVersion: SchemaVersionV1,
			Loader:        world.Loader,
			LoaderRuntime: world.LoaderRuntime,
			RepoRoot:      world.RepoRoot,
			CompiledRoot:  world.CompiledRoot,
			LoadOrder:     append([]ScriptPath(nil), world.FilesLoaded...),
			LoadedFiles:   append([]ScriptPath(nil), world.FilesLoaded...),
		},
		Defs:          defRecords,
		Ships:         ships,
		Modules:       modules,
		Blueprints:    blueprints,
		Resources:     resources,
		UpgradeChains: buildUpgradeChains(modules),
		Compatibility: buildCompatibility(modules, ships),
		Warnings:      append([]string(nil), world.Warnings...),
	}, nil
}

func (resolver *resolver) resolve(id DefID) (resolvedDef, error) {
	if value, ok := resolver.resolved[id]; ok {
		return value, nil
	}

	if _, ok := resolver.resolving[id]; ok {
		return resolvedDef{}, fmt.Errorf("inheritance cycle at %s", id)
	}

	raw, ok := resolver.world.Defs[id]
	if !ok {
		return resolvedDef{}, fmt.Errorf("unknown def %s", id)
	}

	resolver.resolving[id] = struct{}{}
	defer delete(resolver.resolving, id)

	resolvedFields := NewLuaObject()
	inheritChain := make([]DefID, 0, 4)
	hasParent := false

	if parentID, ok := raw.InheritParent.Get(); ok {
		if _, exists := resolver.world.Defs[parentID]; exists {
			parentResolved, err := resolver.resolve(parentID)
			if err != nil {
				return resolvedDef{}, err
			}

			resolvedFields = parentResolved.ResolvedFields
			hasParent = true

			chainSet := linkedhashset.New()
			chainSet.Add(parentID)
			for _, ancestorID := range parentResolved.InheritChain {
				chainSet.Add(ancestorID)
			}

			for _, rawValue := range chainSet.Values() {
				inheritChain = append(inheritChain, rawValue.(DefID))
			}
		}
	}

	if hasParent {
		resolvedFields = mergeObjects(resolvedFields, raw.LocalFields)
	} else {
		resolvedFields = pruneDontInherit(raw.LocalFields)
	}

	value := resolvedDef{
		ID:             raw.ID,
		SourceFile:     raw.SourceFile,
		LocalFields:    raw.LocalFields,
		ResolvedFields: resolvedFields,
		InheritParent:  raw.InheritParent,
		InheritChain:   inheritChain,
		Kind:           classifyDef(resolvedFields),
	}

	resolver.resolved[id] = value
	return value, nil
}

func classifyDef(fields LuaObject) RecordKind {
	if boolField(fields, "is_blueprint") {
		return RecordKindBlueprint
	}

	if boolField(fields, "is_resource") {
		return RecordKindResource
	}

	if fieldString(fields, "class").OrElse("") == "SpaceShip" {
		return RecordKindShip
	}

	if fields.MustGet("module_type").IsNull() && fields.MustGet("weapon_class").IsNull() {
		className := fieldString(fields, "class").OrElse("")
		switch className {
		case "SpaceShipModule", "EngineModule", "ActiveModule":
			return RecordKindModule
		default:
			return RecordKindOther
		}
	}

	return RecordKindModule
}

func buildShips(world *World, defs []resolvedDef) []ShipRecord {
	records := make([]ShipRecord, 0)
	for _, item := range defs {
		if item.Kind != RecordKindShip {
			continue
		}

		if fieldString(item.ResolvedFields, "ship_name").IsAbsent() {
			continue
		}
		if fieldInt64(item.ResolvedFields, "rank").IsAbsent() {
			continue
		}
		if fieldInt64(item.ResolvedFields, "role").IsAbsent() {
			continue
		}

		records = append(records, ShipRecord{
			ID:            ShipID(item.ID),
			SourceFile:    item.SourceFile,
			InheritParent: item.InheritParent,
			InheritChain:  append([]DefID(nil), item.InheritChain...),
			ShipName:      fieldString(item.ResolvedFields, "ship_name"),
			ShipTier:      fieldInt64(item.ResolvedFields, "ship_tier"),
			Rank:          fieldInt64(item.ResolvedFields, "rank"),
			Role: mapOption(enumField(world.Enums.ShipRoles, item.ResolvedFields, "role"), func(value string) ShipRoleName {
				return ShipRoleName(value)
			}),
			ShipClass: mapOption(enumField(world.Enums.ShipClass, item.ResolvedFields, "ship_class"), func(value string) ShipClassName {
				return ShipClassName(value)
			}),
			Race: mapOption(enumField(world.Enums.Race, item.ResolvedFields, "race"), func(value string) RaceName {
				return RaceName(value)
			}),
			IsPremium:      boolField(item.ResolvedFields, "isPremium"),
			DefaultModules: normalizeDefaultModules(item.ResolvedFields.MustGet("default_modules"), world.Enums.SpaceShipModuleSlot),
			SlotTypes:      normalizeSlotTypes(item.ResolvedFields.MustGet("slot_module_types"), world.Enums.SpaceShipModuleSlot, world.Enums.ModuleType),
			Economy: EconomyInfo{
				Purchase: purchaseFromFields(item.ResolvedFields),
				Crafting: CraftingInfo{
					DirectIngredients: normalizeIngredients(item.ResolvedFields.MustGet("craftable_reagents")),
				},
				RecraftCredits: fieldInt64(item.ResolvedFields, "recraft_credits"),
			},
		})
	}

	sort.Slice(records, func(left int, right int) bool {
		return records[left].ID < records[right].ID
	})

	return records
}

func buildBlueprints(defs []resolvedDef, ships []ShipRecord) []BlueprintRecord {
	shipIndex := makeShipIndex(ships)
	records := make([]BlueprintRecord, 0)

	for _, item := range defs {
		if item.Kind != RecordKindBlueprint {
			continue
		}

		craftResult, ok := fieldString(item.ResolvedFields, "craft_result").Get()
		if !ok || craftResult == "" {
			continue
		}

		requiredShipRaw := normalizeShipIDs(item.ResolvedFields.MustGet("required_ship"))
		records = append(records, BlueprintRecord{
			ID:               BlueprintID(item.ID),
			SourceFile:       item.SourceFile,
			CraftResult:      DefID(craftResult),
			CraftResultCount: fieldInt64(item.ResolvedFields, "craft_result_count").OrElse(1),
			Acquisition:      purchaseFromFields(item.ResolvedFields),
			Ingredients:      normalizeIngredients(item.ResolvedFields.MustGet("craft_ingredients")),
			RequiredShipRaw:  requiredShipRaw,
			AllowedShipIDs:   resolveShipIDs(requiredShipRaw, shipIndex),
			RequiredNode:     fieldInt64(item.ResolvedFields, "required_node"),
		})
	}

	sort.Slice(records, func(left int, right int) bool {
		return records[left].ID < records[right].ID
	})

	return records
}

func buildResources(world *World, defs []resolvedDef) []ResourceRecord {
	records := make([]ResourceRecord, 0)
	for _, item := range defs {
		if item.Kind != RecordKindResource {
			continue
		}

		records = append(records, ResourceRecord{
			ID:         item.ID,
			SourceFile: item.SourceFile,
			ItemSubtype: mapOption(enumField(world.Enums.ItemSubtype, item.ResolvedFields, "item_subtype"), func(value string) ItemSubtypeName {
				return ItemSubtypeName(value)
			}),
			Economy: EconomyInfo{
				Purchase: purchaseFromFields(item.ResolvedFields),
			},
		})
	}

	sort.Slice(records, func(left int, right int) bool {
		return records[left].ID < records[right].ID
	})

	return records
}

func buildModules(world *World, defs []resolvedDef, ships []ShipRecord, blueprintRecipes map[DefID][]BlueprintRecord) []ModuleRecord {
	shipIndex := makeShipIndex(ships)
	records := make([]ModuleRecord, 0)

	for _, item := range defs {
		if item.Kind != RecordKindModule {
			continue
		}
		if boolField(item.ResolvedFields, "cant_be_equipped") {
			continue
		}
		if item.ID == DefID("SpaceShipModule") {
			continue
		}

		requiredShipRaw := normalizeShipIDs(world.PreUpgradeRequiredShip[item.ID])
		requiredShipResolved := normalizeShipIDs(item.ResolvedFields.MustGet("required_ship"))

		records = append(records, ModuleRecord{
			ID:            item.ID,
			SourceFile:    item.SourceFile,
			InheritParent: item.InheritParent,
			InheritChain:  append([]DefID(nil), item.InheritChain...),
			ModuleType: mapOption(enumField(world.Enums.ModuleType, item.ResolvedFields, "module_type"), func(value string) ModuleTypeName {
				return ModuleTypeName(value)
			}),
			ItemSubtype: mapOption(enumField(world.Enums.ItemSubtype, item.ResolvedFields, "item_subtype"), func(value string) ItemSubtypeName {
				return ItemSubtypeName(value)
			}),
			Tier:        fieldInt64(item.ResolvedFields, "tier"),
			Mark:        fieldInt64(item.ResolvedFields, "mark"),
			VariantKind: detectVariantKind(item.ID, item.ResolvedFields),
			WeaponClass: mapOption(enumField(world.Enums.WeaponClass, item.ResolvedFields, "weapon_class"), func(value string) WeaponClassName {
				return WeaponClassName(value)
			}),
			Constraints: Constraints{
				RequiredRole: mapOption(enumField(world.Enums.ShipRoles, item.ResolvedFields, "required_role"), func(value string) ShipRoleName {
					return ShipRoleName(value)
				}),
				ClassMask:            maskValue(world.Enums.ShipClass, item.ResolvedFields.MustGet("class_mask")),
				RaceMask:             maskValue(world.Enums.RaceMask, item.ResolvedFields.MustGet("race_mask")),
				RankMin:              fieldInt64(item.ResolvedFields, "rank_min"),
				RankMax:              fieldInt64(item.ResolvedFields, "rank_max"),
				RequiredShipRaw:      requiredShipRaw,
				RequiredShipResolved: requiredShipResolved,
			},
			AllowedShipIDs: computeAllowedShips(world, shipIndex, item.ResolvedFields, requiredShipResolved),
			Upgrade: UpgradeInfo{
				Prev:  mapOption(fieldString(item.ResolvedFields, "prev_upgrade"), func(value string) DefID { return DefID(value) }),
				Next:  mapOption(fieldString(item.ResolvedFields, "next_upgrade"), func(value string) DefID { return DefID(value) }),
				Level: fieldInt64(item.ResolvedFields, "current_upgrade_level"),
			},
			Economy: EconomyInfo{
				Purchase: purchaseFromFields(item.ResolvedFields),
				Crafting: CraftingInfo{
					Recipes: toRecipes(blueprintRecipes[item.ID]),
				},
			},
		})
	}

	sort.Slice(records, func(left int, right int) bool {
		return records[left].ID < records[right].ID
	})

	return records
}

func buildUpgradeChains(modules []ModuleRecord) []UpgradeChain {
	groups := map[string][]ModuleRecord{}
	for _, item := range modules {
		subtype, ok := item.ItemSubtype.Get()
		if !ok {
			continue
		}

		tier, ok := item.Tier.Get()
		if !ok {
			continue
		}

		key := fmt.Sprintf("%s:%d", subtype, tier)
		groups[key] = append(groups[key], item)
	}

	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	chains := make([]UpgradeChain, 0, len(keys))
	for _, key := range keys {
		items := groups[key]
		sort.Slice(items, func(left int, right int) bool {
			leftWeight := chainSortWeight(items[left])
			rightWeight := chainSortWeight(items[right])
			if leftWeight == rightWeight {
				return items[left].ID < items[right].ID
			}
			return leftWeight < rightWeight
		})

		subtype, _ := items[0].ItemSubtype.Get()
		tier, _ := items[0].Tier.Get()

		ids := make([]DefID, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}

		chains = append(chains, UpgradeChain{
			ItemSubtype: subtype,
			Tier:        tier,
			Items:       ids,
		})
	}

	return chains
}

func buildCompatibility(modules []ModuleRecord, ships []ShipRecord) CompatibilityIndex {
	moduleSets := make(map[DefID]*linkedhashset.Set, len(modules))
	shipSets := make(map[ShipID]*linkedhashset.Set, len(ships))

	for _, ship := range ships {
		shipSets[ship.ID] = linkedhashset.New()
	}

	for _, module := range modules {
		moduleSet := linkedhashset.New()
		for _, shipID := range module.AllowedShipIDs {
			moduleSet.Add(shipID)

			shipSet, ok := shipSets[shipID]
			if !ok {
				shipSet = linkedhashset.New()
				shipSets[shipID] = shipSet
			}
			shipSet.Add(module.ID)
		}
		moduleSets[module.ID] = moduleSet
	}

	moduleToShips := make(map[DefID][]ShipID, len(moduleSets))
	for moduleID, set := range moduleSets {
		moduleToShips[moduleID] = canonicalizeShipIDSet(set)
	}

	shipToModules := make(map[ShipID][]DefID, len(shipSets))
	for shipID, set := range shipSets {
		shipToModules[shipID] = canonicalizeDefIDSet(set)
	}

	return CompatibilityIndex{
		ModuleToShips: moduleToShips,
		ShipToModules: shipToModules,
	}
}

func mergeObjects(parent LuaObject, child LuaObject) LuaObject {
	return MergeLuaObjects(parent, child, isDontInherit)
}

func pruneDontInherit(object LuaObject) LuaObject {
	return PruneLuaObject(object, isDontInherit)
}

func isDontInherit(value LuaValue) bool {
	text, ok := value.AsString()
	return ok && text == DontInheritSentinel
}

func fieldString(object LuaObject, key string) Option[string] {
	value, ok := object.Get(key)
	if !ok {
		return None[string]()
	}

	text, ok := value.AsString()
	if !ok {
		return None[string]()
	}

	return Some(text)
}

func fieldInt64(object LuaObject, key string) Option[int64] {
	value, ok := object.Get(key)
	if !ok {
		return None[int64]()
	}

	number, ok := value.AsInt64()
	if !ok {
		return None[int64]()
	}

	return Some(number)
}

func boolField(object LuaObject, key string) bool {
	value, ok := object.Get(key)
	if !ok {
		return false
	}

	result, ok := value.AsBool()
	return ok && result
}

func enumField(enum EnumTable, object LuaObject, key string) Option[string] {
	value, ok := object.Get(key)
	if !ok {
		return None[string]()
	}

	number, ok := value.AsInt64()
	if !ok {
		return None[string]()
	}

	return enum.NameOf(number)
}

func mapOption[T any, U any](value Option[T], mapper func(T) U) Option[U] {
	raw, ok := value.Get()
	if !ok {
		return None[U]()
	}

	return Some(mapper(raw))
}

func maskValue(enum EnumTable, value LuaValue) Option[MaskValue] {
	number, ok := value.AsInt64()
	if !ok {
		return None[MaskValue]()
	}

	return Some(MaskValue{
		Raw:   number,
		Flags: enum.FlagsOf(number),
	})
}

func detectVariantKind(id DefID, fields LuaObject) Option[VariantKind] {
	switch {
	case strings.HasSuffix(id.String(), "_Mk1"):
		return Some(VariantKindMk1)
	case strings.HasSuffix(id.String(), "_Rare"):
		return Some(VariantKindRare)
	case strings.HasSuffix(id.String(), "_Mk3"):
		return Some(VariantKindMk3)
	case strings.HasSuffix(id.String(), "_Epic"):
		return Some(VariantKindEpic)
	case strings.HasSuffix(id.String(), "_Rel"):
		return Some(VariantKindRelic)
	case strings.HasSuffix(id.String(), "_Prem"):
		return Some(VariantKindPremium)
	case strings.HasSuffix(id.String(), "_Tournament"):
		return Some(VariantKindTournament)
	}

	switch fieldInt64(fields, "mark").OrElse(0) {
	case 1:
		return Some(VariantKindMk1)
	case 3:
		return Some(VariantKindMk3)
	case 5:
		return Some(VariantKindRelic)
	case 6:
		return Some(VariantKindRare)
	case 7:
		return Some(VariantKindEpic)
	default:
		return None[VariantKind]()
	}
}

func normalizeStringList(value LuaValue) []string {
	if value.IsNull() {
		return nil
	}

	if text, ok := value.AsString(); ok {
		return []string{text}
	}

	if items, ok := value.AsArray(); ok {
		values := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := item.AsString()
			if ok {
				values = append(values, text)
			}
		}
		return CanonicalizeStrings(values)
	}

	object, ok := value.AsObject()
	if !ok {
		return nil
	}

	keys := object.Keys()
	SortNumericStrings(keys)

	values := make([]string, 0, len(keys))
	for _, key := range keys {
		text, ok := object.MustGet(key).AsString()
		if ok {
			values = append(values, text)
		}
	}

	return CanonicalizeStrings(values)
}

func normalizeShipIDs(value LuaValue) []ShipID {
	raw := normalizeStringList(value)
	ids := make([]ShipID, 0, len(raw))
	for _, item := range raw {
		ids = append(ids, ShipID(item))
	}
	return ids
}

func normalizeIngredients(value LuaValue) []CraftIngredient {
	object, ok := value.AsObject()
	if !ok {
		return nil
	}

	keys := object.Keys()
	SortNumericStrings(keys)

	ingredients := make([]CraftIngredient, 0, len(keys))
	for _, key := range keys {
		entry, ok := object.MustGet(key).AsObject()
		if !ok {
			continue
		}

		amount := fieldInt64(entry, "count").OrElse(fieldInt64(entry, "amount").OrElse(0))
		if amount == 0 {
			continue
		}

		if defID, ok := fieldString(entry, "def").Get(); ok {
			ingredients = append(ingredients, CraftIngredient{
				Kind:   CraftIngredientKindItem,
				ID:     defID,
				Amount: amount,
			})
			continue
		}

		if resourceID, ok := fieldString(entry, "resource").Get(); ok {
			ingredients = append(ingredients, CraftIngredient{
				Kind:   CraftIngredientKindResource,
				ID:     resourceID,
				Amount: amount,
			})
			continue
		}

		if currencyID, ok := fieldString(entry, "currency").Get(); ok {
			ingredients = append(ingredients, CraftIngredient{
				Kind:   CraftIngredientKindCurrency,
				ID:     currencyID,
				Amount: amount,
			})
		}
	}

	return ingredients
}

func purchaseFromFields(fields LuaObject) PurchaseInfo {
	return PurchaseInfo{
		Price:           fieldInt64(fields, "price"),
		PremiumPrice:    fieldInt64(fields, "premiumPrice"),
		TokenPrice:      fieldInt64(fields, "tokenPrice"),
		StoreItemID:     fieldInt64(fields, "storeItemId"),
		CantBeBought:    boolField(fields, "cant_be_bought"),
		ShopCategory:    fieldString(fields, "shopCategory"),
		ShopSubCategory: fieldString(fields, "shopSubCategory"),
	}
}

func normalizeDefaultModules(value LuaValue, slots EnumTable) map[string]DefID {
	object, ok := value.AsObject()
	if !ok {
		return nil
	}

	keys := object.Keys()
	SortNumericStrings(keys)

	out := make(map[string]DefID, len(keys))
	for _, key := range keys {
		slotName := slotNameForKey(slots, key)
		moduleID, ok := object.MustGet(key).AsString()
		if !ok {
			continue
		}
		out[slotName] = DefID(moduleID)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func normalizeSlotTypes(value LuaValue, slots EnumTable, moduleTypes EnumTable) map[string]ModuleTypeName {
	object, ok := value.AsObject()
	if !ok {
		return nil
	}

	keys := object.Keys()
	SortNumericStrings(keys)

	out := make(map[string]ModuleTypeName, len(keys))
	for _, key := range keys {
		slotName := slotNameForKey(slots, key)
		entry := object.MustGet(key)
		if raw, ok := entry.AsString(); ok {
			out[slotName] = ModuleTypeName(raw)
			continue
		}

		number, ok := entry.AsInt64()
		if !ok {
			continue
		}

		name, ok := moduleTypes.NameOf(number).Get()
		if !ok {
			out[slotName] = ModuleTypeName(strconv.FormatInt(number, 10))
			continue
		}

		out[slotName] = ModuleTypeName(name)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func slotNameForKey(slots EnumTable, key string) string {
	slotNumber, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		return key
	}

	name, ok := slots.NameOf(slotNumber).Get()
	if !ok {
		return key
	}

	return name
}

func toRecipes(items []BlueprintRecord) []Recipe {
	recipes := make([]Recipe, 0, len(items))
	for _, item := range items {
		recipes = append(recipes, Recipe{
			BlueprintID:      item.ID,
			Acquisition:      item.Acquisition,
			Ingredients:      append([]CraftIngredient(nil), item.Ingredients...),
			CraftResultCount: item.CraftResultCount,
			RequiredShipRaw:  append([]ShipID(nil), item.RequiredShipRaw...),
			AllowedShipIDs:   append([]ShipID(nil), item.AllowedShipIDs...),
			RequiredNode:     item.RequiredNode,
		})
	}

	sort.Slice(recipes, func(left int, right int) bool {
		return recipes[left].BlueprintID < recipes[right].BlueprintID
	})

	return recipes
}

func makeShipIndex(ships []ShipRecord) map[ShipID]ShipRecord {
	index := make(map[ShipID]ShipRecord, len(ships))
	for _, ship := range ships {
		index[ship.ID] = ship
	}
	return index
}

func groupBlueprintsByCraftResult(items []BlueprintRecord) map[DefID][]BlueprintRecord {
	out := make(map[DefID][]BlueprintRecord, len(items))
	for _, item := range items {
		out[item.CraftResult] = append(out[item.CraftResult], item)
	}
	return out
}

func computeAllowedShips(world *World, shipIndex map[ShipID]ShipRecord, fields LuaObject, requiredShipResolved []ShipID) []ShipID {
	requiredSet := linkedhashset.New()
	for _, shipID := range requiredShipResolved {
		requiredSet.Add(shipID)
	}

	classMask, hasClassMask := fieldInt64(fields, "class_mask").Get()
	requiredRole, hasRequiredRole := fieldInt64(fields, "required_role").Get()
	raceMask, hasRaceMask := fieldInt64(fields, "race_mask").Get()
	rankMin, hasRankMin := fieldInt64(fields, "rank_min").Get()
	rankMax, hasRankMax := fieldInt64(fields, "rank_max").Get()

	shipIDs := make([]ShipID, 0, len(shipIndex))
	for shipID := range shipIndex {
		shipIDs = append(shipIDs, shipID)
	}
	sort.Slice(shipIDs, func(left int, right int) bool {
		return shipIDs[left] < shipIDs[right]
	})

	allowed := linkedhashset.New()
	for _, shipID := range shipIDs {
		ship := shipIndex[shipID]

		if requiredSet.Size() > 0 && !requiredSet.Contains(shipID) {
			continue
		}

		if hasClassMask {
			shipClass, ok := ship.ShipClass.Get()
			if !ok {
				continue
			}
			shipClassValue, ok := world.Enums.ShipClass.ValueOf(string(shipClass)).Get()
			if !ok || classMask&shipClassValue != shipClassValue {
				continue
			}
		}

		if hasRequiredRole {
			role, ok := ship.Role.Get()
			if !ok {
				continue
			}
			roleValue, ok := world.Enums.ShipRoles.ValueOf(string(role)).Get()
			if !ok || roleValue != requiredRole {
				continue
			}
		}

		if hasRaceMask {
			race, ok := ship.Race.Get()
			if !ok {
				continue
			}
			raceValue, ok := world.Enums.RaceMask.ValueOf(string(race)).Get()
			if !ok || raceMask&raceValue != raceValue {
				continue
			}
		}

		if hasRankMin {
			rank, ok := ship.Rank.Get()
			if !ok || rank < rankMin {
				continue
			}
		}

		if hasRankMax {
			rank, ok := ship.Rank.Get()
			if !ok || rank > rankMax {
				continue
			}
		}

		allowed.Add(shipID)
	}

	return canonicalizeShipIDSet(allowed)
}

func resolveShipIDs(values []ShipID, shipIndex map[ShipID]ShipRecord) []ShipID {
	set := linkedhashset.New()
	for _, shipID := range values {
		if _, ok := shipIndex[shipID]; ok {
			set.Add(shipID)
		}
	}

	return canonicalizeShipIDSet(set)
}

func canonicalizeShipIDSet(set *linkedhashset.Set) []ShipID {
	raw := make([]string, 0, set.Size())
	for _, item := range set.Values() {
		raw = append(raw, string(item.(ShipID)))
	}

	raw = CanonicalizeStrings(raw)
	ids := make([]ShipID, 0, len(raw))
	for _, item := range raw {
		ids = append(ids, ShipID(item))
	}
	return ids
}

func canonicalizeDefIDSet(set *linkedhashset.Set) []DefID {
	raw := make([]string, 0, set.Size())
	for _, item := range set.Values() {
		raw = append(raw, string(item.(DefID)))
	}

	raw = CanonicalizeStrings(raw)
	ids := make([]DefID, 0, len(raw))
	for _, item := range raw {
		ids = append(ids, DefID(item))
	}
	return ids
}

func chainSortWeight(item ModuleRecord) int64 {
	if level, ok := item.Upgrade.Level.Get(); ok {
		return level
	}

	variant, ok := item.VariantKind.Get()
	if !ok {
		return 999
	}

	switch variant {
	case VariantKindMk1:
		return 1
	case VariantKindRare:
		return 2
	case VariantKindMk3:
		return 3
	case VariantKindEpic:
		return 4
	case VariantKindRelic:
		return 5
	case VariantKindPremium:
		return 6
	case VariantKindTournament:
		return 7
	default:
		return 999
	}
}

func sortDefIDs(ids []DefID) {
	sort.Slice(ids, func(left int, right int) bool {
		return ids[left] < ids[right]
	})
}
