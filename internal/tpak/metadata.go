package tpak

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"sc_cli/internal/collections"
)

const (
	metadataFileName       = ".tpak.meta.json"
	metadataRawDirName     = ".tpak.raw"
	metadataChunksDirName  = "chunks"
	metadataNameTableFile  = "name_table.bin"
	metadataFileTableFile  = "file_table.bin"
	metadataChunkTableFile = "chunk_table.bin"
)

type metadataIndex struct {
	filesByPath *collections.HashMap[string, *ArchiveFileInfo]
}

func newMetadataIndex(metadata *ArchiveInfo) *metadataIndex {
	filesByPath := collections.NewHashMap[string, *ArchiveFileInfo]()
	for index := range metadata.Files {
		file := &metadata.Files[index]
		filesByPath.Put(file.ArchivePath, file)
	}
	return &metadataIndex{filesByPath: filesByPath}
}

func (index *metadataIndex) Get(path string) (*ArchiveFileInfo, bool) {
	return index.filesByPath.Get(path)
}

func writeArchiveMetadata(targetDir string, reader *archiveReader) error {
	rawDir := filepath.Join(targetDir, metadataRawDirName)
	chunksDir := filepath.Join(rawDir, metadataChunksDirName)
	if err := os.MkdirAll(chunksDir, 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(rawDir, metadataNameTableFile), reader.layout.CompressedNameTable, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(rawDir, metadataFileTableFile), reader.layout.CompressedFileTable, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(rawDir, metadataChunkTableFile), reader.layout.CompressedChunkTable, 0o644); err != nil {
		return err
	}

	metadata := archiveInfoFromLayout("", reader.layout)
	metadata.Tables.NameTable = filepath.ToSlash(filepath.Join(metadataRawDirName, metadataNameTableFile))
	metadata.Tables.FileTable = filepath.ToSlash(filepath.Join(metadataRawDirName, metadataFileTableFile))
	metadata.Tables.ChunkTable = filepath.ToSlash(filepath.Join(metadataRawDirName, metadataChunkTableFile))

	for fileIndex := range reader.layout.Files {
		file := &reader.layout.Files[fileIndex]
		if file.ChunkCount < 0 {
			return fmt.Errorf("invalid negative chunk count for %s", file.ArchivePath)
		}

		targetPath := outputPath(targetDir, file.ArchivePath)
		hash, err := fileSHA1Hex(targetPath)
		if err != nil {
			return err
		}

		chunks := make([]ArchiveChunkInfo, int(file.ChunkCount))
		for chunkOffset := int32(0); chunkOffset < file.ChunkCount; chunkOffset++ {
			chunkIndex := file.ChunkIndex + chunkOffset
			if chunkIndex < 0 || int(chunkIndex) >= len(reader.layout.Chunks) {
				return fmt.Errorf("chunk index %d out of range for %s", chunkIndex, file.ArchivePath)
			}
			chunk := &reader.layout.Chunks[chunkIndex]

			payloadRelativePath := filepath.ToSlash(filepath.Join(metadataChunksDirName, fmt.Sprintf("%06d.bin", chunkIndex)))
			payloadFile, err := os.Create(filepath.Join(rawDir, filepath.FromSlash(payloadRelativePath)))
			if err != nil {
				return err
			}
			writeErr := reader.writeRawChunk(payloadFile, chunk)
			closeErr := payloadFile.Close()
			if err := errors.Join(writeErr, closeErr); err != nil {
				return err
			}

			chunks[int(chunkOffset)] = ArchiveChunkInfo{
				Index:            int(chunkIndex),
				FileOffset:       chunk.FileOffset,
				UncompressedSize: chunk.UncompressedSize,
				DataOffset:       chunk.DataOffset,
				CompressedSize:   chunk.CompressedSize,
				Payload:          payloadRelativePath,
			}
		}

		metadata.Files[fileIndex].SHA1 = hash
		metadata.Files[fileIndex].Chunks = chunks
	}

	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(targetDir, metadataFileName), encoded, 0o644)
}

func readArchiveMetadata(sourceDir string) (ArchiveInfo, bool, error) {
	path := filepath.Join(sourceDir, metadataFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ArchiveInfo{}, false, nil
		}
		return ArchiveInfo{}, false, err
	}

	metadata := ArchiveInfo{}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return ArchiveInfo{}, false, err
	}
	if err := validateArchiveMetadata(&metadata); err != nil {
		return ArchiveInfo{}, false, err
	}

	return metadata, true, nil
}

