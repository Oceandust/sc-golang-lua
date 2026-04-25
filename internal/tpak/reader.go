package tpak

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sync/errgroup"
)

type archiveReader struct {
	file   *os.File
	layout *archiveLayout
}

func openArchive(path string) (*archiveReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	layout, err := readArchiveLayout(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	return &archiveReader{
		file:   file,
		layout: layout,
	}, nil
}

func (reader *archiveReader) Close() error {
	if reader == nil || reader.file == nil {
		return nil
	}
	file := reader.file
	reader.file = nil
	return file.Close()
}

func readArchiveLayout(file *os.File) (*archiveLayout, error) {
	var signature [4]byte
	if _, err := io.ReadFull(file, signature[:]); err != nil {
		return nil, err
	}
	if string(signature[:]) != headerSignature {
		return nil, fmt.Errorf("unsupported signature %q", string(signature[:]))
	}

	header, err := readArchiveHeader(file)
	if err != nil {
		return nil, err
	}
	if header.Version != formatVersion {
		return nil, fmt.Errorf("unsupported TPAK version %d", header.Version)
	}
	if header.FileCount < 0 || header.NameTableSize < 0 || header.CompressedNameTableSize < 0 {
		return nil, fmt.Errorf("invalid negative header values")
	}

	fileCount := int(header.FileCount)
	compressedNames := make([]byte, int(header.CompressedNameTableSize))
	if _, err := io.ReadFull(file, compressedNames); err != nil {
		return nil, err
	}
	if err := xorFirstWord(compressedNames, uint32(header.FileCount)); err != nil {
		return nil, err
	}

	nameTable, err := rawInflate(compressedNames, int(header.NameTableSize))
	if err != nil {
		return nil, fmt.Errorf("inflate name table: %w", err)
	}
	nameRecords, nameIndex, err := decodeNameTable(nameTable, fileCount)
	if err != nil {
		return nil, err
	}

	if _, err := file.Seek(alignOffset(mustCurrentOffset(file)), io.SeekStart); err != nil {
		return nil, err
	}

	fileIndexRaw := make([]byte, fileCount*4)
	if _, err := io.ReadFull(file, fileIndexRaw); err != nil {
		return nil, err
	}
	fileIndex, err := parseInt32Table(fileIndexRaw, fileCount)
	if err != nil {
		return nil, err
	}

	if _, err := file.Seek(alignOffset(mustCurrentOffset(file)), io.SeekStart); err != nil {
		return nil, err
	}

	compressedFileTableSize, err := readInt32(file)
	if err != nil {
		return nil, err
	}
	if compressedFileTableSize < 0 {
		return nil, fmt.Errorf("invalid negative file table size %d", compressedFileTableSize)
	}
	compressedFileTable := make([]byte, int(compressedFileTableSize))
	if _, err := io.ReadFull(file, compressedFileTable); err != nil {
		return nil, err
	}
	if err := xorFirstWord(compressedFileTable, uint32(header.FileCount+compressedFileTableSize)); err != nil {
		return nil, err
	}

	fileTableRaw, err := rawInflate(compressedFileTable, fileCount*16)
	if err != nil {
		return nil, fmt.Errorf("inflate file table: %w", err)
	}
	fileEntries, err := parseFileEntries(fileTableRaw, fileCount)
	if err != nil {
		return nil, err
	}

	if _, err := file.Seek(alignOffset(mustCurrentOffset(file)), io.SeekStart); err != nil {
		return nil, err
	}

	compressedChunkTableSize, err := readInt32(file)
	if err != nil {
		return nil, err
	}
	chunkCount, err := readInt32(file)
	if err != nil {
		return nil, err
	}
	if compressedChunkTableSize < 0 || chunkCount < 0 {
		return nil, fmt.Errorf("invalid chunk table header")
	}
	compressedChunkTable := make([]byte, int(compressedChunkTableSize))
	if _, err := io.ReadFull(file, compressedChunkTable); err != nil {
		return nil, err
	}
	if err := xorFirstWord(compressedChunkTable, uint32(header.FileCount+compressedChunkTableSize+chunkCount)); err != nil {
		return nil, err
	}

	chunkTableRaw, err := rawInflate(compressedChunkTable, int(chunkCount)*16)
	if err != nil {
		return nil, fmt.Errorf("inflate chunk table: %w", err)
	}
	chunks, err := parseChunkEntries(chunkTableRaw, int(chunkCount))
	if err != nil {
		return nil, err
	}

	headerEnd := alignOffset(mustCurrentOffset(file))
	if _, err := file.Seek(headerEnd, io.SeekStart); err != nil {
		return nil, err
	}

	files := make([]archiveFile, len(fileEntries))
	for index := range fileEntries {
		entry := &fileEntries[index]
		name, ok := nameIndex.Get(entry.NameOffset)
		if !ok {
			return nil, fmt.Errorf("missing filename for name offset %d", entry.NameOffset)
		}
		files[index] = archiveFile{
			ArchivePath: name,
			NameOffset:  entry.NameOffset,
			FileSize:    entry.FileSize,
			ChunkCount:  entry.ChunkCount,
			ChunkIndex:  entry.ChunkIndex,
		}
	}

	return &archiveLayout{
		Header:                   header,
		NameRecords:              nameRecords,
		FileIndex:                fileIndex,
		FileEntries:              fileEntries,
		Files:                    files,
		Chunks:                   chunks,
		CompressedFileTableSize:  compressedFileTableSize,
		CompressedChunkTableSize: compressedChunkTableSize,
		HeaderEnd:                headerEnd,
	}, nil
}

func readArchiveHeader(file *os.File) (archiveHeader, error) {
	var raw [24]byte
	if _, err := io.ReadFull(file, raw[:]); err != nil {
		return archiveHeader{}, err
	}

	return archiveHeader{
		Version:                 int32(binary.LittleEndian.Uint32(raw[0:4])),
		Unknown1:                int32(binary.LittleEndian.Uint32(raw[4:8])),
		FileCount:               int32(binary.LittleEndian.Uint32(raw[8:12])),
		Reserved:                int32(binary.LittleEndian.Uint32(raw[12:16])),
		NameTableSize:           int32(binary.LittleEndian.Uint32(raw[16:20])),
		CompressedNameTableSize: int32(binary.LittleEndian.Uint32(raw[20:24])),
	}, nil
}

func readInt32(file *os.File) (int32, error) {
	var raw [4]byte
	if _, err := io.ReadFull(file, raw[:]); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(raw[:])), nil
}

