package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sc_cli/internal/defgraph"
)

type snapshotCmd struct {
	Export snapshotExportCmd `cmd:"" help:"Export the normalized snapshot."`
}

type snapshotExportCmd struct {
	Root         ExistingDir `help:"Repo root to load." default:".."`
	CompiledRoot ExistingDir `help:"Compiled snapshot root to execute." required:""`
	Out          OutputFile  `help:"Output snapshot path." default:"out/sc_cli.snapshot.json"`
}

type defsCmd struct {
	Inspect defsInspectCmd `cmd:"" help:"Inspect a def and its normalized records."`
	Check   defsCheckCmd   `cmd:"" help:"Validate the loader against known fixtures."`
}

type defsInspectCmd struct {
	Root         ExistingDir  `help:"Repo root to load." default:".."`
	CompiledRoot ExistingDir  `help:"Compiled snapshot root to execute." required:""`
	ID           string       `help:"Definition ID to inspect." required:""`
	Format       outputFormat `help:"Output format." enum:"text,json" default:"text"`
}

type defsCheckCmd struct {
	Root         ExistingDir `help:"Repo root to load." default:".."`
	CompiledRoot ExistingDir `help:"Compiled snapshot root to execute." required:""`
}

type inspectBundle struct {
	Def       defgraph.Option[defgraph.DefRecord]       `json:"def,omitempty"`
	Module    defgraph.Option[defgraph.ModuleRecord]    `json:"module,omitempty"`
	Ship      defgraph.Option[defgraph.ShipRecord]      `json:"ship,omitempty"`
	Blueprint defgraph.Option[defgraph.BlueprintRecord] `json:"blueprint,omitempty"`
	Resource  defgraph.Option[defgraph.ResourceRecord]  `json:"resource,omitempty"`
}

func (bundle inspectBundle) empty() bool {
	return bundle.Def.IsAbsent() &&
		bundle.Module.IsAbsent() &&
		bundle.Ship.IsAbsent() &&
		bundle.Blueprint.IsAbsent() &&
		bundle.Resource.IsAbsent()
}

func (command *snapshotExportCmd) Run(app *App) error {
	snapshotData, err := app.defgraph.LoadSnapshot(command.Root.Path(), command.CompiledRoot.Path())
	if err != nil {
		return err
	}

	outPath := command.Out.Path()
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(".", outPath)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(snapshotData, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return err
	}

	fmt.Printf(
		"exported %d defs, %d ships, %d modules, %d blueprints to %s\n",
		len(snapshotData.Defs),
		len(snapshotData.Ships),
		len(snapshotData.Modules),
		len(snapshotData.Blueprints),
		filepath.Clean(outPath),
	)
	return nil
}

func (command *defsInspectCmd) Run(app *App) error {
	snapshotData, err := app.defgraph.LoadSnapshot(command.Root.Path(), command.CompiledRoot.Path())
	if err != nil {
		return err
	}

	targetID := defgraph.DefID(command.ID)
	shipID := defgraph.ShipID(command.ID)
	blueprintID := defgraph.BlueprintID(command.ID)

	var bundle inspectBundle
	for _, item := range snapshotData.Defs {
		if item.ID == targetID {
			bundle.Def = defgraph.Some(item)
			break
		}
	}
	for _, item := range snapshotData.Modules {
		if item.ID == targetID {
			bundle.Module = defgraph.Some(item)
			break
		}
	}
	for _, item := range snapshotData.Ships {
		if item.ID == shipID {
			bundle.Ship = defgraph.Some(item)
			break
		}
	}
	for _, item := range snapshotData.Blueprints {
		if item.ID == blueprintID {
			bundle.Blueprint = defgraph.Some(item)
			break
		}
	}
	for _, item := range snapshotData.Resources {
		if item.ID == targetID {
			bundle.Resource = defgraph.Some(item)
			break
		}
	}

	if bundle.empty() {
		return fmt.Errorf("id %s not found", command.ID)
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}

	if command.Format == outputFormatJSON {
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("id: %s\n", command.ID)
	if bundle.Def.IsPresent() {
		fmt.Println("record: def")
	}
	if bundle.Module.IsPresent() {
		fmt.Println("record: module")
	}
	if bundle.Ship.IsPresent() {
		fmt.Println("record: ship")
	}
	if bundle.Blueprint.IsPresent() {
		fmt.Println("record: blueprint")
	}
	if bundle.Resource.IsPresent() {
		fmt.Println("record: resource")
	}
	fmt.Println(string(data))
	return nil
}

func (command *defsCheckCmd) Run(app *App) error {
	snapshotData, err := app.defgraph.LoadSnapshot(command.Root.Path(), command.CompiledRoot.Path())
	if err != nil {
		return err
	}

	if err := defgraph.ValidateSnapshot(snapshotData); err != nil {
		return err
	}

	fmt.Printf(
		"check passed: %d defs, %d ships, %d modules, %d blueprints\n",
		len(snapshotData.Defs),
		len(snapshotData.Ships),
		len(snapshotData.Modules),
		len(snapshotData.Blueprints),
	)
	return nil
}
