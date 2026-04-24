package tpak

import "sc_cli/internal/collections"

const (
	archiveExtension       = ".pak"
	headerSignature        = "TPAK"
	formatVersion    int32 = 7
	headerUnknown1   int32 = 0
	headerReserved   int32 = -29
	tableAlignment         = 4
)

type Result struct {
	ArchiveCount int    `json:"archive_count"`
	OutputDir    string `json:"output_dir"`
}

type UnpackOptions struct {
	DumpMetadata bool
	Threads      int
}

type ArchiveInfo struct {
	Path        string                  `json:"path,omitempty"`
	Header      ArchiveHeaderInfo       `json:"header"`
	Tables      ArchiveTablesInfo       `json:"tables"`
	NameRecords []ArchiveNameRecordInfo `json:"name_records"`
	FileIndex   []int32                 `json:"file_index"`
	FileEntries []ArchiveFileInfo       `json:"file_entries"`
	Files       []ArchiveFileInfo       `json:"files"`
	Chunks      []ArchiveChunkInfo      `json:"chunks"`
}

type ArchiveHeaderInfo struct {
	Signature               string `json:"signature"`
	Version                 int32  `json:"version"`
	Unknown1                int32  `json:"unknown1"`
	FileCount               int32  `json:"file_count"`
	Reserved                int32  `json:"reserved"`
	NameTableSize           int32  `json:"name_table_size"`
	CompressedNameTableSize int32  `json:"compressed_name_table_size"`
}

type ArchiveTablesInfo struct {
	NameRecordCount          int    `json:"name_record_count"`
	FileIndexCount           int    `json:"file_index_count"`
	FileEntryCount           int    `json:"file_entry_count"`
	ChunkCount               int    `json:"chunk_count"`
	CompressedFileTableSize  int32  `json:"compressed_file_table_size"`
	CompressedChunkTableSize int32  `json:"compressed_chunk_table_size"`
	HeaderEnd                int64  `json:"header_end"`
	NameTable                string `json:"name_table,omitempty"`
	FileTable                string `json:"file_table,omitempty"`
	ChunkTable               string `json:"chunk_table,omitempty"`
}

type ArchiveNameRecordInfo struct {
	Index  int    `json:"index"`
	Offset int32  `json:"offset"`
	Name   string `json:"name"`
}

type ArchiveFileInfo struct {
	Index       int                `json:"index"`
	ArchivePath string             `json:"archive_path,omitempty"`
	FileSize    int32              `json:"file_size"`
	NameOffset  int32              `json:"name_offset"`
	ChunkCount  int32              `json:"chunk_count"`
	ChunkIndex  int32              `json:"chunk_index"`
	SHA1        string             `json:"sha1,omitempty"`
	Chunks      []ArchiveChunkInfo `json:"chunks,omitempty"`
}

type ArchiveChunkInfo struct {
	Index            int    `json:"index"`
	FileOffset       int32  `json:"file_offset"`
	UncompressedSize int32  `json:"uncompressed_size"`
	DataOffset       int32  `json:"data_offset"`
	CompressedSize   int32  `json:"compressed_size"`
	Payload          string `json:"payload,omitempty"`
}

type archiveHeader struct {
	Version                 int32
	Unknown1                int32
	FileCount               int32
	Reserved                int32
	NameTableSize           int32
	CompressedNameTableSize int32
}

type fileEntry struct {
	FileSize   int32
	NameOffset int32
	ChunkCount int32
	ChunkIndex int32
}

type chunkEntry struct {
	FileOffset       int32
	UncompressedSize int32
	DataOffset       int32
	CompressedSize   int32
}

type archiveFile struct {
	ArchivePath string
	NameOffset  int32
	FileSize    int32
	ChunkCount  int32
	ChunkIndex  int32
}

type archiveLayout struct {
	Name                     string
	Header                   archiveHeader
	NameRecords              []nameRecord
	FileIndex                []int32
	FileEntries              []fileEntry
	Files                    []archiveFile
	Chunks                   []chunkEntry
	CompressedNameTable      []byte
	CompressedFileTable      []byte
	CompressedChunkTable     []byte
	CompressedFileTableSize  int32
	CompressedChunkTableSize int32
	HeaderEnd                int64
}

type sourceFile struct {
	ArchivePath string
	SourcePath  string
	NameOffset  int32
	FileSize    int32
}

type nameRecord struct {
	Offset int32
	Name   string
}

type spoolChunk struct {
	Entry chunkEntry
}

type nameOffsetIndex struct {
	items *collections.HashMap[int32, string]
}

func newNameOffsetIndex() *nameOffsetIndex {
	return &nameOffsetIndex{items: collections.NewHashMap[int32, string]()}
}

func (index *nameOffsetIndex) Put(offset int32, name string) {
	index.items.Put(offset, name)
}

func (index *nameOffsetIndex) Get(offset int32) (string, bool) {
	return index.items.Get(offset)
}
