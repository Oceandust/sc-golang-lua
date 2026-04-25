package tpak

import (
	"path/filepath"
	"strings"

	"sc_cli/internal/collections"
)

type chunkCompression uint8

const (
	chunkCompressionRaw chunkCompression = iota
	chunkCompressionDeflate
	chunkCompressionAuto
)

var alwaysRawExtensions = collections.NewHashSet[string](
	".fsb",
	".ogg",
	".ogv",
	".txt",
)

var alwaysDeflateExtensions = collections.NewHashSet[string](
	".cam",
	".dae",
	".fev",
	".fntp",
	".fx",
	".h",
	".list",
	".lst",
	".ma",
	".mdl-skl",
	".mdp",
	".mel",
	".mmh",
	".mmp",
	".ps",
	".psys",
	".sanim",
	".skins",
	".sot",
	".vs",
	".lua",
)

func compressionForArchivePath(archivePath string) chunkCompression {
	extension := strings.ToLower(filepath.Ext(relativePathFromArchive(archivePath)))
	if alwaysRawExtensions.Contains(extension) {
		return chunkCompressionRaw
	}
	if alwaysDeflateExtensions.Contains(extension) {
		return chunkCompressionDeflate
	}
	return chunkCompressionAuto
}
