package tpak

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type archiveReader struct {
	file   *os.File
	layout archiveLayout
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
	return reader.file.Close()
}

func readArchiveLayout(file *os.File) (archiveLayout, error) {
	var signature [4]byte
	if _, err := io.ReadFull(file, signature[:]); err != nil {
		return archiveLayout{}, err
	}
	if string(signature[:]) != headerSignature {
		return archiveLayout{}, fmt.Errorf("unsupported signature %q", string(signature[:]))
	}

	header := archiveHeader{}
	if err := binary.Read(file, binary.LittleEndian, &header.Version); err != nil {
		return archiveLayout{}, err
	}
	if err := binary.Read(file, binary.LittleEndian, &header.Unknown1); err != nil {
		return archiveLayout{}, err
	}
	if err := binary.Read(file, binary.LittleEndian, &header.FileCount); err != nil {
		return archiveLayout{}, err
	}
	if err := binary.Read(file, binary.LittleEndian, &header.Reserved); err != nil {
		return archiveLayout{}, err
	}
	if err := binary.Read(file, binary.LittleEndian, &header.NameTableSize); err != nil {
		return archiveLayout{}, err
	}
	if err := binary.Read(file, binary.LittleEndian, &header.CompressedNameTableSize); err != nil {
		return archiveLayout{}, err
	}
	if header.Version != formatVersion {
		return archiveLayout{}, fmt.Errorf("unsupported TPAK version %d", header.Version)
	}
	if header.FileCount < 0 || header.NameTableSize < 0 || header.CompressedNameTableSize < 0 {
		return archiveLayout{}, fmt.Errorf("invalid negative header values")
	}

	compressedNames := make([]byte, header.CompressedNameTableSize)
	if _, err := io.ReadFull(file, compressedNames); err != nil {
		return archiveLayout{}, err
	}
	rawCompressedNames := append([]byte(nil), compressedNames...)
	if err := xorFirstWord(compressedNames, uint32(header.FileCount)); err != nil {
		return archiveLayout{}, err
	}

	nameTable, err := rawInflate(compressedNames, int(header.NameTableSize))
	if err != nil {
		return archiveLayout{}, fmt.Errorf("inflate name table: %w", err)
	}
	nameRecords, nameIndex, err := decodeNameTable(nameTable, int(header.FileCount))
	if err != nil {
		return archiveLayout{}, err
	}

	if _, err := file.Seek(alignOffset(mustCurrentOffset(file)), io.SeekStart); err != nil {
		return archiveLayout{}, err
	}

	fileIndexRaw := make([]byte, int(header.FileCount)*4)
	if _, err := io.ReadFull(file, fileIndexRaw); err != nil {
		return archiveLayout{}, err
	}
	fileIndex, err := parseInt32Table(fileIndexRaw, int(header.FileCount))
	if err != nil {
		return archiveLayout{}, err
	}

	if _, err := file.Seek(alignOffset(mustCurrentOffset(file)), io.SeekStart); err != nil {
		return archiveLayout{}, err
	}

	var compressedFileTableSize int32
	if err := binary.Read(file, binary.LittleEndian, &compressedFileTableSize); err != nil {
		return archiveLayout{}, err
	}
	if compressedFileTableSize < 0 {
		return archiveLayout{}, fmt.Errorf("invalid negative file table size %d", compressedFileTableSize)
	}
	compressedFileTable := make([]byte, compressedFileTableSize)
	if _, err := io.ReadFull(file, compressedFileTable); err != nil {
		return archiveLayout{}, err
	}
	rawCompressedFileTable := append([]byte(nil), compressedFileTable...)
	if err := xorFirstWord(compressedFileTable, uint32(header.FileCount+compressedFileTableSize)); err != nil {
		return archiveLayout{}, err
	}

	fileTableRaw, err := rawInflate(compressedFileTable, int(header.FileCount)*16)
	if err != nil {
		return archiveLayout{}, fmt.Errorf("inflate file table: %w", err)
	}
	fileEntries, err := parseFileEntries(fileTableRaw, int(header.FileCount))
	if err != nil {
		return archiveLayout{}, err
	}

	if _, err := file.Seek(alignOffset(mustCurrentOffset(file)), io.SeekStart); err != nil {
		return archiveLayout{}, err
	}

	var compressedChunkTableSize int32
	var chunkCount int32
	if err := binary.Read(file, binary.LittleEndian, &compressedChunkTableSize); err != nil {
		return archiveLayout{}, err
	}
	if err := binary.Read(file, binary.LittleEndian, &chunkCount); err != nil {
		return archiveLayout{}, err
	}
	if compressedChunkTableSize < 0 || chunkCount < 0 {
		return archiveLayout{}, fmt.Errorf("invalid chunk table header")
	}
	compressedChunkTable := make([]byte, compressedChunkTableSize)
	if _, err := io.ReadFull(file, compressedChunkTable); err != nil {
		return archiveLayout{}, err
	}
	rawCompressedChunkTable := append([]byte(nil), compressedChunkTable...)
	if err := xorFirstWord(compressedChunkTable, uint32(header.FileCount+compressedChunkTableSize+chunkCount)); err != nil {
		return archiveLayout{}, err
	}

	chunkTableRaw, err := rawInflate(compressedChunkTable, int(chunkCount)*16)
	if err != nil {
		return archiveLayout{}, fmt.Errorf("inflate chunk table: %w", err)
	}
	chunks, err := parseChunkEntries(chunkTableRaw, int(chunkCount))
	if err != nil {
		return archiveLayout{}, err
	}

	headerEnd := alignOffset(mustCurrentOffset(file))
	if _, err := file.Seek(headerEnd, io.SeekStart); err != nil {
		return archiveLayout{}, err
	}

	files := make([]archiveFile, 0, len(fileEntries))
	for _, entry := range fileEntries {
		name, ok := nameIndex.Get(entry.NameOffset)
		if !ok {
			return archiveLayout{}, fmt.Errorf("missing filename for name offset %d", entry.NameOffset)
		}
		files = append(files, archiveFile{
			ArchivePath: name,
			NameOffset:  entry.NameOffset,
			FileSize:    entry.FileSize,
			ChunkCount:  entry.ChunkCount,
			ChunkIndex:  entry.ChunkIndex,
		})
	}

	return archiveLayout{
		Header:                   header,
		NameRecords:              nameRecords,
		FileIndex:                fileIndex,
		FileEntries:              fileEntries,
		Files:                    files,
		Chunks:                   chunks,
		CompressedNameTable:      rawCompressedNames,
		CompressedFileTable:      rawCompressedFileTable,
		CompressedChunkTable:     rawCompressedChunkTable,
		CompressedFileTableSize:  compressedFileTableSize,
		CompressedChunkTableSize: compressedChunkTableSize,
		HeaderEnd:                headerEnd,
	}, nil
}

