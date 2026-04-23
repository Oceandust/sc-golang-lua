package loader

import (
	"path/filepath"
	"reflect"
	"strings"

	"defgraph/internal/luavalue"
)

var bootstrapScripts = []string{
	bootstrapAIConstants,
	bootstrapAISpellConstants,
	bootstrapCosmosConstants,
	bootstrapAIAchievementConstants,
	bootstrapAIQuestConstants,
	bootstrapAISituationGlobals,
	bootstrapAIAdventureGlobals,
	bootstrapMasterServer,
	bootstrapItemSubtypes,
	bootstrapDesignersDefs,
}

const (
	bootstrapAIConstants            = "scripts/ai/constants.lua"
	bootstrapAISpellConstants       = "scripts/ai/spellconstants.lua"
	bootstrapCosmosConstants        = "scripts/ai/cosmos_constants.lua"
	bootstrapAIAchievementConstants = "scripts/ai/achievementconstants.lua"
	bootstrapAIQuestConstants       = "scripts/ai/questconstants.lua"
	bootstrapAISituationGlobals     = "scripts/ai/situation_globals.lua"
	bootstrapAIAdventureGlobals     = "scripts/ai/adventure_globals.lua"
	bootstrapMasterServer           = "scripts/masterserver.lua"
	bootstrapItemSubtypes           = "gamedata/shared/item_subtypes.lua"
	bootstrapDesignersDefs          = "gamedata/def/designersdefs.lua"
	moduleUpgradeChainScript        = "gamedata/shared/module_upgrade_chain.lua"
)

func LoadedManifest() []string {
	return append([]string(nil), bootstrapScripts...)
}

func NormalizeRepoRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		root = ".."
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}

	return filepath.Clean(absolute)
}

func NormalizeCompiledRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}

	return filepath.Clean(absolute)
}

func ResolveCompiledPath(compiledRoot string, logicalPath string) string {
	return filepath.Join(compiledRoot, filepath.FromSlash(normalizeLogicalPath(logicalPath)))
}

func normalizeLogicalPath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

func diffObjectFields(baseline luavalue.Object, current luavalue.Object) luavalue.Object {
	out := luavalue.NewObject()

	current.Range(func(key string, value luavalue.Value) bool {
		baselineValue, ok := baseline.Get(key)
		if !ok {
			out.Set(key, value.Clone())
			return true
		}

		currentObject, currentIsObject := value.AsObject()
		baselineObject, baselineIsObject := baselineValue.AsObject()
		if currentIsObject && baselineIsObject {
			nested := diffObjectFields(baselineObject, currentObject)
			if nested.Len() > 0 {
				out.Set(key, luavalue.ObjectValue(nested))
			}
			return true
		}

		if !reflect.DeepEqual(baselineValue, value) {
			out.Set(key, value.Clone())
		}
		return true
	})

	return out
}