func decodeNameTable(raw []byte, fileCount int) ([]nameRecord, *nameOffsetIndex, error) {
	records := make([]nameRecord, fileCount)
	index := newNameOffsetIndex()
	offset := 0
	for entryIndex := 0; entryIndex < fileCount; entryIndex++ {
		if offset+4 > len(raw) {
			return nil, index, fmt.Errorf("name table truncated at entry %d", entryIndex)
		}
		length := int(binary.LittleEndian.Uint32(raw[offset : offset+4]))
		offset += 4
		if length < 0 || offset+length+1 > len(raw) {
			return nil, index, fmt.Errorf("invalid name length %d at entry %d", length, entryIndex)
		}

		encoded := raw[offset : offset+length]
		mask := (length % 5) + entryIndex
		for indexValue := range encoded {
			encoded[indexValue] ^= byte(((indexValue + length) * 2) + mask)
		}

		stringOffset := int32(offset)
		name := string(encoded)
		index.Put(stringOffset, name)
		records[entryIndex] = nameRecord{
			Offset: stringOffset,
			Name:   name,
		}
		offset += length + 1
	}

	return records, index, nil
}

func parseFileEntries(raw []byte, count int) ([]fileEntry, error) {
	if len(raw) != count*16 {
		return nil, fmt.Errorf("invalid file table length %d", len(raw))
	}

	entries := make([]fileEntry, count)
	for item := range entries {
		offset := item * 16
		entries[item] = fileEntry{
			FileSize:   int32(binary.LittleEndian.Uint32(raw[offset : offset+4])),
			NameOffset: int32(binary.LittleEndian.Uint32(raw[offset+4 : offset+8])),
			ChunkCount: int32(binary.LittleEndian.Uint32(raw[offset+8 : offset+12])),
			ChunkIndex: int32(binary.LittleEndian.Uint32(raw[offset+12 : offset+16])),
		}
	}

	return entries, nil
}

