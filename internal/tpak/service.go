package tpak

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func UnpackDirectory(inputDir string, outputDir string) (Result, error) {
	pakFiles, err := collectPakFiles(inputDir)
	if err != nil {
		return Result{}, err
	}
	if len(pakFiles) == 0 {
		return Result{}, fmt.Errorf("no %s files found in %s", archiveExtension, inputDir)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, err
	}

	for _, pakPath := range pakFiles {
		reader, err := openArchive(pakPath)
		if err != nil {
			return Result{}, err
		}

		archiveName := strings.TrimSuffix(filepath.Base(pakPath), filepath.Ext(pakPath))
		targetDir := filepath.Join(outputDir, archiveName)
		if err := reader.ExtractAll(targetDir); err != nil {
			_ = reader.Close()
			return Result{}, err
		}
		if err := writeArchiveMetadata(targetDir, reader); err != nil {
			_ = reader.Close()
			return Result{}, err
		}
		if err := reader.Close(); err != nil {
			return Result{}, err
		}
	}

	return Result{
		ArchiveCount: len(pakFiles),
		OutputDir:    outputDir,
	}, nil
}

func PackDirectory(inputDir string, outputDir string) (Result, error) {
	archives, err := collectArchiveDirectories(inputDir)
	if err != nil {
		return Result{}, err
	}
	if len(archives) == 0 {
		return Result{}, fmt.Errorf("no archive directories found in %s", inputDir)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, err
	}

	for _, archiveDir := range archives {
		archiveName := filepath.Base(archiveDir)
		targetPath := filepath.Join(outputDir, archiveName+archiveExtension)
		if err := packArchiveDirectory(archiveDir, targetPath); err != nil {
			return Result{}, err
		}
	}

	return Result{
		ArchiveCount: len(archives),
		OutputDir:    outputDir,
	}, nil
}

func collectPakFiles(inputDir string) ([]string, error) {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), archiveExtension) {
			continue
		}
		paths = append(paths, filepath.Join(inputDir, entry.Name()))
	}

	sort.Strings(paths)
	return paths, nil
}

func collectArchiveDirectories(inputDir string) ([]string, error) {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(inputDir, entry.Name()))
	}

	sort.Strings(paths)
	return paths, nil
}

func outputPath(rootDir string, archivePath string) string {
	normalized := relativePathFromArchive(archivePath)
	return filepath.Join(rootDir, filepath.FromSlash(normalized))
}
