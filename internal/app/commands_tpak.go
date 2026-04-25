package app

import (
	"encoding/json"
	"fmt"
	"os"

	"sc_cli/internal/tpak"
)

type tpakPackCmd struct {
	InputDir  ExistingDir `arg:"" help:"Directory containing one top-level directory per archive to pack."`
	OutputDir OutputDir   `arg:"" help:"Directory that will receive the generated .pak files."`
}

type tpakUnpackCmd struct {
	Threads   int         `help:"Number of concurrent file extraction workers per archive." default:"4"`
	InputDir  ExistingDir `arg:"" help:"Directory containing .pak files to unpack."`
	OutputDir OutputDir   `arg:"" help:"Directory that will receive one top-level directory per archive."`
}

type tpakUnpackFileCmd struct {
	Threads   int          `help:"Number of concurrent file extraction workers." default:"4"`
	InputPak  ExistingFile `arg:"" help:"TPAK archive to unpack."`
	OutputDir OutputDir    `arg:"" help:"Directory that will receive the unpacked archive contents."`
}

type tpakInspectCmd struct {
	InputPak ExistingFile `arg:"" help:"TPAK archive to inspect."`
	Format   outputFormat `help:"Output format." enum:"text,json" default:"text"`
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
	result, err := tpak.UnpackDirectoryWithOptions(command.InputDir.Path(), command.OutputDir.Path(), &tpak.UnpackOptions{
		Threads: command.Threads,
	})
	if err != nil {
		return err
	}

	fmt.Printf("unpacked %d archives into %s\n", result.ArchiveCount, result.OutputDir)
	return nil
}

func (command *tpakUnpackFileCmd) Run(_ *App) error {
	result, err := tpak.UnpackFileWithOptions(command.InputPak.Path(), command.OutputDir.Path(), &tpak.UnpackOptions{
		Threads: command.Threads,
	})
	if err != nil {
		return err
	}

	fmt.Printf("unpacked %d archive into %s\n", result.ArchiveCount, result.OutputDir)
	return nil
}

func (command *tpakInspectCmd) Run(_ *App) error {
	inspection, err := tpak.InspectArchive(command.InputPak.Path())
	if err != nil {
		return err
	}

	if command.Format == outputFormatJSON {
		data, err := json.MarshalIndent(inspection, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	return tpak.WriteArchiveInspectionText(os.Stdout, inspection)
}
