package tpak

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"sc_cli/internal/collections"
)

func packArchiveDirectory(sourceDir string, outputPath string) error {
	metadata, hasMetadata, err := readArchiveMetadata(sourceDir)
	if err != nil {
		return err
	}

	files, fileIndex, nameTable, err := collectSourceFiles(sourceDir, &metadata, hasMetadata)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("archive %s is empty", sourceDir)
	}

	canReplayOriginal, err := canReplayOriginalArchive(files, &metadata, hasMetadata)
	if err != nil {
		return err
	}
	if canReplayOriginal {
		return replayOriginalArchive(sourceDir, outputPath, nameTable, &metadata)
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

	metadataLookup := newMetadataIndex(&metadata)
	fileEntries := make([]fileEntry, 0, len(files))
	chunkEntries := make([]chunkEntry, 0, len(files))
	for index := range files {
		item := &files[index]
		entry, chunks, err := spoolArchiveFile(dataSpool, sourceDir, item, metadataLookup, hasMetadata)
		if err != nil {
			return err
		}

		entry.NameOffset = item.NameOffset
		entry.ChunkIndex = int32(len(chunkEntries))
		fileEntries = append(fileEntries, entry)
		chunkEntries = append(chunkEntries, chunks...)
	}

	fileTableRaw, err := binaryEntries(fileEntries)
	if err != nil {
		return err
	}
	fileTableCompressed, err := rawDeflate(fileTableRaw)
	if err != nil {
		return err
	}
	if err := xorFirstWord(fileTableCompressed, uint32(len(files)+len(fileTableCompressed))); err != nil {
		return err
	}

	chunkTableRaw, err := binaryEntries(chunkEntries)
	if err != nil {
		return err
	}
	chunkTableCompressed, err := rawDeflate(chunkTableRaw)
	if err != nil {
		return err
	}
	if err := xorFirstWord(chunkTableCompressed, uint32(len(files)+len(chunkTableCompressed)+len(chunkEntries))); err != nil {
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
		Version:                 metadata.Header.Version,
		Unknown1:                metadata.Header.Unknown1,
		FileCount:               int32(len(files)),
		Reserved:                metadata.Header.Reserved,
		NameTableSize:           int32(len(nameTable)),
		CompressedNameTableSize: int32(len(nameTableCompressed)),
	}
	if !hasMetadata {
		header.Version = formatVersion
		header.Unknown1 = headerUnknown1
		header.Reserved = headerReserved
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

func collectSourceFiles(sourceDir string, metadata *ArchiveInfo, hasMetadata bool) ([]sourceFile, []int32, []byte, error) {
	archivePaths := collections.NewOrderedSet[string]()
	files := make([]sourceFile, 0)

	if err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			relativePath, err := filepath.Rel(sourceDir, path)
			if err != nil {
				return err
			}
			if filepath.ToSlash(relativePath) == metadataRawDirName {
				return filepath.SkipDir
			}
			return nil
		}

		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)
		if relativePath == metadataFileName {
			return nil
		}
		archivePath := archivePathFromRelative(relativePath)
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
		files = append(files, sourceFile{
			ArchivePath: archivePath,
			SourcePath:  path,
			FileSize:    int32(info.Size()),
		})
		return nil
	}); err != nil {
		return nil, nil, nil, err
	}

	files, fileIndex, err := orderSourceFiles(files, metadata, hasMetadata)
	if err != nil {
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

	return files, fileIndex, nameTable.Bytes(), nil
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

func spoolFileChunk(dataSpool *os.File, sourcePath string) (spoolChunk, error) {
	chunkOffset, err := dataSpool.Seek(0, io.SeekCurrent)
	if err != nil {
		return spoolChunk{}, err
	}

	compressedSpool, err := os.CreateTemp("", "tpak-compressed-*")
	if err != nil {
		return spoolChunk{}, err
	}
	compressedPath := compressedSpool.Name()
	defer func() {
		_ = compressedSpool.Close()
		_ = os.Remove(compressedPath)
	}()

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return spoolChunk{}, err
	}

	compressor, err := flate.NewWriter(compressedSpool, flate.BestCompression)
	if err != nil {
		_ = sourceFile.Close()
		return spoolChunk{}, err
	}

	uncompressedSize, copyErr := io.Copy(compressor, sourceFile)
	closeErr := compressor.Close()
	_ = sourceFile.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return spoolChunk{}, err
	}
	if uncompressedSize > int64(^uint32(0)>>1) {
		return spoolChunk{}, fmt.Errorf("file %s is too large for TPAK v7", sourcePath)
	}

	compressedSize, err := compressedSpool.Seek(0, io.SeekCurrent)
	if err != nil {
		return spoolChunk{}, err
	}

	if _, err := compressedSpool.Seek(0, io.SeekStart); err != nil {
		return spoolChunk{}, err
	}

	storedSize := compressedSize
	if compressedSize < uncompressedSize {
		if _, err := io.Copy(dataSpool, compressedSpool); err != nil {
			return spoolChunk{}, err
		}
	} else {
		rawSource, err := os.Open(sourcePath)
		if err != nil {
			return spoolChunk{}, err
		}
		if _, err := io.Copy(dataSpool, rawSource); err != nil {
			_ = rawSource.Close()
			return spoolChunk{}, err
		}
		_ = rawSource.Close()
		storedSize = uncompressedSize
	}

	return spoolChunk{
		Entry: chunkEntry{
			FileOffset:       0,
			UncompressedSize: int32(uncompressedSize),
			DataOffset:       int32(chunkOffset),
			CompressedSize:   int32(storedSize),
		},
	}, nil
}

