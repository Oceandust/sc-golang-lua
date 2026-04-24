package defgraph

type RawDef struct {
	ID            DefID
	SourceFile    ScriptPath
	LocalFields   LuaObject
	InheritParent Option[DefID]
}

type World struct {
	RepoRoot               string
	CompiledRoot           string
	Loader                 LoaderName
	LoaderRuntime          LoaderRuntime
	FilesLoaded            []ScriptPath
	Warnings               []string
	Enums                  EnumRegistry
	Defs                   map[DefID]RawDef
	PreUpgradeRequiredShip map[DefID]LuaValue
}

type MaskValue struct {
	Raw   int64    `json:"raw"`
	Flags []string `json:"flags,omitempty"`
}

type Meta struct {
	SchemaVersion SchemaVersion `json:"schema_version"`
	Loader        LoaderName    `json:"loader"`
	LoaderRuntime LoaderRuntime `json:"loader_runtime,omitempty"`
	RepoRoot      string        `json:"repo_root,omitempty"`
	CompiledRoot  string        `json:"compiled_root,omitempty"`
	LoadOrder     []ScriptPath  `json:"load_order,omitempty"`
	LoadedFiles   []ScriptPath  `json:"loaded_files,omitempty"`
}

type PurchaseInfo struct {
	Price           Option[int64]  `json:"price,omitempty"`
	PremiumPrice    Option[int64]  `json:"premiumPrice,omitempty"`
	TokenPrice      Option[int64]  `json:"tokenPrice,omitempty"`
	StoreItemID     Option[int64]  `json:"storeItemId,omitempty"`
	CantBeBought    bool           `json:"cantBeBought,omitempty"`
	ShopCategory    Option[string] `json:"shopCategory,omitempty"`
	ShopSubCategory Option[string] `json:"shopSubCategory,omitempty"`
}

type CraftIngredient struct {
	Kind   CraftIngredientKind `json:"kind"`
	ID     string              `json:"id"`
	Amount int64               `json:"amount"`
}

type Recipe struct {
	BlueprintID      BlueprintID       `json:"blueprint_id"`
	Acquisition      PurchaseInfo      `json:"acquisition"`
	Ingredients      []CraftIngredient `json:"ingredients,omitempty"`
	CraftResultCount int64             `json:"craft_result_count"`
	RequiredShipRaw  []ShipID          `json:"required_ship_raw,omitempty"`
	AllowedShipIDs   []ShipID          `json:"allowed_ship_ids,omitempty"`
	RequiredNode     Option[int64]     `json:"required_node,omitempty"`
}

type CraftingInfo struct {
	DirectIngredients []CraftIngredient `json:"direct_ingredients,omitempty"`
	Recipes           []Recipe          `json:"recipes,omitempty"`
}

type EconomyInfo struct {
	Purchase       PurchaseInfo  `json:"purchase"`
	Crafting       CraftingInfo  `json:"crafting,omitempty"`
	RecraftCredits Option[int64] `json:"recraft_credits,omitempty"`
}

type DefRecord struct {
	ID             DefID         `json:"id"`
	Kind           RecordKind    `json:"kind"`
	SourceFile     ScriptPath    `json:"source_file"`
	InheritParent  Option[DefID] `json:"inherit_parent,omitempty"`
	InheritChain   []DefID       `json:"inherit_chain,omitempty"`
	LocalFields    LuaObject     `json:"local_fields"`
	ResolvedFields LuaObject     `json:"resolved_fields"`
}

type Constraints struct {
	RequiredRole         Option[ShipRoleName] `json:"required_role,omitempty"`
	ClassMask            Option[MaskValue]    `json:"class_mask,omitempty"`
	RaceMask             Option[MaskValue]    `json:"race_mask,omitempty"`
	RankMin              Option[int64]        `json:"rank_min,omitempty"`
	RankMax              Option[int64]        `json:"rank_max,omitempty"`
	RequiredShipRaw      []ShipID             `json:"required_ship_raw,omitempty"`
	RequiredShipResolved []ShipID             `json:"required_ship_resolved,omitempty"`
}