func parseChunkEntries(raw []byte, count int) ([]chunkEntry, error) {
	if len(raw) != count*16 {
		return nil, fmt.Errorf("invalid chunk table length %d", len(raw))
	}

	entries := make([]chunkEntry, count)
	for item := range entries {
		offset := item * 16
		entries[item] = chunkEntry{
			FileOffset:       int32(binary.LittleEndian.Uint32(raw[offset : offset+4])),
			UncompressedSize: int32(binary.LittleEndian.Uint32(raw[offset+4 : offset+8])),
			DataOffset:       int32(binary.LittleEndian.Uint32(raw[offset+8 : offset+12])),
			CompressedSize:   int32(binary.LittleEndian.Uint32(raw[offset+12 : offset+16])),
		}
	}

	return entries, nil
}

func parseInt32Table(raw []byte, count int) ([]int32, error) {
	if len(raw) != count*4 {
		return nil, fmt.Errorf("invalid int32 table length %d", len(raw))
	}

	values := make([]int32, count)
	for item := range values {
		offset := item * 4
		values[item] = int32(binary.LittleEndian.Uint32(raw[offset : offset+4]))
	}

	return values, nil
}

func (reader *archiveReader) ExtractAll(outputDir string, threadCount int) error {
	layout := reader.layout
	group := errgroup.Group{}
	group.SetLimit(threadCount)

	for index := range layout.Files {
		fileEntry := &layout.Files[index]
		group.Go(func() error {
			targetPath := outputPath(outputDir, fileEntry.ArchivePath)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}

			targetFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			writeErr := reader.writeFile(targetFile, fileEntry)
			closeErr := targetFile.Close()
			return errors.Join(writeErr, closeErr)
		})
	}

	return group.Wait()
}

func (reader *archiveReader) writeFile(destination io.Writer, fileEntry *archiveFile) error {
	if fileEntry.FileSize < 0 {
		return fmt.Errorf("invalid negative file size for %s", fileEntry.ArchivePath)
	}
	if fileEntry.ChunkCount < 0 {
		return fmt.Errorf("invalid negative chunk count for %s", fileEntry.ArchivePath)
	}

	for chunkOffset := int32(0); chunkOffset < fileEntry.ChunkCount; chunkOffset++ {
		chunkIndex := fileEntry.ChunkIndex + chunkOffset
		if chunkIndex < 0 || int(chunkIndex) >= len(reader.layout.Chunks) {
			return fmt.Errorf("chunk index %d out of range for %s", chunkIndex, fileEntry.ArchivePath)
		}

		chunk := &reader.layout.Chunks[chunkIndex]
		if err := reader.writeChunk(destination, chunk); err != nil {
			return fmt.Errorf("write chunk %d for %s: %w", chunkIndex, fileEntry.ArchivePath, err)
		}
	}

	return nil
}

func (reader *archiveReader) writeChunk(destination io.Writer, chunk *chunkEntry) error {
	section, err := reader.chunkSection(chunk)
	if err != nil {
		return err
	}

	if chunk.CompressedSize == chunk.UncompressedSize {
		_, err := io.CopyN(destination, section, int64(chunk.CompressedSize))
		return err
	}

	return rawInflateToWriter(section, destination)
}

func (reader *archiveReader) chunkSection(chunk *chunkEntry) (*io.SectionReader, error) {
	if chunk.CompressedSize < 0 || chunk.UncompressedSize < 0 {
		return nil, fmt.Errorf("invalid negative chunk size")
	}
	if chunk.DataOffset < 0 {
		return nil, fmt.Errorf("invalid negative chunk data offset %d", chunk.DataOffset)
	}

	offset := reader.layout.HeaderEnd + int64(chunk.DataOffset)
	return io.NewSectionReader(reader.file, offset, int64(chunk.CompressedSize)), nil
}

func mustCurrentOffset(file *os.File) int64 {
	offset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		panic(err)
	}
	return offset
}