func canReplayOriginalArchive(files []sourceFile, metadata *ArchiveInfo, hasMetadata bool) (bool, error) {
	if !hasMetadata || len(files) == 0 || len(files) != len(metadata.Files) {
		return false, nil
	}
	if len(metadata.FileIndex) != len(files) {
		return false, nil
	}
	if metadata.Tables.NameTable == "" || metadata.Tables.FileTable == "" || metadata.Tables.ChunkTable == "" {
		return false, nil
	}

	for index := range files {
		item := &files[index]
		metadataFile := &metadata.Files[index]
		if item.ArchivePath != metadataFile.ArchivePath {
			return false, nil
		}

		hash, err := fileSHA1Hex(item.SourcePath)
		if err != nil {
			return false, err
		}
		if hash != metadataFile.SHA1 {
			return false, nil
		}
	}

	return true, nil
}

func replayOriginalArchive(sourceDir string, outputPath string, nameTable []byte, metadata *ArchiveInfo) error {
	nameTableCompressed, err := os.ReadFile(metadataRelativePath(sourceDir, metadata.Tables.NameTable))
	if err != nil {
		return err
	}
	fileTableCompressed, err := os.ReadFile(metadataRelativePath(sourceDir, metadata.Tables.FileTable))
	if err != nil {
		return err
	}
	chunkTableCompressed, err := os.ReadFile(metadataRelativePath(sourceDir, metadata.Tables.ChunkTable))
	if err != nil {
		return err
	}

	chunkPayloads, chunkCount, err := orderedChunkPayloads(sourceDir, metadata)
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
		Version:                 metadata.Header.Version,
		Unknown1:                metadata.Header.Unknown1,
		FileCount:               int32(len(metadata.Files)),
		Reserved:                metadata.Header.Reserved,
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
	for _, value := range metadata.FileIndex {
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
	if err := binary.Write(outputFile, binary.LittleEndian, int32(chunkCount)); err != nil {
		return err
	}
	if _, err := outputFile.Write(chunkTableCompressed); err != nil {
		return err
	}
	if err := writeAlignment(outputFile); err != nil {
		return err
	}

	for index := range chunkPayloads {
		payloadPath := chunkPayloads[index]
		if err := appendFileContents(outputFile, payloadPath); err != nil {
			return err
		}
	}

	return nil
}

func orderedChunkPayloads(sourceDir string, metadata *ArchiveInfo) ([]string, int, error) {
	chunkCount := 0
	for index := range metadata.Files {
		file := &metadata.Files[index]
		chunkCount += len(file.Chunks)
	}

	chunkPayloads := make([]string, chunkCount)
	for fileIndex := range metadata.Files {
		file := &metadata.Files[fileIndex]
		if int(file.ChunkCount) != len(file.Chunks) {
			return nil, 0, fmt.Errorf("metadata chunk count mismatch for %s", file.ArchivePath)
		}

		for chunkOffset := range file.Chunks {
			chunk := &file.Chunks[chunkOffset]
			chunkIndex := int(file.ChunkIndex) + chunkOffset
			if chunkIndex < 0 || chunkIndex >= len(chunkPayloads) {
				return nil, 0, fmt.Errorf("metadata chunk index %d out of range for %s", chunkIndex, file.ArchivePath)
			}
			chunkPayloads[chunkIndex] = metadataChunkPayloadPath(sourceDir, chunk.Payload)
		}
	}

	for index, payloadPath := range chunkPayloads {
		if payloadPath == "" {
			return nil, 0, fmt.Errorf("missing payload path for chunk %d", index)
		}
	}

	return chunkPayloads, chunkCount, nil
}

func spoolArchiveFile(dataSpool *os.File, sourceDir string, item *sourceFile, metadata *metadataIndex, hasMetadata bool) (fileEntry, []chunkEntry, error) {
	if hasMetadata {
		metadataFile, ok := metadata.Get(item.ArchivePath)
		if ok {
			hash, err := fileSHA1Hex(item.SourcePath)
			if err != nil {
				return fileEntry{}, nil, err
			}
			if hash == metadataFile.SHA1 {
				chunks, err := spoolOriginalChunks(dataSpool, sourceDir, metadataFile)
				if err != nil {
					return fileEntry{}, nil, err
				}
				return fileEntry{
					FileSize:   item.FileSize,
					ChunkCount: int32(len(chunks)),
				}, chunks, nil
			}
		}
	}

	chunk, err := spoolFileChunk(dataSpool, item.SourcePath)
	if err != nil {
		return fileEntry{}, nil, err
	}
	return fileEntry{
		FileSize:   item.FileSize,
		ChunkCount: 1,
	}, []chunkEntry{chunk.Entry}, nil
}

func spoolOriginalChunks(dataSpool *os.File, sourceDir string, metadataFile *ArchiveFileInfo) ([]chunkEntry, error) {
	if int(metadataFile.ChunkCount) != len(metadataFile.Chunks) {
		return nil, fmt.Errorf("metadata chunk count mismatch for %s", metadataFile.ArchivePath)
	}

	chunks := make([]chunkEntry, 0, len(metadataFile.Chunks))
	for index := range metadataFile.Chunks {
		metadataChunk := &metadataFile.Chunks[index]
		chunkOffset, err := dataSpool.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		if err := appendFileContents(dataSpool, metadataChunkPayloadPath(sourceDir, metadataChunk.Payload)); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunkEntry{
			FileOffset:       metadataChunk.FileOffset,
			UncompressedSize: metadataChunk.UncompressedSize,
			DataOffset:       int32(chunkOffset),
			CompressedSize:   metadataChunk.CompressedSize,
		})
	}

	return chunks, nil
}

func appendFileContents(destination *os.File, sourcePath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	_, err = io.Copy(destination, sourceFile)
	return err
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