func decodeNameTable(raw []byte, fileCount int) ([]nameRecord, nameOffsetIndex, error) {
	records := make([]nameRecord, 0, fileCount)
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

		encoded := append([]byte(nil), raw[offset:offset+length]...)
		mask := (length % 5) + entryIndex
		for indexValue := 0; indexValue < length; indexValue++ {
			encoded[indexValue] ^= byte(((indexValue + length) * 2) + mask)
		}

		stringOffset := int32(offset)
		name := string(encoded)
		index.Put(stringOffset, name)
		records = append(records, nameRecord{
			Offset: stringOffset,
			Name:   name,
		})
		offset += length + 1
	}

	return records, index, nil
}

func parseFileEntries(raw []byte, count int) ([]fileEntry, error) {
	if len(raw) != count*16 {
		return nil, fmt.Errorf("invalid file table length %d", len(raw))
	}

	entries := make([]fileEntry, 0, count)
	reader := bytes.NewReader(raw)
	for item := 0; item < count; item++ {
		entry := fileEntry{}
		if err := binary.Read(reader, binary.LittleEndian, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func parseChunkEntries(raw []byte, count int) ([]chunkEntry, error) {
	if len(raw) != count*16 {
		return nil, fmt.Errorf("invalid chunk table length %d", len(raw))
	}

	entries := make([]chunkEntry, 0, count)
	reader := bytes.NewReader(raw)
	for item := 0; item < count; item++ {
		entry := chunkEntry{}
		if err := binary.Read(reader, binary.LittleEndian, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func parseInt32Table(raw []byte, count int) ([]int32, error) {
	if len(raw) != count*4 {
		return nil, fmt.Errorf("invalid int32 table length %d", len(raw))
	}

	values := make([]int32, 0, count)
	reader := bytes.NewReader(raw)
	for item := 0; item < count; item++ {
		var value int32
		if err := binary.Read(reader, binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	return values, nil
}

func (reader *archiveReader) ExtractAll(outputDir string) error {
	for _, fileEntry := range reader.layout.Files {
		data, err := reader.readFile(fileEntry)
		if err != nil {
			return err
		}

		targetPath := outputPath(outputDir, fileEntry.ArchivePath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func (reader *archiveReader) readFile(fileEntry archiveFile) ([]byte, error) {
	if fileEntry.ChunkCount < 0 {
		return nil, fmt.Errorf("invalid negative chunk count for %s", fileEntry.ArchivePath)
	}

	buffer := make([]byte, 0, fileEntry.FileSize)
	for chunkOffset := int32(0); chunkOffset < fileEntry.ChunkCount; chunkOffset++ {
		chunkIndex := fileEntry.ChunkIndex + chunkOffset
		if chunkIndex < 0 || int(chunkIndex) >= len(reader.layout.Chunks) {
			return nil, fmt.Errorf("chunk index %d out of range for %s", chunkIndex, fileEntry.ArchivePath)
		}

		chunk := reader.layout.Chunks[chunkIndex]
		payload := make([]byte, chunk.CompressedSize)
		if _, err := reader.file.ReadAt(payload, reader.layout.HeaderEnd+int64(chunk.DataOffset)); err != nil {
			return nil, err
		}

		if chunk.CompressedSize == chunk.UncompressedSize {
			buffer = append(buffer, payload...)
			continue
		}

		inflated, err := rawInflate(payload, int(chunk.UncompressedSize))
		if err != nil {
			return nil, fmt.Errorf("inflate chunk %d for %s: %w", chunkIndex, fileEntry.ArchivePath, err)
		}
		buffer = append(buffer, inflated...)
	}

	return buffer, nil
}

func (reader *archiveReader) readRawChunk(chunk chunkEntry) ([]byte, error) {
	payload := make([]byte, chunk.CompressedSize)
	if _, err := reader.file.ReadAt(payload, reader.layout.HeaderEnd+int64(chunk.DataOffset)); err != nil {
		return nil, err
	}
	return payload, nil
}

func mustCurrentOffset(file *os.File) int64 {
	offset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		panic(err)
	}
	return offset
}
