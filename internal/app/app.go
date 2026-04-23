package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"defgraph/internal/snapshot"
	"defgraph/internal/tpak"
	"defgraph/internal/types"
	"defgraph/internal/yup"

	"github.com/alecthomas/kong"
)

type defgraphService interface {
	LoadSnapshot(repoRoot string, compiledRoot string) (*types.Snapshot, error)
}

type App struct {
	defgraph defgraphService
}

type CLI struct {
	Snapshot   snapshotCmd   `cmd:"" help:"Snapshot commands."`
	Defs       defsCmd       `cmd:"" help:"Definition inspection and validation commands."`
	TPAKPack   tpakPackCmd   `cmd:"" name:"tpak-pack" help:"Pack a mirrored directory tree into a set of TPAK archives."`
	TPAKUnpack tpakUnpackCmd `cmd:"" name:"tpak-unpack" help:"Unpack a directory of TPAK archives into mirrored directories."`
	YUPRewrite yupRewriteCmd `cmd:"" name:"yup-rewrite" help:"Rewrite a Star Conflict yup manifest with updated file metadata."`
}

type snapshotCmd struct {
	Export snapshotExportCmd `cmd:"" help:"Export the normalized snapshot."`
}

type snapshotExportCmd struct {
	Root         ExistingDir `help:"Repo root to load." default:".."`
	CompiledRoot ExistingDir `help:"Compiled snapshot root to execute." required:""`
	Out          OutputFile  `help:"Output snapshot path." default:"out/defgraph.snapshot.json"`
}

type defsCmd struct {
	Inspect defsInspectCmd `cmd:"" help:"Inspect a def and its normalized records."`
	Check   defsCheckCmd   `cmd:"" help:"Validate the loader against known fixtures."`
}

type defsInspectCmd struct {
	Root         ExistingDir        `help:"Repo root to load." default:".."`
	CompiledRoot ExistingDir        `help:"Compiled snapshot root to execute." required:""`
	ID           string             `help:"Definition ID to inspect." required:""`
	Format       types.OutputFormat `help:"Output format." enum:"text,json" default:"text"`
}

type defsCheckCmd struct {
	Root         ExistingDir `help:"Repo root to load." default:".."`
	CompiledRoot ExistingDir `help:"Compiled snapshot root to execute." required:""`
}

type tpakPackCmd struct {
	InputDir  ExistingDir `arg:"" help:"Directory containing one top-level directory per archive to pack."`
	OutputDir OutputDir   `arg:"" help:"Directory that will receive the generated .pak files."`
}

type tpakUnpackCmd struct {
	InputDir  ExistingDir `arg:"" help:"Directory containing .pak files to unpack."`
	OutputDir OutputDir   `arg:"" help:"Directory that will receive one top-level directory per archive."`
}

type yupRewriteCmd struct {
	Manifest ExistingFile `arg:"" help:"Manifest file to rewrite."`
	RootDir  ExistingDir  `arg:"" help:"Root directory used to resolve manifest paths."`
	Out      string       `help:"Optional output manifest path; defaults to overwriting the input file."`
}

type inspectBundle struct {
	Def       types.Option[types.DefRecord]       `json:"def,omitempty"`
	Module    types.Option[types.ModuleRecord]    `json:"module,omitempty"`
	Ship      types.Option[types.ShipRecord]      `json:"ship,omitempty"`
	Blueprint types.Option[types.BlueprintRecord] `json:"blueprint,omitempty"`
	Resource  types.Option[types.ResourceRecord]  `json:"resource,omitempty"`
}

func (bundle inspectBundle) empty() bool {
	return bundle.Def.IsAbsent() &&
		bundle.Module.IsAbsent() &&
		bundle.Ship.IsAbsent() &&
		bundle.Blueprint.IsAbsent() &&
		bundle.Resource.IsAbsent()
}

func Run() error {
	application := &App{
		defgraph: newDefgraphService(),
	}

	cli := CLI{}
	parser, err := kong.New(
		&cli,
		kong.Name("defgraph"),
		kong.Description("Multi-tool CLI for defgraph and TPAK workflows."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	if err != nil {
		return err
	}

	ctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	return ctx.Run(application)
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

	targetID := types.DefID(command.ID)
	shipID := types.ShipID(command.ID)
	blueprintID := types.BlueprintID(command.ID)

	var bundle inspectBundle
	for _, item := range snapshotData.Defs {
		if item.ID == targetID {
			bundle.Def = types.Some(item)
			break
		}
	}
	for _, item := range snapshotData.Modules {
		if item.ID == targetID {
			bundle.Module = types.Some(item)
			break
		}
	}
	for _, item := range snapshotData.Ships {
		if item.ID == shipID {
			bundle.Ship = types.Some(item)
			break
		}
	}
	for _, item := range snapshotData.Blueprints {
		if item.ID == blueprintID {
			bundle.Blueprint = types.Some(item)
			break
		}
	}
	for _, item := range snapshotData.Resources {
		if item.ID == targetID {
			bundle.Resource = types.Some(item)
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

	if command.Format == types.OutputFormatJSON {
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

	if err := snapshot.Validate(snapshotData); err != nil {
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

func (command *tpakPackCmd) Run(_ *App) error {
	result, err := tpak.PackDirectory(command.InputDir.Path(), command.OutputDir.Path())
	if err != nil {
		return err
	}

	fmt.Printf("packed %d archives into %s\n", result.ArchiveCount, result.OutputDir)
	return nil
}

func (command *tpakUnpackCmd) Run(_ *App) error {
	result, err := tpak.UnpackDirectory(command.InputDir.Path(), command.OutputDir.Path())
	if err != nil {
		return err
	}

	fmt.Printf("unpacked %d archives into %s\n", result.ArchiveCount, result.OutputDir)
	return nil
}

func (command *yupRewriteCmd) Run(_ *App) error {
	outputPath := command.Out
	if outputPath != "" && !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(".", outputPath)
	}

	result, err := yup.RewriteManifest(command.Manifest.Path(), command.RootDir.Path(), outputPath)
	if err != nil {
		return err
	}

	fmt.Printf("rewrote %d manifest entries into %s\n", result.UpdatedEntries, result.OutputPath)
	return nil
}
