package defgraph

import (
	"fmt"
	"slices"
	"strings"
)

func ValidateSnapshot(snapshot *Snapshot) error {
	defIndex := make(map[DefID]DefRecord, len(snapshot.Defs))
	moduleIndex := make(map[DefID]ModuleRecord, len(snapshot.Modules))
	shipIndex := make(map[ShipID]ShipRecord, len(snapshot.Ships))
	blueprintIndex := make(map[BlueprintID]BlueprintRecord, len(snapshot.Blueprints))

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

	weapon, ok := moduleIndex[DefID("Weapon_DoubleThermalBlastgun_T5_Rare")]
	if !ok {
		return fmt.Errorf("missing Weapon_DoubleThermalBlastgun_T5_Rare")
	}
	if weapon.InheritParent.OrElse("") != DefID("Weapon_DoubleThermalBlastGun") {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare inherit_parent mismatch")
	}
	if def, ok := defIndex[weapon.ID]; !ok || !slices.Contains(def.InheritChain, DefID("Weapon_AutoGun")) {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare inheritance chain missing Weapon_AutoGun")
	}
	if weapon.WeaponClass.OrElse("") != WeaponClassLaser {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare weapon_class mismatch")
	}
	if weapon.Constraints.RequiredRole.OrElse("") != ShipRoleSniper {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare required_role mismatch")
	}

	classMask, ok := weapon.Constraints.ClassMask.Get()
	if !ok || !slices.Contains(classMask.Flags, string(ShipClassLarge)) {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare class_mask missing LARGE")
	}
	if !strings.HasSuffix(weapon.Upgrade.Prev.OrElse(DefID("")).String(), "_Mk1") {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare prev upgrade mismatch")
	}
	if !strings.HasSuffix(weapon.Upgrade.Next.OrElse(DefID("")).String(), "_Mk3") {
		return fmt.Errorf("Weapon_DoubleThermalBlastgun_T5_Rare next upgrade mismatch")
	}

	ship, ok := shipIndex[ShipID("Ship_Race1_L_T5_Sniper")]
	if !ok {
		return fmt.Errorf("missing Ship_Race1_L_T5_Sniper")
	}
	if ship.Role.OrElse("") != ShipRoleSniper {
		return fmt.Errorf("Ship_Race1_L_T5_Sniper role mismatch")
	}
	if ship.ShipClass.OrElse("") != ShipClassLarge {
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

	blueprint, ok := blueprintIndex[BlueprintID("BP_Weapon_FrontLineCannon_T3_Mk1")]
	if !ok {
		return fmt.Errorf("missing BP_Weapon_FrontLineCannon_T3_Mk1")
	}
	if blueprint.CraftResult != DefID("Weapon_FrontLineCannon_T3_Mk1") {
		return fmt.Errorf("BP_Weapon_FrontLineCannon_T3_Mk1 craft_result mismatch")
	}
	if len(blueprint.Ingredients) == 0 {
		return fmt.Errorf("BP_Weapon_FrontLineCannon_T3_Mk1 missing ingredients")
	}

	heatingBlueprint, ok := blueprintIndex[BlueprintID("BP_Weapon_HeatingGun_T3_Mk1")]
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