func validateArchiveMetadata(metadata *ArchiveInfo) error {
	if metadata.Header.Signature != headerSignature {
		return fmt.Errorf("invalid metadata signature %q", metadata.Header.Signature)
	}
	if metadata.Header.Version != formatVersion {
		return fmt.Errorf("unsupported metadata TPAK version %d", metadata.Header.Version)
	}
	if metadata.Header.FileCount != int32(len(metadata.Files)) {
		return fmt.Errorf("metadata file count mismatch: header=%d files=%d", metadata.Header.FileCount, len(metadata.Files))
	}
	if metadata.Tables.NameRecordCount != len(metadata.NameRecords) {
		return fmt.Errorf("metadata name record count mismatch: table=%d records=%d", metadata.Tables.NameRecordCount, len(metadata.NameRecords))
	}
	if metadata.Tables.FileIndexCount != len(metadata.FileIndex) {
		return fmt.Errorf("metadata file index count mismatch: table=%d index=%d", metadata.Tables.FileIndexCount, len(metadata.FileIndex))
	}
	if metadata.Tables.FileEntryCount != len(metadata.FileEntries) {
		return fmt.Errorf("metadata file entry count mismatch: table=%d entries=%d", metadata.Tables.FileEntryCount, len(metadata.FileEntries))
	}
	if metadata.Tables.ChunkCount != len(metadata.Chunks) {
		return fmt.Errorf("metadata chunk count mismatch: table=%d chunks=%d", metadata.Tables.ChunkCount, len(metadata.Chunks))
	}
	if metadata.Tables.NameTable == "" || metadata.Tables.FileTable == "" || metadata.Tables.ChunkTable == "" {
		return fmt.Errorf("metadata raw table paths are required")
	}
	if metadata.Tables.CompressedFileTableSize <= 0 || metadata.Tables.CompressedChunkTableSize <= 0 || metadata.Tables.HeaderEnd <= 0 {
		return fmt.Errorf("metadata table sizes and header end are required")
	}
	for index := range metadata.Files {
		file := &metadata.Files[index]
		if file.Index != index {
			return fmt.Errorf("metadata file index mismatch for %s: index=%d position=%d", file.ArchivePath, file.Index, index)
		}
		if file.ArchivePath == "" {
			return fmt.Errorf("metadata file at index %d is missing archive path", index)
		}
		if file.SHA1 == "" {
			return fmt.Errorf("metadata file %s is missing sha1", file.ArchivePath)
		}
		if int(file.ChunkCount) != len(file.Chunks) {
			return fmt.Errorf("metadata chunk count mismatch for %s", file.ArchivePath)
		}
		for chunkOffset := range file.Chunks {
			if file.Chunks[chunkOffset].Payload == "" {
				return fmt.Errorf("metadata file %s chunk %d is missing payload", file.ArchivePath, chunkOffset)
			}
		}
	}
	return nil
}

func orderSourceFiles(files []sourceFile, metadata *ArchiveInfo, hasMetadata bool) ([]sourceFile, []int32, error) {
	if !hasMetadata {
		sort.Slice(files, func(left int, right int) bool {
			return files[left].ArchivePath < files[right].ArchivePath
		})

		fileIndex := make([]int32, len(files))
		for index := range files {
			fileIndex[index] = int32(index)
		}
		return files, fileIndex, nil
	}

	filesByPath := collections.NewHashMap[string, sourceFile]()
	for _, file := range files {
		filesByPath.Put(file.ArchivePath, file)
	}

	ordered := make([]sourceFile, 0, len(files))
	for _, metadataFile := range metadata.Files {
		file, ok := filesByPath.Get(metadataFile.ArchivePath)
		if !ok {
			return nil, nil, fmt.Errorf("metadata file order references missing archive path %s", metadataFile.ArchivePath)
		}
		ordered = append(ordered, file)
		filesByPath.Delete(metadataFile.ArchivePath)
	}

	if filesByPath.Len() > 0 {
		extraPaths := filesByPath.Keys()
		sort.Strings(extraPaths)
		for _, archivePath := range extraPaths {
			file, _ := filesByPath.Get(archivePath)
			ordered = append(ordered, file)
		}
	}

	fileIndex := make([]int32, len(ordered))
	if len(metadata.FileIndex) == len(ordered) {
		copy(fileIndex, metadata.FileIndex)
		return ordered, fileIndex, nil
	}

	for index := range ordered {
		fileIndex[index] = int32(index)
	}
	return ordered, fileIndex, nil
}

func metadataRelativePath(baseDir string, relativePath string) string {
	return filepath.Join(baseDir, filepath.FromSlash(relativePath))
}

func metadataChunkPayloadPath(baseDir string, relativePath string) string {
	return filepath.Join(baseDir, metadataRawDirName, filepath.FromSlash(relativePath))
}

func fileSHA1Hex(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
