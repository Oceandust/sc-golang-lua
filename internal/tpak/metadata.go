package tpak

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"defgraph/internal/collections"
)

const (
	metadataFileName       = ".tpak.meta.json"
	metadataRawDirName     = ".tpak.raw"
	metadataChunksDirName  = "chunks"
	metadataNameTableFile  = "name_table.bin"
	metadataFileTableFile  = "file_table.bin"
	metadataChunkTableFile = "chunk_table.bin"
)

type archiveMetadata struct {
	Header    archiveMetadataHeader `json:"header"`
	FileIndex []int32               `json:"file_index"`
	Files     []archiveMetadataFile `json:"files"`
	Tables    archiveMetadataTables `json:"tables"`
}

type archiveMetadataHeader struct {
	Version  int32 `json:"version"`
	Unknown1 int32 `json:"unknown1"`
	Reserved int32 `json:"reserved"`
}

type archiveMetadataTables struct {
	NameTable  string `json:"name_table"`
	FileTable  string `json:"file_table"`
	ChunkTable string `json:"chunk_table"`
}

type archiveMetadataFile struct {
	ArchivePath string                 `json:"archive_path"`
	NameOffset  int32                  `json:"name_offset"`
	FileSize    int32                  `json:"file_size"`
	ChunkCount  int32                  `json:"chunk_count"`
	ChunkIndex  int32                  `json:"chunk_index"`
	SHA1        string                 `json:"sha1"`
	Chunks      []archiveMetadataChunk `json:"chunks"`
}

type archiveMetadataChunk struct {
	FileOffset       int32  `json:"file_offset"`
	UncompressedSize int32  `json:"uncompressed_size"`
	CompressedSize   int32  `json:"compressed_size"`
	Payload          string `json:"payload"`
}

type metadataIndex struct {
	filesByPath *collections.HashMap[string, archiveMetadataFile]
}

func newMetadataIndex(metadata archiveMetadata) metadataIndex {
	filesByPath := collections.NewHashMap[string, archiveMetadataFile]()
	for _, file := range metadata.Files {
		filesByPath.Put(file.ArchivePath, file)
	}
	return metadataIndex{filesByPath: filesByPath}
}

func (index metadataIndex) Get(path string) (archiveMetadataFile, bool) {
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

	files := make([]archiveMetadataFile, 0, len(reader.layout.Files))
	for _, file := range reader.layout.Files {
		targetPath := outputPath(targetDir, file.ArchivePath)
		hash, err := fileSHA1Hex(targetPath)
		if err != nil {
			return err
		}

		chunks := make([]archiveMetadataChunk, 0, file.ChunkCount)
		for chunkOffset := int32(0); chunkOffset < file.ChunkCount; chunkOffset++ {
			chunkIndex := file.ChunkIndex + chunkOffset
			chunk := reader.layout.Chunks[chunkIndex]
			payload, err := reader.readRawChunk(chunk)
			if err != nil {
				return err
			}

			payloadRelativePath := filepath.ToSlash(filepath.Join(metadataChunksDirName, fmt.Sprintf("%06d.bin", chunkIndex)))
			if err := os.WriteFile(filepath.Join(rawDir, filepath.FromSlash(payloadRelativePath)), payload, 0o644); err != nil {
				return err
			}

			chunks = append(chunks, archiveMetadataChunk{
				FileOffset:       chunk.FileOffset,
				UncompressedSize: chunk.UncompressedSize,
				CompressedSize:   chunk.CompressedSize,
				Payload:          payloadRelativePath,
			})
		}

		files = append(files, archiveMetadataFile{
			ArchivePath: file.ArchivePath,
			NameOffset:  file.NameOffset,
			FileSize:    file.FileSize,
			ChunkCount:  file.ChunkCount,
			ChunkIndex:  file.ChunkIndex,
			SHA1:        hash,
			Chunks:      chunks,
		})
	}

	fileIndex := make([]int32, len(reader.layout.FileIndex))
	copy(fileIndex, reader.layout.FileIndex)

	metadata := archiveMetadata{
		Header: archiveMetadataHeader{
			Version:  reader.layout.Header.Version,
			Unknown1: reader.layout.Header.Unknown1,
			Reserved: reader.layout.Header.Reserved,
		},
		FileIndex: fileIndex,
		Files:     files,
		Tables: archiveMetadataTables{
			NameTable:  filepath.ToSlash(filepath.Join(metadataRawDirName, metadataNameTableFile)),
			FileTable:  filepath.ToSlash(filepath.Join(metadataRawDirName, metadataFileTableFile)),
			ChunkTable: filepath.ToSlash(filepath.Join(metadataRawDirName, metadataChunkTableFile)),
		},
	}

	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(targetDir, metadataFileName), encoded, 0o644)
}

func readArchiveMetadata(sourceDir string) (archiveMetadata, bool, error) {
	path := filepath.Join(sourceDir, metadataFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return archiveMetadata{}, false, nil
		}
		return archiveMetadata{}, false, err
	}

	metadata := archiveMetadata{}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return archiveMetadata{}, false, err
	}
	if metadata.Header.Version == 0 {
		metadata.Header.Version = formatVersion
	}
	if metadata.Header.Reserved == 0 {
		metadata.Header.Reserved = headerReserved
	}

	return metadata, true, nil
}

func orderSourceFiles(files []sourceFile, metadata archiveMetadata, hasMetadata bool) ([]sourceFile, []int32, error) {
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
