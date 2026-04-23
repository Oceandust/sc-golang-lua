package types

import (
	"sort"

	"defgraph/internal/collections"
)

type EnumTable struct {
	Name    string
	ByName  *collections.HashMap[string, int64]
	ByValue *collections.HashMap[int64, string]
}

func (table EnumTable) ValueOf(name string) Option[int64] {
	if table.ByName == nil {
		return None[int64]()
	}

	value, ok := table.ByName.Get(name)
	if !ok {
		return None[int64]()
	}

	return Some(value)
}

func (table EnumTable) NameOf(value int64) Option[string] {
	if table.ByValue == nil {
		return None[string]()
	}

	name, ok := table.ByValue.Get(value)
	if !ok {
		return None[string]()
	}

	return Some(name)
}

func (table EnumTable) FlagsOf(mask int64) []string {
	type pair struct {
		name  string
		value int64
	}

	pairs := make([]pair, 0)
	if table.ByName != nil {
		table.ByName.Range(func(name string, value int64) bool {
			pairs = append(pairs, pair{name: name, value: value})
			return true
		})
	}

	sort.Slice(pairs, func(left int, right int) bool {
		if pairs[left].value == pairs[right].value {
			return pairs[left].name < pairs[right].name
		}

		return pairs[left].value < pairs[right].value
	})

	flags := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		if pair.value == 0 {
			continue
		}

		if mask&pair.value == pair.value {
			flags = append(flags, pair.name)
		}
	}

	return flags
}

type EnumRegistry struct {
	ItemSubtype         EnumTable
	ModuleType          EnumTable
	ShipClass           EnumTable
	ShipRoles           EnumTable
	WeaponClass         EnumTable
	Race                EnumTable
	RaceMask            EnumTable
	SpaceShipModuleSlot EnumTable
}