type UpgradeInfo struct {
	Prev  Option[DefID] `json:"prev,omitempty"`
	Next  Option[DefID] `json:"next,omitempty"`
	Level Option[int64] `json:"level,omitempty"`
}

type ModuleRecord struct {
	ID             DefID                   `json:"id"`
	SourceFile     ScriptPath              `json:"source_file"`
	InheritParent  Option[DefID]           `json:"inherit_parent,omitempty"`
	InheritChain   []DefID                 `json:"inherit_chain,omitempty"`
	ModuleType     Option[ModuleTypeName]  `json:"module_type,omitempty"`
	ItemSubtype    Option[ItemSubtypeName] `json:"item_subtype,omitempty"`
	Tier           Option[int64]           `json:"tier,omitempty"`
	Mark           Option[int64]           `json:"mark,omitempty"`
	VariantKind    Option[VariantKind]     `json:"variant_kind,omitempty"`
	WeaponClass    Option[WeaponClassName] `json:"weapon_class,omitempty"`
	Constraints    Constraints             `json:"constraints"`
	AllowedShipIDs []ShipID                `json:"allowed_ship_ids,omitempty"`
	Upgrade        UpgradeInfo             `json:"upgrade"`
	Economy        EconomyInfo             `json:"economy"`
}

type ShipRecord struct {
	ID             ShipID                    `json:"id"`
	SourceFile     ScriptPath                `json:"source_file"`
	InheritParent  Option[DefID]             `json:"inherit_parent,omitempty"`
	InheritChain   []DefID                   `json:"inherit_chain,omitempty"`
	ShipName       Option[string]            `json:"ship_name,omitempty"`
	ShipTier       Option[int64]             `json:"ship_tier,omitempty"`
	Rank           Option[int64]             `json:"rank,omitempty"`
	Role           Option[ShipRoleName]      `json:"role,omitempty"`
	ShipClass      Option[ShipClassName]     `json:"ship_class,omitempty"`
	Race           Option[RaceName]          `json:"race,omitempty"`
	IsPremium      bool                      `json:"is_premium,omitempty"`
	DefaultModules map[string]DefID          `json:"default_modules,omitempty"`
	SlotTypes      map[string]ModuleTypeName `json:"slot_types,omitempty"`
	Economy        EconomyInfo               `json:"economy"`
}

type BlueprintRecord struct {
	ID               BlueprintID       `json:"id"`
	SourceFile       ScriptPath        `json:"source_file"`
	CraftResult      DefID             `json:"craft_result"`
	CraftResultCount int64             `json:"craft_result_count"`
	Acquisition      PurchaseInfo      `json:"acquisition"`
	Ingredients      []CraftIngredient `json:"ingredients,omitempty"`
	RequiredShipRaw  []ShipID          `json:"required_ship_raw,omitempty"`
	AllowedShipIDs   []ShipID          `json:"allowed_ship_ids,omitempty"`
	RequiredNode     Option[int64]     `json:"required_node,omitempty"`
}

type ResourceRecord struct {
	ID          DefID                   `json:"id"`
	SourceFile  ScriptPath              `json:"source_file"`
	ItemSubtype Option[ItemSubtypeName] `json:"item_subtype,omitempty"`
	Economy     EconomyInfo             `json:"economy"`
}

type UpgradeChain struct {
	ItemSubtype ItemSubtypeName `json:"item_subtype"`
	Tier        int64           `json:"tier"`
	Items       []DefID         `json:"items"`
}

type CompatibilityIndex struct {
	ModuleToShips map[DefID][]ShipID `json:"module_to_ships"`
	ShipToModules map[ShipID][]DefID `json:"ship_to_modules"`
}

type Snapshot struct {
	Meta          Meta               `json:"meta"`
	Defs          []DefRecord        `json:"defs"`
	Ships         []ShipRecord       `json:"ships"`
	Modules       []ModuleRecord     `json:"modules"`
	Blueprints    []BlueprintRecord  `json:"blueprints"`
	Resources     []ResourceRecord   `json:"resources"`
	UpgradeChains []UpgradeChain     `json:"upgrade_chains"`
	Compatibility CompatibilityIndex `json:"compatibility"`
	Warnings      []string           `json:"warnings,omitempty"`
}
