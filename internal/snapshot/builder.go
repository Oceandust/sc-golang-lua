package snapshot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"defgraph/internal/luavalue"
	"defgraph/internal/types"

	"github.com/emirpasic/gods/sets/linkedhashset"
)

type resolvedDef struct {
	ID             types.DefID
	SourceFile     types.ScriptPath
	LocalFields    luavalue.Object
	ResolvedFields luavalue.Object
	InheritParent  types.Option[types.DefID]
	InheritChain   []types.DefID
	Kind           types.RecordKind
}

type resolver struct {
	world     *types.World
	resolved  map[types.DefID]resolvedDef
	resolving map[types.DefID]struct{}
}

func Build(world *types.World) (*types.Snapshot, error) {
	ids := make([]types.DefID, 0, len(world.Defs))
	for id := range world.Defs {
		ids = append(ids, id)
	}
	sortDefIDs(ids)

	resolver := resolver{
		world:     world,
		resolved:  make(map[types.DefID]resolvedDef, len(ids)),
		resolving: make(map[types.DefID]struct{}, len(ids)),
	}

	defs := make([]resolvedDef, 0, len(ids))
	for _, id := range ids {
		item, err := resolver.resolve(id)
		if err != nil {
			return nil, err
		}
		defs = append(defs, item)
	}

	defRecords := make([]types.DefRecord, 0, len(defs))
	for _, item := range defs {
		defRecords = append(defRecords, types.DefRecord{
			ID:             item.ID,
			Kind:           item.Kind,
			SourceFile:     item.SourceFile,
			InheritParent:  item.InheritParent,
			InheritChain:   append([]types.DefID(nil), item.InheritChain...),
			LocalFields:    item.LocalFields.Clone(),
			ResolvedFields: item.ResolvedFields.Clone(),
		})
	}

	ships := buildShips(world, defs)
	blueprints := buildBlueprints(defs, ships)
	blueprintRecipes := groupBlueprintsByCraftResult(blueprints)
	resources := buildResources(world, defs)
	modules := buildModules(world, defs, ships, blueprintRecipes)

	return &types.Snapshot{
		Meta: types.Meta{
			SchemaVersion: types.SchemaVersionV1,
			Loader:        world.Loader,
			LoaderRuntime: world.LoaderRuntime,
			RepoRoot:      world.RepoRoot,
			CompiledRoot:  world.CompiledRoot,
			LoadOrder:     append([]types.ScriptPath(nil), world.FilesLoaded...),
			LoadedFiles:   append([]types.ScriptPath(nil), world.FilesLoaded...),
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

func (resolver *resolver) resolve(id types.DefID) (resolvedDef, error) {
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

	resolvedFields := luavalue.NewObject()
	inheritChain := make([]types.DefID, 0, 4)

	if parentID, ok := raw.InheritParent.Get(); ok {
		if _, exists := resolver.world.Defs[parentID]; exists {
			parentResolved, err := resolver.resolve(parentID)
			if err != nil {
				return resolvedDef{}, err
			}

			resolvedFields = parentResolved.ResolvedFields.Clone()

			chainSet := linkedhashset.New()
			chainSet.Add(parentID)
			for _, ancestorID := range parentResolved.InheritChain {
				chainSet.Add(ancestorID)
			}

			for _, rawValue := range chainSet.Values() {
				inheritChain = append(inheritChain, rawValue.(types.DefID))
			}
		}
	}

	resolvedFields = mergeObjects(resolvedFields, raw.LocalFields)

	value := resolvedDef{
		ID:             raw.ID,
		SourceFile:     raw.SourceFile,
		LocalFields:    raw.LocalFields.Clone(),
		ResolvedFields: resolvedFields,
		InheritParent:  raw.InheritParent,
		InheritChain:   inheritChain,
		Kind:           classifyDef(resolvedFields),
	}

	resolver.resolved[id] = value
	return value, nil
}

func classifyDef(fields luavalue.Object) types.RecordKind {
	if boolField(fields, "is_blueprint") {
		return types.RecordKindBlueprint
	}

	if boolField(fields, "is_resource") {
		return types.RecordKindResource
	}

	if fieldString(fields, "class").OrElse("") == "SpaceShip" {
		return types.RecordKindShip
	}

	if fields.MustGet("module_type").IsNull() && fields.MustGet("weapon_class").IsNull() {
		className := fieldString(fields, "class").OrElse("")
		switch className {
		case "SpaceShipModule", "EngineModule", "ActiveModule":
			return types.RecordKindModule
		default:
			return types.RecordKindOther
		}
	}

	return types.RecordKindModule
}

func buildShips(world *types.World, defs []resolvedDef) []types.ShipRecord {
	records := make([]types.ShipRecord, 0)
	for _, item := range defs {
		if item.Kind != types.RecordKindShip {
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

		records = append(records, types.ShipRecord{
			ID:            types.ShipID(item.ID),
			SourceFile:    item.SourceFile,
			InheritParent: item.InheritParent,
			InheritChain:  append([]types.DefID(nil), item.InheritChain...),
			ShipName:      fieldString(item.ResolvedFields, "ship_name"),
			ShipTier:      fieldInt64(item.ResolvedFields, "ship_tier"),
			Rank:          fieldInt64(item.ResolvedFields, "rank"),
			Role: mapOption(enumField(world.Enums.ShipRoles, item.ResolvedFields, "role"), func(value string) types.ShipRoleName {
				return types.ShipRoleName(value)
			}),
			ShipClass: mapOption(enumField(world.Enums.ShipClass, item.ResolvedFields, "ship_class"), func(value string) types.ShipClassName {
				return types.ShipClassName(value)
			}),
			Race: mapOption(enumField(world.Enums.Race, item.ResolvedFields, "race"), func(value string) types.RaceName {
				return types.RaceName(value)
			}),
			IsPremium:      boolField(item.ResolvedFields, "isPremium"),
			DefaultModules: normalizeDefaultModules(item.ResolvedFields.MustGet("default_modules"), world.Enums.SpaceShipModuleSlot),
			SlotTypes:      normalizeSlotTypes(item.ResolvedFields.MustGet("slot_module_types"), world.Enums.SpaceShipModuleSlot, world.Enums.ModuleType),
			Economy: types.EconomyInfo{
				Purchase: purchaseFromFields(item.ResolvedFields),
				Crafting: types.CraftingInfo{
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

func buildBlueprints(defs []resolvedDef, ships []types.ShipRecord) []types.BlueprintRecord {
	shipIndex := makeShipIndex(ships)
	records := make([]types.BlueprintRecord, 0)

	for _, item := range defs {
		if item.Kind != types.RecordKindBlueprint {
			continue
		}

		craftResult, ok := fieldString(item.ResolvedFields, "craft_result").Get()
		if !ok || craftResult == "" {
			continue
		}

		requiredShipRaw := normalizeShipIDs(item.ResolvedFields.MustGet("required_ship"))
		records = append(records, types.BlueprintRecord{
			ID:               types.BlueprintID(item.ID),
			SourceFile:       item.SourceFile,
			CraftResult:      types.DefID(craftResult),
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

func buildResources(world *types.World, defs []resolvedDef) []types.ResourceRecord {
	records := make([]types.ResourceRecord, 0)
	for _, item := range defs {
		if item.Kind != types.RecordKindResource {
			continue
		}

		records = append(records, types.ResourceRecord{
			ID:         item.ID,
			SourceFile: item.SourceFile,
			ItemSubtype: mapOption(enumField(world.Enums.ItemSubtype, item.ResolvedFields, "item_subtype"), func(value string) types.ItemSubtypeName {
				return types.ItemSubtypeName(value)
			}),
			Economy: types.EconomyInfo{
				Purchase: purchaseFromFields(item.ResolvedFields),
			},
		})
	}

	sort.Slice(records, func(left int, right int) bool {
		return records[left].ID < records[right].ID
	})

	return records
}

func buildModules(world *types.World, defs []resolvedDef, ships []types.ShipRecord, blueprintRecipes map[types.DefID][]types.BlueprintRecord) []types.ModuleRecord {
	shipIndex := makeShipIndex(ships)
	records := make([]types.ModuleRecord, 0)

	for _, item := range defs {
		if item.Kind != types.RecordKindModule {
			continue
		}
		if boolField(item.ResolvedFields, "cant_be_equipped") {
			continue
		}
		if item.ID == types.DefID("SpaceShipModule") {
			continue
		}

		requiredShipRaw := normalizeShipIDs(world.PreUpgradeRequiredShip[item.ID])
		requiredShipResolved := normalizeShipIDs(item.ResolvedFields.MustGet("required_ship"))

		records = append(records, types.ModuleRecord{
			ID:            item.ID,
			SourceFile:    item.SourceFile,
			InheritParent: item.InheritParent,
			InheritChain:  append([]types.DefID(nil), item.InheritChain...),
			ModuleType: mapOption(enumField(world.Enums.ModuleType, item.ResolvedFields, "module_type"), func(value string) types.ModuleTypeName {
				return types.ModuleTypeName(value)
			}),
			ItemSubtype: mapOption(enumField(world.Enums.ItemSubtype, item.ResolvedFields, "item_subtype"), func(value string) types.ItemSubtypeName {
				return types.ItemSubtypeName(value)
			}),
			Tier:        fieldInt64(item.ResolvedFields, "tier"),
			Mark:        fieldInt64(item.ResolvedFields, "mark"),
			VariantKind: detectVariantKind(item.ID, item.ResolvedFields),
			WeaponClass: mapOption(enumField(world.Enums.WeaponClass, item.ResolvedFields, "weapon_class"), func(value string) types.WeaponClassName {
				return types.WeaponClassName(value)
			}),
			Constraints: types.Constraints{
				RequiredRole: mapOption(enumField(world.Enums.ShipRoles, item.ResolvedFields, "required_role"), func(value string) types.ShipRoleName {
					return types.ShipRoleName(value)
				}),
				ClassMask:            maskValue(world.Enums.ShipClass, item.ResolvedFields.MustGet("class_mask")),
				RaceMask:             maskValue(world.Enums.RaceMask, item.ResolvedFields.MustGet("race_mask")),
				RankMin:              fieldInt64(item.ResolvedFields, "rank_min"),
				RankMax:              fieldInt64(item.ResolvedFields, "rank_max"),
				RequiredShipRaw:      requiredShipRaw,
				RequiredShipResolved: requiredShipResolved,
			},
			AllowedShipIDs: computeAllowedShips(world, shipIndex, item.ResolvedFields, requiredShipResolved),
			Upgrade: types.UpgradeInfo{
				Prev:  mapOption(fieldString(item.ResolvedFields, "prev_upgrade"), func(value string) types.DefID { return types.DefID(value) }),
				Next:  mapOption(fieldString(item.ResolvedFields, "next_upgrade"), func(value string) types.DefID { return types.DefID(value) }),
				Level: fieldInt64(item.ResolvedFields, "current_upgrade_level"),
			},
			Economy: types.EconomyInfo{
				Purchase: purchaseFromFields(item.ResolvedFields),
				Crafting: types.CraftingInfo{
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

func buildUpgradeChains(modules []types.ModuleRecord) []types.UpgradeChain {
	groups := map[string][]types.ModuleRecord{}
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

	chains := make([]types.UpgradeChain, 0, len(keys))
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

		ids := make([]types.DefID, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}

		chains = append(chains, types.UpgradeChain{
			ItemSubtype: subtype,
			Tier:        tier,
			Items:       ids,
		})
	}

	return chains
}

func buildCompatibility(modules []types.ModuleRecord, ships []types.ShipRecord) types.CompatibilityIndex {
	moduleSets := make(map[types.DefID]*linkedhashset.Set, len(modules))
	shipSets := make(map[types.ShipID]*linkedhashset.Set, len(ships))

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

	moduleToShips := make(map[types.DefID][]types.ShipID, len(moduleSets))
	for moduleID, set := range moduleSets {
		moduleToShips[moduleID] = canonicalizeShipIDSet(set)
	}

	shipToModules := make(map[types.ShipID][]types.DefID, len(shipSets))
	for shipID, set := range shipSets {
		shipToModules[shipID] = canonicalizeDefIDSet(set)
	}

	return types.CompatibilityIndex{
		ModuleToShips: moduleToShips,
		ShipToModules: shipToModules,
	}
}

func mergeObjects(parent luavalue.Object, child luavalue.Object) luavalue.Object {
	out := parent.Clone()
	child.Range(func(key string, value luavalue.Value) bool {
		if isDontInherit(value) {
			out.Delete(key)
			return true
		}

		childObject, childIsObject := value.AsObject()
		parentValue, hasParent := out.Get(key)
		parentObject, parentIsObject := parentValue.AsObject()

		switch {
		case childIsObject && hasParent && parentIsObject:
			out.Set(key, luavalue.ObjectValue(mergeObjects(parentObject, childObject)))
		case childIsObject:
			out.Set(key, luavalue.ObjectValue(pruneDontInherit(childObject)))
		default:
			out.Set(key, value.Clone())
		}

		return true
	})

	return out
}

func pruneDontInherit(object luavalue.Object) luavalue.Object {
	out := luavalue.NewObject()
	object.Range(func(key string, value luavalue.Value) bool {
		if isDontInherit(value) {
			return true
		}

		nested, ok := value.AsObject()
		if ok {
			out.Set(key, luavalue.ObjectValue(pruneDontInherit(nested)))
			return true
		}

		out.Set(key, value.Clone())
		return true
	})

	return out
}

func isDontInherit(value luavalue.Value) bool {
	text, ok := value.AsString()
	return ok && text == types.DontInheritSentinel
}

func fieldString(object luavalue.Object, key string) types.Option[string] {
	value, ok := object.Get(key)
	if !ok {
		return types.None[string]()
	}

	text, ok := value.AsString()
	if !ok {
		return types.None[string]()
	}

	return types.Some(text)
}

func fieldInt64(object luavalue.Object, key string) types.Option[int64] {
	value, ok := object.Get(key)
	if !ok {
		return types.None[int64]()
	}

	number, ok := value.AsInt64()
	if !ok {
		return types.None[int64]()
	}

	return types.Some(number)
}

func boolField(object luavalue.Object, key string) bool {
	value, ok := object.Get(key)
	if !ok {
		return false
	}

	result, ok := value.AsBool()
	return ok && result
}

func enumField(enum types.EnumTable, object luavalue.Object, key string) types.Option[string] {
	value, ok := object.Get(key)
	if !ok {
		return types.None[string]()
	}

	number, ok := value.AsInt64()
	if !ok {
		return types.None[string]()
	}

	return enum.NameOf(number)
}

func mapOption[T any, U any](value types.Option[T], mapper func(T) U) types.Option[U] {
	raw, ok := value.Get()
	if !ok {
		return types.None[U]()
	}

	return types.Some(mapper(raw))
}

func maskValue(enum types.EnumTable, value luavalue.Value) types.Option[types.MaskValue] {
	number, ok := value.AsInt64()
	if !ok {
		return types.None[types.MaskValue]()
	}

	return types.Some(types.MaskValue{
		Raw:   number,
		Flags: enum.FlagsOf(number),
	})
}

func detectVariantKind(id types.DefID, fields luavalue.Object) types.Option[types.VariantKind] {
	switch {
	case strings.HasSuffix(id.String(), "_Mk1"):
		return types.Some(types.VariantKindMk1)
	case strings.HasSuffix(id.String(), "_Rare"):
		return types.Some(types.VariantKindRare)
	case strings.HasSuffix(id.String(), "_Mk3"):
		return types.Some(types.VariantKindMk3)
	case strings.HasSuffix(id.String(), "_Epic"):
		return types.Some(types.VariantKindEpic)
	case strings.HasSuffix(id.String(), "_Rel"):
		return types.Some(types.VariantKindRelic)
	case strings.HasSuffix(id.String(), "_Prem"):
		return types.Some(types.VariantKindPremium)
	case strings.HasSuffix(id.String(), "_Tournament"):
		return types.Some(types.VariantKindTournament)
	}

	switch fieldInt64(fields, "mark").OrElse(0) {
	case 1:
		return types.Some(types.VariantKindMk1)
	case 3:
		return types.Some(types.VariantKindMk3)
	case 5:
		return types.Some(types.VariantKindRelic)
	case 6:
		return types.Some(types.VariantKindRare)
	case 7:
		return types.Some(types.VariantKindEpic)
	default:
		return types.None[types.VariantKind]()
	}
}

func normalizeStringList(value luavalue.Value) []string {
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
		return types.CanonicalizeStrings(values)
	}

	object, ok := value.AsObject()
	if !ok {
		return nil
	}

	keys := object.Keys()
	types.SortNumericStrings(keys)

	values := make([]string, 0, len(keys))
	for _, key := range keys {
		text, ok := object.MustGet(key).AsString()
		if ok {
			values = append(values, text)
		}
	}

	return types.CanonicalizeStrings(values)
}

func normalizeShipIDs(value luavalue.Value) []types.ShipID {
	raw := normalizeStringList(value)
	ids := make([]types.ShipID, 0, len(raw))
	for _, item := range raw {
		ids = append(ids, types.ShipID(item))
	}
	return ids
}

func normalizeIngredients(value luavalue.Value) []types.CraftIngredient {
	object, ok := value.AsObject()
	if !ok {
		return nil
	}

	keys := object.Keys()
	types.SortNumericStrings(keys)

	ingredients := make([]types.CraftIngredient, 0, len(keys))
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
			ingredients = append(ingredients, types.CraftIngredient{
				Kind:   types.CraftIngredientKindItem,
				ID:     defID,
				Amount: amount,
			})
			continue
		}

		if resourceID, ok := fieldString(entry, "resource").Get(); ok {
			ingredients = append(ingredients, types.CraftIngredient{
				Kind:   types.CraftIngredientKindResource,
				ID:     resourceID,
				Amount: amount,
			})
			continue
		}

		if currencyID, ok := fieldString(entry, "currency").Get(); ok {
			ingredients = append(ingredients, types.CraftIngredient{
				Kind:   types.CraftIngredientKindCurrency,
				ID:     currencyID,
				Amount: amount,
			})
		}
	}

	return ingredients
}

func purchaseFromFields(fields luavalue.Object) types.PurchaseInfo {
	return types.PurchaseInfo{
		Price:           fieldInt64(fields, "price"),
		PremiumPrice:    fieldInt64(fields, "premiumPrice"),
		TokenPrice:      fieldInt64(fields, "tokenPrice"),
		StoreItemID:     fieldInt64(fields, "storeItemId"),
		CantBeBought:    boolField(fields, "cant_be_bought"),
		ShopCategory:    fieldString(fields, "shopCategory"),
		ShopSubCategory: fieldString(fields, "shopSubCategory"),
	}
}

func normalizeDefaultModules(value luavalue.Value, slots types.EnumTable) map[string]types.DefID {
	object, ok := value.AsObject()
	if !ok {
		return nil
	}

	keys := object.Keys()
	types.SortNumericStrings(keys)

	out := make(map[string]types.DefID, len(keys))
	for _, key := range keys {
		slotName := slotNameForKey(slots, key)
		moduleID, ok := object.MustGet(key).AsString()
		if !ok {
			continue
		}
		out[slotName] = types.DefID(moduleID)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func normalizeSlotTypes(value luavalue.Value, slots types.EnumTable, moduleTypes types.EnumTable) map[string]types.ModuleTypeName {
	object, ok := value.AsObject()
	if !ok {
		return nil
	}

	keys := object.Keys()
	types.SortNumericStrings(keys)

	out := make(map[string]types.ModuleTypeName, len(keys))
	for _, key := range keys {
		slotName := slotNameForKey(slots, key)
		entry := object.MustGet(key)
		if raw, ok := entry.AsString(); ok {
			out[slotName] = types.ModuleTypeName(raw)
			continue
		}

		number, ok := entry.AsInt64()
		if !ok {
			continue
		}

		name, ok := moduleTypes.NameOf(number).Get()
		if !ok {
			out[slotName] = types.ModuleTypeName(strconv.FormatInt(number, 10))
			continue
		}

		out[slotName] = types.ModuleTypeName(name)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func slotNameForKey(slots types.EnumTable, key string) string {
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

func toRecipes(items []types.BlueprintRecord) []types.Recipe {
	recipes := make([]types.Recipe, 0, len(items))
	for _, item := range items {
		recipes = append(recipes, types.Recipe{
			BlueprintID:      item.ID,
			Acquisition:      item.Acquisition,
			Ingredients:      append([]types.CraftIngredient(nil), item.Ingredients...),
			CraftResultCount: item.CraftResultCount,
			RequiredShipRaw:  append([]types.ShipID(nil), item.RequiredShipRaw...),
			AllowedShipIDs:   append([]types.ShipID(nil), item.AllowedShipIDs...),
			RequiredNode:     item.RequiredNode,
		})
	}

	sort.Slice(recipes, func(left int, right int) bool {
		return recipes[left].BlueprintID < recipes[right].BlueprintID
	})

	return recipes
}

func makeShipIndex(ships []types.ShipRecord) map[types.ShipID]types.ShipRecord {
	index := make(map[types.ShipID]types.ShipRecord, len(ships))
	for _, ship := range ships {
		index[ship.ID] = ship
	}
	return index
}

func groupBlueprintsByCraftResult(items []types.BlueprintRecord) map[types.DefID][]types.BlueprintRecord {
	out := make(map[types.DefID][]types.BlueprintRecord, len(items))
	for _, item := range items {
		out[item.CraftResult] = append(out[item.CraftResult], item)
	}
	return out
}

func computeAllowedShips(world *types.World, shipIndex map[types.ShipID]types.ShipRecord, fields luavalue.Object, requiredShipResolved []types.ShipID) []types.ShipID {
	requiredSet := linkedhashset.New()
	for _, shipID := range requiredShipResolved {
		requiredSet.Add(shipID)
	}

	classMask, hasClassMask := fieldInt64(fields, "class_mask").Get()
	requiredRole, hasRequiredRole := fieldInt64(fields, "required_role").Get()
	raceMask, hasRaceMask := fieldInt64(fields, "race_mask").Get()
	rankMin, hasRankMin := fieldInt64(fields, "rank_min").Get()
	rankMax, hasRankMax := fieldInt64(fields, "rank_max").Get()

	shipIDs := make([]types.ShipID, 0, len(shipIndex))
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

func resolveShipIDs(values []types.ShipID, shipIndex map[types.ShipID]types.ShipRecord) []types.ShipID {
	set := linkedhashset.New()
	for _, shipID := range values {
		if _, ok := shipIndex[shipID]; ok {
			set.Add(shipID)
		}
	}

	return canonicalizeShipIDSet(set)
}

func canonicalizeShipIDSet(set *linkedhashset.Set) []types.ShipID {
	raw := make([]string, 0, set.Size())
	for _, item := range set.Values() {
		raw = append(raw, string(item.(types.ShipID)))
	}

	raw = types.CanonicalizeStrings(raw)
	ids := make([]types.ShipID, 0, len(raw))
	for _, item := range raw {
		ids = append(ids, types.ShipID(item))
	}
	return ids
}

func canonicalizeDefIDSet(set *linkedhashset.Set) []types.DefID {
	raw := make([]string, 0, set.Size())
	for _, item := range set.Values() {
		raw = append(raw, string(item.(types.DefID)))
	}

	raw = types.CanonicalizeStrings(raw)
	ids := make([]types.DefID, 0, len(raw))
	for _, item := range raw {
		ids = append(ids, types.DefID(item))
	}
	return ids
}

func chainSortWeight(item types.ModuleRecord) int64 {
	if level, ok := item.Upgrade.Level.Get(); ok {
		return level
	}

	variant, ok := item.VariantKind.Get()
	if !ok {
		return 999
	}

	switch variant {
	case types.VariantKindMk1:
		return 1
	case types.VariantKindRare:
		return 2
	case types.VariantKindMk3:
		return 3
	case types.VariantKindEpic:
		return 4
	case types.VariantKindRelic:
		return 5
	case types.VariantKindPremium:
		return 6
	case types.VariantKindTournament:
		return 7
	default:
		return 999
	}
}

func sortDefIDs(ids []types.DefID) {
	sort.Slice(ids, func(left int, right int) bool {
		return ids[left] < ids[right]
	})
}
