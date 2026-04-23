package types

import (
	"encoding"
	"encoding/json"
	"fmt"
)

type DefID string
type ShipID string
type BlueprintID string
type ScriptPath string
type SchemaVersion string

func (id DefID) String() string {
	return string(id)
}

func (id ShipID) String() string {
	return string(id)
}

func (id BlueprintID) String() string {
	return string(id)
}

func (path ScriptPath) String() string {
	return string(path)
}

const (
	SchemaVersionV1 SchemaVersion = "1"

	DontInheritSentinel = "__DEFGRAPH_DONT_INHERIT__"
)

type RecordKind string

const (
	RecordKindOther     RecordKind = "other"
	RecordKindModule    RecordKind = "module"
	RecordKindShip      RecordKind = "ship"
	RecordKindBlueprint RecordKind = "blueprint"
	RecordKindResource  RecordKind = "resource"
)

type LoaderName string

const (
	LoaderNameGoLua LoaderName = "golua"
)

type LoaderRuntime string

const (
	LoaderRuntimeLuaJIT LoaderRuntime = "luajit"
)

type OutputFormat string

const (
	OutputFormatText OutputFormat = "text"
	OutputFormatJSON OutputFormat = "json"
)

type VariantKind string

const (
	VariantKindMk1        VariantKind = "mk1"
	VariantKindRare       VariantKind = "rare"
	VariantKindMk3        VariantKind = "mk3"
	VariantKindEpic       VariantKind = "epic"
	VariantKindRelic      VariantKind = "relic"
	VariantKindPremium    VariantKind = "premium"
	VariantKindTournament VariantKind = "tournament"
)

type CraftIngredientKind string

const (
	CraftIngredientKindItem     CraftIngredientKind = "item"
	CraftIngredientKindResource CraftIngredientKind = "resource"
	CraftIngredientKindCurrency CraftIngredientKind = "currency"
)

type ModuleTypeName string
type ItemSubtypeName string
type ShipRoleName string
type ShipClassName string
type WeaponClassName string
type RaceName string

const (
	ShipRoleSniper ShipRoleName = "SNIPER"

	ShipClassLarge ShipClassName = "LARGE"

	WeaponClassLaser WeaponClassName = "LASER"
)

var (
	_ encoding.TextMarshaler   = (*OutputFormat)(nil)
	_ encoding.TextUnmarshaler = (*OutputFormat)(nil)
	_ json.Marshaler           = (*RecordKind)(nil)
	_ json.Unmarshaler         = (*RecordKind)(nil)
	_ json.Marshaler           = (*LoaderName)(nil)
	_ json.Unmarshaler         = (*LoaderName)(nil)
	_ json.Marshaler           = (*LoaderRuntime)(nil)
	_ json.Unmarshaler         = (*LoaderRuntime)(nil)
	_ json.Marshaler           = (*OutputFormat)(nil)
	_ json.Unmarshaler         = (*OutputFormat)(nil)
	_ json.Marshaler           = (*VariantKind)(nil)
	_ json.Unmarshaler         = (*VariantKind)(nil)
	_ json.Marshaler           = (*CraftIngredientKind)(nil)
	_ json.Unmarshaler         = (*CraftIngredientKind)(nil)
)

func (value RecordKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

func (value *RecordKind) UnmarshalJSON(data []byte) error {
	return unmarshalStringEnum(data, []RecordKind{
		RecordKindOther,
		RecordKindModule,
		RecordKindShip,
		RecordKindBlueprint,
		RecordKindResource,
	}, value, "RecordKind")
}

func (value LoaderName) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

func (value *LoaderName) UnmarshalJSON(data []byte) error {
	return unmarshalStringEnum(data, []LoaderName{LoaderNameGoLua}, value, "LoaderName")
}

func (value LoaderRuntime) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

func (value *LoaderRuntime) UnmarshalJSON(data []byte) error {
	return unmarshalStringEnum(data, []LoaderRuntime{LoaderRuntimeLuaJIT}, value, "LoaderRuntime")
}

func (value OutputFormat) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

func (value *OutputFormat) UnmarshalJSON(data []byte) error {
	return unmarshalStringEnum(data, []OutputFormat{OutputFormatText, OutputFormatJSON}, value, "OutputFormat")
}

func (value OutputFormat) MarshalText() ([]byte, error) {
	return []byte(value), nil
}

func (value *OutputFormat) UnmarshalText(text []byte) error {
	return unmarshalTextEnum(text, []OutputFormat{OutputFormatText, OutputFormatJSON}, value, "OutputFormat")
}

func (value VariantKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

func (value *VariantKind) UnmarshalJSON(data []byte) error {
	return unmarshalStringEnum(data, []VariantKind{
		VariantKindMk1,
		VariantKindRare,
		VariantKindMk3,
		VariantKindEpic,
		VariantKindRelic,
		VariantKindPremium,
		VariantKindTournament,
	}, value, "VariantKind")
}

func (value CraftIngredientKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(value))
}

func (value *CraftIngredientKind) UnmarshalJSON(data []byte) error {
	return unmarshalStringEnum(data, []CraftIngredientKind{
		CraftIngredientKindItem,
		CraftIngredientKindResource,
		CraftIngredientKindCurrency,
	}, value, "CraftIngredientKind")
}

func unmarshalStringEnum[T ~string](data []byte, allowed []T, target *T, name string) error {
	if target == nil {
		return nil
	}

	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	value := T(raw)
	if !containsEnum(allowed, value) {
		return fmt.Errorf("invalid %s %q", name, raw)
	}

	*target = value
	return nil
}

func unmarshalTextEnum[T ~string](text []byte, allowed []T, target *T, name string) error {
	if target == nil {
		return nil
	}

	value := T(string(text))
	if !containsEnum(allowed, value) {
		return fmt.Errorf("invalid %s %q", name, string(text))
	}

	*target = value
	return nil
}

func containsEnum[T comparable](values []T, target T) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
