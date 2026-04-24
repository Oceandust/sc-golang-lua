package tpak

import (
	"fmt"
	"io"
)

func InspectArchive(inputPak string) (*ArchiveInfo, error) {
	reader, err := openArchiveWithMetadata(inputPak, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	return archiveInfoFromLayout(inputPak, reader.layout), nil
}

func WriteArchiveInspectionText(writer io.Writer, inspection *ArchiveInfo) error {
	if inspection == nil {
		return fmt.Errorf("archive inspection is nil")
	}

	if _, err := fmt.Fprintf(writer, "archive: %s\n", inspection.Path); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "header:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  signature: %s\n", inspection.Header.Signature); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  version: %d\n", inspection.Header.Version); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  unknown1: %d\n", inspection.Header.Unknown1); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  file_count: %d\n", inspection.Header.FileCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  reserved: %d\n", inspection.Header.Reserved); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  name_table_size: %d\n", inspection.Header.NameTableSize); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  compressed_name_table_size: %d\n", inspection.Header.CompressedNameTableSize); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "tables:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  name_records: %d\n", inspection.Tables.NameRecordCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  file_index: %d\n", inspection.Tables.FileIndexCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  file_entries: %d\n", inspection.Tables.FileEntryCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  chunks: %d\n", inspection.Tables.ChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  compressed_file_table_size: %d\n", inspection.Tables.CompressedFileTableSize); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  compressed_chunk_table_size: %d\n", inspection.Tables.CompressedChunkTableSize); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "  header_end: %d\n", inspection.Tables.HeaderEnd); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, "name_records:"); err != nil {
		return err
	}
	for index := range inspection.NameRecords {
		record := &inspection.NameRecords[index]
		if _, err := fmt.Fprintf(writer, "  [%d] offset=%d name=%q\n", record.Index, record.Offset, record.Name); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(writer, "file_index:"); err != nil {
		return err
	}
	for index, value := range inspection.FileIndex {
		if _, err := fmt.Fprintf(writer, "  [%d] %d\n", index, value); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(writer, "file_entries:"); err != nil {
		return err
	}
	for index := range inspection.FileEntries {
		file := &inspection.FileEntries[index]
		if _, err := fmt.Fprintf(writer, "  [%d] size=%d name_offset=%d chunk_count=%d chunk_index=%d\n", file.Index, file.FileSize, file.NameOffset, file.ChunkCount, file.ChunkIndex); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(writer, "files:"); err != nil {
		return err
	}
	for index := range inspection.Files {
		file := &inspection.Files[index]
		if _, err := fmt.Fprintf(writer, "  [%d] path=%q size=%d name_offset=%d chunk_count=%d chunk_index=%d\n", file.Index, file.ArchivePath, file.FileSize, file.NameOffset, file.ChunkCount, file.ChunkIndex); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(writer, "chunks:"); err != nil {
		return err
	}
	for index := range inspection.Chunks {
		chunk := &inspection.Chunks[index]
		if _, err := fmt.Fprintf(writer, "  [%d] file_offset=%d uncompressed_size=%d data_offset=%d compressed_size=%d\n", chunk.Index, chunk.FileOffset, chunk.UncompressedSize, chunk.DataOffset, chunk.CompressedSize); err != nil {
			return err
		}
	}

	return nil
}

func archiveInfoFromLayout(path string, layout *archiveLayout) *ArchiveInfo {
	inspection := &ArchiveInfo{
		Path: path,
		Header: ArchiveHeaderInfo{
			Signature:               headerSignature,
			Version:                 layout.Header.Version,
			Unknown1:                layout.Header.Unknown1,
			FileCount:               layout.Header.FileCount,
			Reserved:                layout.Header.Reserved,
			NameTableSize:           layout.Header.NameTableSize,
			CompressedNameTableSize: layout.Header.CompressedNameTableSize,
		},
		Tables: ArchiveTablesInfo{
			NameRecordCount:          len(layout.NameRecords),
			FileIndexCount:           len(layout.FileIndex),
			FileEntryCount:           len(layout.FileEntries),
			ChunkCount:               len(layout.Chunks),
			CompressedFileTableSize:  layout.CompressedFileTableSize,
			CompressedChunkTableSize: layout.CompressedChunkTableSize,
			HeaderEnd:                layout.HeaderEnd,
		},
		FileIndex: make([]int32, len(layout.FileIndex)),
	}
	copy(inspection.FileIndex, layout.FileIndex)

	inspection.NameRecords = make([]ArchiveNameRecordInfo, len(layout.NameRecords))
	for index := range layout.NameRecords {
		record := &layout.NameRecords[index]
		inspection.NameRecords[index] = ArchiveNameRecordInfo{
			Index:  index,
			Offset: record.Offset,
			Name:   record.Name,
		}
	}

	inspection.FileEntries = make([]ArchiveFileInfo, len(layout.FileEntries))
	for index := range layout.FileEntries {
		entry := &layout.FileEntries[index]
		inspection.FileEntries[index] = ArchiveFileInfo{
			Index:      index,
			FileSize:   entry.FileSize,
			NameOffset: entry.NameOffset,
			ChunkCount: entry.ChunkCount,
			ChunkIndex: entry.ChunkIndex,
		}
	}

	inspection.Files = make([]ArchiveFileInfo, len(layout.Files))
	for index := range layout.Files {
		file := &layout.Files[index]
		inspection.Files[index] = ArchiveFileInfo{
			Index:       index,
			ArchivePath: file.ArchivePath,
			FileSize:    file.FileSize,
			NameOffset:  file.NameOffset,
			ChunkCount:  file.ChunkCount,
			ChunkIndex:  file.ChunkIndex,
		}
	}

	inspection.Chunks = make([]ArchiveChunkInfo, len(layout.Chunks))
	for index := range layout.Chunks {
		chunk := &layout.Chunks[index]
		inspection.Chunks[index] = ArchiveChunkInfo{
			Index:            index,
			FileOffset:       chunk.FileOffset,
			UncompressedSize: chunk.UncompressedSize,
			DataOffset:       chunk.DataOffset,
			CompressedSize:   chunk.CompressedSize,
		}
	}

	return inspection
}
