package tpak

import "defgraph/internal/collections"

const (
	archiveExtension       = ".pak"
	headerSignature        = "TPAK"
	formatVersion    int32 = 7
	headerUnknown1   int32 = 0
	headerReserved   int32 = -29
	tableAlignment         = 4
)

type Result struct {
	ArchiveCount int
	OutputDir    string
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

func newNameOffsetIndex() nameOffsetIndex {
	return nameOffsetIndex{items: collections.NewHashMap[int32, string]()}
}

func (index nameOffsetIndex) Put(offset int32, name string) {
	index.items.Put(offset, name)
}

func (index nameOffsetIndex) Get(offset int32) (string, bool) {
	return index.items.Get(offset)
}
