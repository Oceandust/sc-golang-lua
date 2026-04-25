package tpak

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"sc_cli/internal/collections"
)

func packArchiveDirectory(sourceDir string, outputPath string) error {
	files, fileIndex, nameTable, err := collectSourceFiles(sourceDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("archive %s is empty", sourceDir)
	}

	nameTableCompressed, err := rawDeflate(nameTable)
	if err != nil {
		return err
	}
	if err := xorFirstWord(nameTableCompressed, uint32(len(files))); err != nil {
		return err
	}

	dataSpool, err := os.CreateTemp("", "tpak-data-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = dataSpool.Close()
		_ = os.Remove(dataSpool.Name())
	}()

	fileEntries := make([]fileEntry, 0, len(files))
	chunkEntries := make([]chunkEntry, 0, len(files))
	for index := range files {
		item := &files[index]
		entry, chunk, err := spoolArchiveFile(dataSpool, item)
		if err != nil {
			return err
		}

		entry.NameOffset = item.NameOffset
		entry.ChunkIndex = int32(len(chunkEntries))
		fileEntries = append(fileEntries, entry)
		chunkEntries = append(chunkEntries, chunk)
	}

	fileTableCompressed, err := compressTable(fileEntries, uint32(len(files)))
	if err != nil {
		return err
	}

	chunkTableCompressed, err := compressTable(chunkEntries, uint32(len(files)+len(chunkEntries)))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = outputFile.Close()
	}()

	if _, err := outputFile.Write([]byte(headerSignature)); err != nil {
		return err
	}
	header := archiveHeader{
		Version:                 formatVersion,
		Unknown1:                headerUnknown1,
		FileCount:               int32(len(files)),
		Reserved:                headerReserved,
		NameTableSize:           int32(len(nameTable)),
		CompressedNameTableSize: int32(len(nameTableCompressed)),
	}
	if err := writeArchiveHeader(outputFile, &header); err != nil {
		return err
	}
	if _, err := outputFile.Write(nameTableCompressed); err != nil {
		return err
	}
	if err := writeAlignment(outputFile); err != nil {
		return err
	}

	for _, value := range fileIndex {
		if err := binary.Write(outputFile, binary.LittleEndian, value); err != nil {
			return err
		}
	}
	if err := writeAlignment(outputFile); err != nil {
		return err
	}

	if err := binary.Write(outputFile, binary.LittleEndian, int32(len(fileTableCompressed))); err != nil {
		return err
	}
	if _, err := outputFile.Write(fileTableCompressed); err != nil {
		return err
	}
	if err := writeAlignment(outputFile); err != nil {
		return err
	}

	if err := binary.Write(outputFile, binary.LittleEndian, int32(len(chunkTableCompressed))); err != nil {
		return err
	}
	if err := binary.Write(outputFile, binary.LittleEndian, int32(len(chunkEntries))); err != nil {
		return err
	}
	if _, err := outputFile.Write(chunkTableCompressed); err != nil {
		return err
	}
	if err := writeAlignment(outputFile); err != nil {
		return err
	}

	if _, err := dataSpool.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.Copy(outputFile, dataSpool); err != nil {
		return err
	}

	return nil
}

func collectSourceFiles(sourceDir string) ([]sourceFile, []int32, []byte, error) {
	archivePaths := collections.NewHashSet[string]()
	files := make([]sourceFile, 0)

	if err := walkSourceFiles(sourceDir, sourceDir, archivePaths, &files); err != nil {
		return nil, nil, nil, err
	}

	nameTable := bytes.NewBuffer(nil)
	for index := range files {
		files[index].NameOffset = int32(nameTable.Len() + 4)
		record, err := encodeNameRecord(files[index].ArchivePath, index)
		if err != nil {
			return nil, nil, nil, err
		}
		if _, err := nameTable.Write(record); err != nil {
			return nil, nil, nil, err
		}
	}

	return files, buildFileIndex(files), nameTable.Bytes(), nil
}

