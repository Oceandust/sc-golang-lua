package tpak

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
)

const (
	defaultUnpackThreads = 4
)

type parsedArchive struct {
	Path      string
	TargetDir string
	Reader    *archiveReader
}

func UnpackDirectory(inputDir string, outputDir string) (Result, error) {
	return UnpackDirectoryWithOptions(inputDir, outputDir, nil)
}

func UnpackDirectoryWithOptions(inputDir string, outputDir string, options *UnpackOptions) (Result, error) {
	unpackOptions, err := normalizeUnpackOptions(options)
	if err != nil {
		return Result{}, err
	}

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

	archives := make([]parsedArchive, len(pakFiles))
	group := errgroup.Group{}
	group.SetLimit(unpackOptions.Threads)
	for index := range pakFiles {
		index := index
		pakPath := pakFiles[index]
		group.Go(func() error {
			reader, err := openArchive(pakPath)
			if err != nil {
				return err
			}

			archiveName := strings.TrimSuffix(filepath.Base(pakPath), filepath.Ext(pakPath))
			archives[index] = parsedArchive{
				Path:      pakPath,
				TargetDir: filepath.Join(outputDir, archiveName),
				Reader:    reader,
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return Result{}, errors.Join(err, closeParsedArchives(archives))
	}

	for index := range archives {
		if err := unpackParsedArchive(&archives[index], unpackOptions); err != nil {
			return Result{}, errors.Join(err, closeParsedArchives(archives))
		}
	}

	return Result{
		ArchiveCount: len(pakFiles),
		OutputDir:    outputDir,
	}, nil
}

func UnpackFileWithOptions(inputPak string, outputDir string, options *UnpackOptions) (Result, error) {
	unpackOptions, err := normalizeUnpackOptions(options)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, err
	}

	reader, err := openArchive(inputPak)
	if err != nil {
		return Result{}, err
	}
	archive := parsedArchive{
		Path:      inputPak,
		TargetDir: outputDir,
		Reader:    reader,
	}

	if err := unpackParsedArchive(&archive, unpackOptions); err != nil {
		return Result{}, errors.Join(err, archive.Reader.Close())
	}

	return Result{
		ArchiveCount: 1,
		OutputDir:    outputDir,
	}, nil
}

func normalizeUnpackOptions(options *UnpackOptions) (UnpackOptions, error) {
	unpackOptions := UnpackOptions{
		Threads: defaultUnpackThreads,
	}
	if options != nil {
		unpackOptions = *options
	}
	if unpackOptions.Threads == 0 {
		unpackOptions.Threads = defaultUnpackThreads
	}
	if unpackOptions.Threads < 1 {
		return UnpackOptions{}, fmt.Errorf("tpak unpack threads must be at least 1")
	}
	return unpackOptions, nil
}

func unpackParsedArchive(archive *parsedArchive, options UnpackOptions) error {
	if archive == nil || archive.Reader == nil {
		return fmt.Errorf("archive was not parsed")
	}
	if err := archive.Reader.ExtractAll(archive.TargetDir, options.Threads); err != nil {
		return err
	}
	return archive.Reader.Close()
}

func closeParsedArchives(archives []parsedArchive) error {
	var closeErr error
	for index := range archives {
		if archives[index].Reader == nil {
			continue
		}
		closeErr = errors.Join(closeErr, archives[index].Reader.Close())
	}
	return closeErr
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

	for index := range archives {
		archiveDir := archives[index]
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
	entries, err := readDirectoryEntries(inputDir)
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
	return paths, nil
}

func collectArchiveDirectories(inputDir string) ([]string, error) {
	entries, err := readDirectoryEntries(inputDir)
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
	return paths, nil
}

func readDirectoryEntries(path string) ([]os.DirEntry, error) {
	directory, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	return entries, errors.Join(readErr, closeErr)
}

func outputPath(rootDir string, archivePath string) string {
	normalized := relativePathFromArchive(archivePath)
	return filepath.Join(rootDir, filepath.FromSlash(normalized))
}
