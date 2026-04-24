package app

import (
	"fmt"
	"path/filepath"

	"sc_cli/internal/yup"
)

type yupRewriteCmd struct {
	Manifest ExistingFile `arg:"" help:"Manifest file to rewrite."`
	RootDir  ExistingDir  `arg:"" help:"Root directory used to resolve manifest paths."`
	Out      string       `help:"Optional output manifest path; defaults to overwriting the input file."`
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
