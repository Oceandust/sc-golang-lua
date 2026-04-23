package snapshot

import (
	"fmt"
	"slices"
	"strings"

	"defgraph/internal/types"
)

func Validate(snapshot *types.Snapshot) error {
	defIndex := make(map[types.DefID]types.DefRecord, len(snapshot.Defs))
	moduleIndex := make(map[types.DefID]types.ModuleRecord, len(snapshot.Modules))
	shipIndex := make(map[types.ShipID]types.ShipRecord, len(snapshot.Ships))
	blueprintIndex := make(map[types.BlueprintID]types.BlueprintRecord, len(snapshot.Blueprints))

	for _, item := range snapshot.Defs {
		defIndex[item.ID] = item
	}
	for _, item := range snapshot.Modules {
		moduleIndex[item.ID] = item
	}
	for _, item := range snapshot.Ships {
		shipIndex[item.ID] = item
	}
	for _, item := range snapshot.Blueprints {
		blueprintIndex[item.ID] = item
	}

	weapon, ok := moduleIndex[types.DefID("Weapon_DoubleThermalBlastgun_T5_Rare")]
	if !ok {
		return fmt.Errorf("missing Weapon_DoubleThermalBlastgun_T5_Rare")
	}
	if weapon.InheritParent.OrElse("") != types.DefID("Weapon_DoubleThermalBlastGun") {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare inherit_parent mismatch")
	}
	if def, ok := defIndex[weapon.ID]; !ok || !slices.Contains(def.InheritChain, types.DefID("Weapon_AutoGun")) {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare inheritance chain missing Weapon_AutoGun")
	}
	if weapon.WeaponClass.OrElse("") != types.WeaponClassLaser {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare weapon_class mismatch")
	}
	if weapon.Constraints.RequiredRole.OrElse("") != types.ShipRoleSniper {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare required_role mismatch")
	}

	classMask, ok := weapon.Constraints.ClassMask.Get()
	if !ok || !slices.Contains(classMask.Flags, string(types.ShipClassLarge)) {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare class_mask missing LARGE")
	}
	if !strings.HasSuffix(weapon.Upgrade.Prev.OrElse(types.DefID("")).String(), "_Mk1") {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare prev upgrade mismatch")
	}
	if !strings.HasSuffix(weapon.Upgrade.Next.OrElse(types.DefID("")).String(), "_Mk3") {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare next upgrade mismatch")
	}

	ship, ok := shipIndex[types.ShipID("Ship_Race1_L_T5_Sniper")]
	if !ok {
		return fmt.Errorf("missing Ship_Race1_L_T5_Sniper")
	}
	if ship.Role.OrElse("") != types.ShipRoleSniper {
		return fmt.Errorf("Ship_Race1_L_T5_Sniper role mismatch")
	}
	if ship.ShipClass.OrElse("") != types.ShipClassLarge {
		return fmt.Errorf("Ship_Race1_L_T5_Sniper ship_class mismatch")
	}
	if ship.Economy.Purchase.Price.OrElse(0) == 0 {
		return fmt.Errorf("Ship_Race1_L_T5_Sniper missing purchase price")
	}
	if len(ship.Economy.Crafting.DirectIngredients) == 0 {
		return fmt.Errorf("Ship_Race1_L_T5_Sniper missing craftable reagents")
	}
	if ship.Economy.RecraftCredits.OrElse(0) == 0 {
		return fmt.Errorf("Ship_Race1_L_T5_Sniper missing recraft credits")
	}

	blueprint, ok := blueprintIndex[types.BlueprintID("BP_Weapon_FrontLineCannon_T3_Mk1")]
	if !ok {
		return fmt.Errorf("missing BP_Weapon_FrontLineCannon_T3_Mk1")
	}
	if blueprint.CraftResult != types.DefID("Weapon_FrontLineCannon_T3_Mk1") {
		return fmt.Errorf("BP_Weapon_FrontLineCannon_T3_Mk1 craft_result mismatch")
	}
	if len(blueprint.Ingredients) == 0 {
		return fmt.Errorf("BP_Weapon_FrontLineCannon_T3_Mk1 missing ingredients")
	}

	heatingBlueprint, ok := blueprintIndex[types.BlueprintID("BP_Weapon_HeatingGun_T3_Mk1")]
	if !ok {
		return fmt.Errorf("missing BP_Weapon_HeatingGun_T3_Mk1")
	}
	if heatingBlueprint.Acquisition.Price.OrElse(0) == 0 {
		return fmt.Errorf("BP_Weapon_HeatingGun_T3_Mk1 missing acquisition price")
	}

	if len(weapon.Constraints.RequiredShipRaw) == 0 {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare missing raw required_ship")
	}
	if len(weapon.Constraints.RequiredShipResolved) == 0 {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare missing resolved required_ship")
	}

	if len(snapshot.Modules) == 0 || len(snapshot.Ships) == 0 || len(snapshot.Defs) == 0 {
		return fmt.Errorf("snapshot is empty")
	}

	return nil
}
