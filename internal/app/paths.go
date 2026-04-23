package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
)

type ExistingDir string
type ExistingFile string
type OutputDir string
type OutputFile string

func (value *ExistingDir) Decode(ctx *kong.DecodeContext) error {
	path, err := decodePathValue(ctx, "directory")
	if err != nil {
		return err
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		return statErr
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	*value = ExistingDir(path)
	return nil
}

func (value *ExistingFile) Decode(ctx *kong.DecodeContext) error {
	path, err := decodePathValue(ctx, "file")
	if err != nil {
		return err
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		return statErr
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}

	*value = ExistingFile(path)
	return nil
}

func (value *OutputDir) Decode(ctx *kong.DecodeContext) error {
	path, err := decodePathValue(ctx, "directory")
	if err != nil {
		return err
	}

	*value = OutputDir(path)
	return nil
}

func (value *OutputFile) Decode(ctx *kong.DecodeContext) error {
	path, err := decodePathValue(ctx, "file")
	if err != nil {
		return err
	}

	*value = OutputFile(path)
	return nil
}

func (value ExistingDir) Path() string  { return string(value) }
func (value ExistingFile) Path() string { return string(value) }
func (value OutputDir) Path() string    { return string(value) }
func (value OutputFile) Path() string   { return string(value) }

func decodePathValue(ctx *kong.DecodeContext, label string) (string, error) {
	var raw string
	if err := ctx.Scan.PopValueInto(label, &raw); err != nil {
		return "", err
	}

	path := strings.TrimSpace(raw)
	if path == "" {
		return "", fmt.Errorf("%s path is required", label)
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path), nil
	}
	return filepath.Clean(absolute), nil
}