func walkSourceFiles(rootDir string, currentDir string, archivePaths *collections.HashSet[string], files *[]sourceFile) error {
	entries, err := readDirectoryEntries(currentDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(currentDir, entry.Name())
		if entry.IsDir() {
			if err := walkSourceFiles(rootDir, path, archivePaths, files); err != nil {
				return err
			}
			continue
		}

		relativePath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		archivePath := archivePathFromRelative(filepath.ToSlash(relativePath))
		if archivePaths.Contains(archivePath) {
			return fmt.Errorf("duplicate archive path %s", archivePath)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > int64(^uint32(0)>>1) {
			return fmt.Errorf("file %s is too large for TPAK v7", path)
		}

		archivePaths.Add(archivePath)
		*files = append(*files, sourceFile{
			ArchivePath: archivePath,
			SourcePath:  path,
			FileSize:    int32(info.Size()),
		})
	}

	return nil
}

func buildFileIndex(files []sourceFile) []int32 {
	indices := make([]int, len(files))
	for index := range files {
		indices[index] = index
	}

	sort.SliceStable(indices, func(left int, right int) bool {
		return files[indices[left]].ArchivePath < files[indices[right]].ArchivePath
	})

	fileIndex := make([]int32, len(files))
	for index, fileEntryIndex := range indices {
		fileIndex[index] = int32(fileEntryIndex)
	}
	return fileIndex
}

func encodeNameRecord(name string, entryIndex int) ([]byte, error) {
	raw := []byte(name)
	buffer := bytes.NewBuffer(make([]byte, 0, 4+len(raw)+1))
	if err := binary.Write(buffer, binary.LittleEndian, int32(len(raw))); err != nil {
		return nil, err
	}

	encoded := append([]byte(nil), raw...)
	mask := (len(encoded) % 5) + entryIndex
	for index := range encoded {
		encoded[index] ^= byte(((index + len(encoded)) * 2) + mask)
	}

	if _, err := buffer.Write(encoded); err != nil {
		return nil, err
	}
	if err := buffer.WriteByte(0); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func compressTable[T any](entries []T, xorBase uint32) ([]byte, error) {
	raw, err := binaryEntries(entries)
	if err != nil {
		return nil, err
	}
	compressed, err := rawDeflate(raw)
	if err != nil {
		return nil, err
	}
	if err := xorFirstWord(compressed, xorBase+uint32(len(compressed))); err != nil {
		return nil, err
	}
	return compressed, nil
}

func spoolArchiveFile(dataSpool *os.File, item *sourceFile) (fileEntry, chunkEntry, error) {
	chunk, err := spoolFileChunk(dataSpool, item)
	if err != nil {
		return fileEntry{}, chunkEntry{}, err
	}
	return fileEntry{
		FileSize:   item.FileSize,
		ChunkCount: 1,
	}, chunk.Entry, nil
}

func spoolFileChunk(dataSpool *os.File, item *sourceFile) (spoolChunk, error) {
	compression := compressionForArchivePath(item.ArchivePath)
	switch compression {
	case chunkCompressionDeflate:
		return spoolDeflatedFileChunk(dataSpool, item.SourcePath, false)
	case chunkCompressionAuto:
		return spoolDeflatedFileChunk(dataSpool, item.SourcePath, true)
	default:
		return spoolRawFileChunk(dataSpool, item.SourcePath)
	}
}

func spoolRawFileChunk(dataSpool *os.File, sourcePath string) (spoolChunk, error) {
	chunkOffset, err := dataSpool.Seek(0, io.SeekCurrent)
	if err != nil {
		return spoolChunk{}, err
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return spoolChunk{}, err
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	uncompressedSize, err := io.Copy(dataSpool, sourceFile)
	if err != nil {
		return spoolChunk{}, err
	}
	if uncompressedSize > int64(^uint32(0)>>1) {
		return spoolChunk{}, fmt.Errorf("file %s is too large for TPAK v7", sourcePath)
	}

	return spoolChunk{
		Entry: chunkEntry{
			FileOffset:       0,
			UncompressedSize: int32(uncompressedSize),
			DataOffset:       int32(chunkOffset),
			CompressedSize:   int32(uncompressedSize),
		},
	}, nil
}

func spoolDeflatedFileChunk(dataSpool *os.File, sourcePath string, onlyIfSmaller bool) (spoolChunk, error) {
	chunkOffset, err := dataSpool.Seek(0, io.SeekCurrent)
	if err != nil {
		return spoolChunk{}, err
	}

	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return spoolChunk{}, err
	}
	uncompressedSize := len(sourceData)
	if int64(uncompressedSize) > int64(^uint32(0)>>1) {
		return spoolChunk{}, fmt.Errorf("file %s is too large for TPAK v7", sourcePath)
	}

	compressedData, err := rawDeflate(sourceData)
	if err != nil {
		return spoolChunk{}, err
	}
	compressedSize := len(compressedData)
	if compressedSize == uncompressedSize || (onlyIfSmaller && compressedSize > uncompressedSize) {
		return spoolRawFileChunk(dataSpool, sourcePath)
	}
	if _, err := dataSpool.Write(compressedData); err != nil {
		return spoolChunk{}, err
	}

	return spoolChunk{
		Entry: chunkEntry{
			FileOffset:       0,
			UncompressedSize: int32(uncompressedSize),
			DataOffset:       int32(chunkOffset),
			CompressedSize:   int32(compressedSize),
		},
	}, nil
}

func binaryEntries[T any](entries []T) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	for index := range entries {
		if err := binary.Write(buffer, binary.LittleEndian, &entries[index]); err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

func writeAlignment(file *os.File) error {
	offset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	padding := alignmentPadding(offset)
	if len(padding) == 0 {
		return nil
	}

	_, err = file.Write(padding)
	return err
}

func writeArchiveHeader(file *os.File, header *archiveHeader) error {
	if err := binary.Write(file, binary.LittleEndian, header.Version); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, header.Unknown1); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, header.FileCount); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, header.Reserved); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, header.NameTableSize); err != nil {
		return err
	}
	return binary.Write(file, binary.LittleEndian, header.CompressedNameTableSize)
}
