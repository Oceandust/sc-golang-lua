package tpak

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPackAndUnpackDirectoryRoundTrip(t *testing.T) {
	inputRoot := t.TempDir()
	outputRoot := t.TempDir()
	unpackRoot := t.TempDir()

	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), []byte("print('a')\n"))
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "textures", "alpha.txt"), []byte("alpha"))
	writeFixtureFile(t, filepath.Join(inputRoot, "ui", "layout", "menu.json"), []byte("{\"menu\":true}\n"))

	result, err := PackDirectory(inputRoot, outputRoot)
	if err != nil {
		t.Fatalf("pack directory: %v", err)
	}
	if result.ArchiveCount != 2 {
		t.Fatalf("expected 2 archives, got %d", result.ArchiveCount)
	}

	if _, err := os.Stat(filepath.Join(outputRoot, "gamedata.pak")); err != nil {
		t.Fatalf("missing gamedata.pak: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputRoot, "ui.pak")); err != nil {
		t.Fatalf("missing ui.pak: %v", err)
	}

	if _, err := UnpackDirectory(outputRoot, unpackRoot); err != nil {
		t.Fatalf("unpack directory: %v", err)
	}

	assertFileBytes(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), filepath.Join(unpackRoot, "gamedata", "scripts", "a.lua"))
	assertFileBytes(t, filepath.Join(inputRoot, "gamedata", "textures", "alpha.txt"), filepath.Join(unpackRoot, "gamedata", "textures", "alpha.txt"))
	assertFileBytes(t, filepath.Join(inputRoot, "ui", "layout", "menu.json"), filepath.Join(unpackRoot, "ui", "layout", "menu.json"))
	assertPathMissing(t, filepath.Join(unpackRoot, "gamedata", metadataFileName))
	assertPathMissing(t, filepath.Join(unpackRoot, "gamedata", metadataRawDirName))
}

func TestUnpackDirectoryDumpMetadata(t *testing.T) {
	inputRoot := t.TempDir()
	outputRoot := t.TempDir()
	unpackRoot := t.TempDir()

	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), []byte("print('a')\n"))
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "textures", "alpha.txt"), []byte("alpha"))

	if _, err := PackDirectory(inputRoot, outputRoot); err != nil {
		t.Fatalf("pack directory: %v", err)
	}

	if _, err := UnpackDirectoryWithOptions(outputRoot, unpackRoot, &UnpackOptions{DumpMetadata: true}); err != nil {
		t.Fatalf("unpack directory with metadata: %v", err)
	}

	assertFileBytes(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), filepath.Join(unpackRoot, "gamedata", "scripts", "a.lua"))
	assertFileBytes(t, filepath.Join(inputRoot, "gamedata", "textures", "alpha.txt"), filepath.Join(unpackRoot, "gamedata", "textures", "alpha.txt"))
	metadataPath := filepath.Join(unpackRoot, "gamedata", metadataFileName)
	assertPathExists(t, metadataPath)
	assertPathExists(t, filepath.Join(unpackRoot, "gamedata", metadataRawDirName))

	rawMetadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var metadata ArchiveInfo
	if err := json.Unmarshal(rawMetadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata.Header.Signature != headerSignature {
		t.Fatalf("metadata signature = %q", metadata.Header.Signature)
	}
	if len(metadata.NameRecords) != 2 || len(metadata.FileEntries) != 2 || len(metadata.Chunks) != 2 {
		t.Fatalf("metadata did not use unified archive model: names=%d file_entries=%d chunks=%d", len(metadata.NameRecords), len(metadata.FileEntries), len(metadata.Chunks))
	}
	if metadata.Tables.NameTable == "" || metadata.Tables.FileTable == "" || metadata.Tables.ChunkTable == "" {
		t.Fatalf("metadata missing raw table paths: %#v", metadata.Tables)
	}
	if len(metadata.Files) != 2 || metadata.Files[0].SHA1 == "" || len(metadata.Files[0].Chunks) == 0 || metadata.Files[0].Chunks[0].Payload == "" {
		t.Fatalf("metadata missing replay fields: %#v", metadata.Files)
	}
}

func TestUnpackDirectoryWithThreadCount(t *testing.T) {
	inputRoot := t.TempDir()
	outputRoot := t.TempDir()
	unpackRoot := t.TempDir()

	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), []byte("print('a')\n"))
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "b.lua"), []byte("print('b')\n"))
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "textures", "alpha.txt"), []byte("alpha"))

	if _, err := PackDirectory(inputRoot, outputRoot); err != nil {
		t.Fatalf("pack directory: %v", err)
	}

	if _, err := UnpackDirectoryWithOptions(outputRoot, unpackRoot, &UnpackOptions{Threads: 2}); err != nil {
		t.Fatalf("unpack directory with threads: %v", err)
	}

	assertFileBytes(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), filepath.Join(unpackRoot, "gamedata", "scripts", "a.lua"))
	assertFileBytes(t, filepath.Join(inputRoot, "gamedata", "scripts", "b.lua"), filepath.Join(unpackRoot, "gamedata", "scripts", "b.lua"))
	assertFileBytes(t, filepath.Join(inputRoot, "gamedata", "textures", "alpha.txt"), filepath.Join(unpackRoot, "gamedata", "textures", "alpha.txt"))
}

func TestUnpackFileWithOptionsExtractsIntoOutputRoot(t *testing.T) {
	inputRoot := t.TempDir()
	outputRoot := t.TempDir()
	unpackRoot := t.TempDir()

	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), []byte("print('a')\n"))
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "textures", "alpha.txt"), []byte("alpha"))

	if _, err := PackDirectory(inputRoot, outputRoot); err != nil {
		t.Fatalf("pack directory: %v", err)
	}

	if _, err := UnpackFileWithOptions(filepath.Join(outputRoot, "gamedata.pak"), unpackRoot, &UnpackOptions{Threads: 2}); err != nil {
		t.Fatalf("unpack file: %v", err)
	}

	assertFileBytes(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), filepath.Join(unpackRoot, "scripts", "a.lua"))
	assertFileBytes(t, filepath.Join(inputRoot, "gamedata", "textures", "alpha.txt"), filepath.Join(unpackRoot, "textures", "alpha.txt"))
	assertPathMissing(t, filepath.Join(unpackRoot, "gamedata"))
}

func TestUnpackDirectoryRejectsInvalidThreadCount(t *testing.T) {
	if _, err := UnpackDirectoryWithOptions(t.TempDir(), t.TempDir(), &UnpackOptions{Threads: -1}); err == nil {
		t.Fatal("expected invalid thread count error")
	}
}

func TestInspectArchiveJSONModel(t *testing.T) {
	inputRoot := t.TempDir()
	outputRoot := t.TempDir()

	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), []byte("print('a')\n"))
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "textures", "alpha.txt"), []byte("alpha"))

	if _, err := PackDirectory(inputRoot, outputRoot); err != nil {
		t.Fatalf("pack directory: %v", err)
	}

	inspection, err := InspectArchive(filepath.Join(outputRoot, "gamedata.pak"))
	if err != nil {
		t.Fatalf("inspect archive: %v", err)
	}
	data, err := json.Marshal(inspection)
	if err != nil {
		t.Fatalf("marshal inspection: %v", err)
	}
	if !json.Valid(data) {
		t.Fatal("inspection JSON is invalid")
	}
	if inspection.Header.Signature != headerSignature {
		t.Fatalf("unexpected signature %s", inspection.Header.Signature)
	}
	if len(inspection.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(inspection.Files))
	}
	if len(inspection.Chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(inspection.Chunks))
	}
	if len(inspection.NameRecords) != 2 || len(inspection.FileEntries) != 2 {
		t.Fatalf("expected unified model details, got names=%d file_entries=%d", len(inspection.NameRecords), len(inspection.FileEntries))
	}
	if inspection.Files[0].ArchivePath != archivePathFromRelative("scripts/a.lua") {
		t.Fatalf("unexpected first file path %s", inspection.Files[0].ArchivePath)
	}
}

func TestWriteArchiveInspectionText(t *testing.T) {
	inputRoot := t.TempDir()
	outputRoot := t.TempDir()

	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), []byte("print('a')\n"))

	if _, err := PackDirectory(inputRoot, outputRoot); err != nil {
		t.Fatalf("pack directory: %v", err)
	}

	inspection, err := InspectArchive(filepath.Join(outputRoot, "gamedata.pak"))
	if err != nil {
		t.Fatalf("inspect archive: %v", err)
	}

	var buffer bytes.Buffer
	if err := WriteArchiveInspectionText(&buffer, inspection); err != nil {
		t.Fatalf("write inspection text: %v", err)
	}

	text := buffer.String()
	for _, expected := range []string{"header:", "tables:", "files:", "chunks:", "scripts\\\\a.lua"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("inspection text missing %q:\n%s", expected, text)
		}
	}
}

func TestPackDirectoryDeterministic(t *testing.T) {
	inputRoot := t.TempDir()
	firstOutput := t.TempDir()
	secondOutput := t.TempDir()

	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "a.lua"), []byte("print('same')\n"))
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "b.lua"), []byte("print('stable')\n"))

	if _, err := PackDirectory(inputRoot, firstOutput); err != nil {
		t.Fatalf("first pack: %v", err)
	}
	if _, err := PackDirectory(inputRoot, secondOutput); err != nil {
		t.Fatalf("second pack: %v", err)
	}

	firstBytes, err := os.ReadFile(filepath.Join(firstOutput, "gamedata.pak"))
	if err != nil {
		t.Fatalf("read first output: %v", err)
	}
	secondBytes, err := os.ReadFile(filepath.Join(secondOutput, "gamedata.pak"))
	if err != nil {
		t.Fatalf("read second output: %v", err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("pack output is not deterministic")
	}
}

func TestOpenRealArchiveSample(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	samplePath := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "gamedata.pak"))
	if _, err := os.Stat(samplePath); err != nil {
		t.Skip("gamedata.pak sample is not present")
	}

	reader, err := openArchive(samplePath)
	if err != nil {
		t.Fatalf("open real archive sample: %v", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("close real archive sample: %v", closeErr)
		}
	}()

	if len(reader.layout.Files) == 0 {
		t.Fatal("real archive sample parsed with no files")
	}
}

func TestRealArchiveRoundTripPreservesOrderingMetadata(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	originalPath := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "roundtrip", "in", "gamedata.pak"))
	if _, err := os.Stat(originalPath); err != nil {
		t.Skipf("original roundtrip sample is not present: %v", err)
	}

	unpackInput := t.TempDir()
	unpackOutput := t.TempDir()
	repackOutput := t.TempDir()

	originalBytes, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("read original sample: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unpackInput, "gamedata.pak"), originalBytes, 0o644); err != nil {
		t.Fatalf("write unpack input: %v", err)
	}

	if _, err := UnpackDirectoryWithOptions(unpackInput, unpackOutput, &UnpackOptions{DumpMetadata: true}); err != nil {
		t.Fatalf("unpack original sample: %v", err)
	}
	if _, err := PackDirectory(unpackOutput, repackOutput); err != nil {
		t.Fatalf("repack unpacked sample: %v", err)
	}

	original, err := openArchive(originalPath)
	if err != nil {
		t.Fatalf("open original sample: %v", err)
	}
	defer func() {
		if closeErr := original.Close(); closeErr != nil {
			t.Errorf("close original sample: %v", closeErr)
		}
	}()

	repacked, err := openArchive(filepath.Join(repackOutput, "gamedata.pak"))
	if err != nil {
		t.Fatalf("open repacked sample: %v", err)
	}
	defer func() {
		if closeErr := repacked.Close(); closeErr != nil {
			t.Errorf("close repacked sample: %v", closeErr)
		}
	}()

	if len(original.layout.FileIndex) != len(repacked.layout.FileIndex) {
		t.Fatalf("file index length mismatch: %d != %d", len(original.layout.FileIndex), len(repacked.layout.FileIndex))
	}
	for index := range original.layout.FileIndex {
		if original.layout.FileIndex[index] != repacked.layout.FileIndex[index] {
			t.Fatalf("file index mismatch at %d: %d != %d", index, original.layout.FileIndex[index], repacked.layout.FileIndex[index])
		}
	}

	if len(original.layout.Files) != len(repacked.layout.Files) {
		t.Fatalf("file count mismatch: %d != %d", len(original.layout.Files), len(repacked.layout.Files))
	}
	for index := range original.layout.Files {
		if original.layout.Files[index].ArchivePath != repacked.layout.Files[index].ArchivePath {
			t.Fatalf("file order mismatch at %d: %s != %s", index, original.layout.Files[index].ArchivePath, repacked.layout.Files[index].ArchivePath)
		}
	}
}

func TestRealArchiveRoundTripIsByteIdentical(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	originalPath := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "roundtrip", "in", "gamedata.pak"))
	if _, err := os.Stat(originalPath); err != nil {
		t.Skipf("original roundtrip sample is not present: %v", err)
	}

	unpackInput := t.TempDir()
	unpackOutput := t.TempDir()
	repackOutput := t.TempDir()

	originalBytes, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("read original sample: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unpackInput, "gamedata.pak"), originalBytes, 0o644); err != nil {
		t.Fatalf("write unpack input: %v", err)
	}

	if _, err := UnpackDirectoryWithOptions(unpackInput, unpackOutput, &UnpackOptions{DumpMetadata: true}); err != nil {
		t.Fatalf("unpack original sample: %v", err)
	}
	if _, err := PackDirectory(unpackOutput, repackOutput); err != nil {
		t.Fatalf("repack unpacked sample: %v", err)
	}

	repackedBytes, err := os.ReadFile(filepath.Join(repackOutput, "gamedata.pak"))
	if err != nil {
		t.Fatalf("read repacked sample: %v", err)
	}
	if !bytes.Equal(originalBytes, repackedBytes) {
		t.Fatal("repacked archive is not byte-identical to the original")
	}
}

func TestSampleArchiveMetadataDiagnostics(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	originalPath := filepath.Join(root, "roundtrip", "in", "gamedata.pak")
	repackedPath := filepath.Join(root, "roundtrip", "repacked", "gamedata.pak")
	if _, err := os.Stat(originalPath); err != nil {
		t.Skipf("original sample missing: %v", err)
	}
	if _, err := os.Stat(repackedPath); err != nil {
		t.Skipf("repacked sample missing: %v", err)
	}

	original, err := openArchive(originalPath)
	if err != nil {
		t.Fatalf("open original: %v", err)
	}
	defer func() {
		if closeErr := original.Close(); closeErr != nil {
			t.Errorf("close original diagnostics sample: %v", closeErr)
		}
	}()

	repacked, err := openArchive(repackedPath)
	if err != nil {
		t.Fatalf("open repacked: %v", err)
	}
	defer func() {
		if closeErr := repacked.Close(); closeErr != nil {
			t.Errorf("close repacked diagnostics sample: %v", closeErr)
		}
	}()

	t.Logf("original: fileCount=%d chunkCount=%d fileTableCompressed=%d chunkTableCompressed=%d", len(original.layout.Files), len(original.layout.Chunks), original.layout.CompressedFileTableSize, original.layout.CompressedChunkTableSize)
	t.Logf("repacked: fileCount=%d chunkCount=%d fileTableCompressed=%d chunkTableCompressed=%d", len(repacked.layout.Files), len(repacked.layout.Chunks), repacked.layout.CompressedFileTableSize, repacked.layout.CompressedChunkTableSize)
	t.Logf("original header=%+v", original.layout.Header)
	t.Logf("repacked header=%+v", repacked.layout.Header)
	t.Logf("original first indexes=%v", headInt32(original.layout.FileIndex, 12))
	t.Logf("repacked first indexes=%v", headInt32(repacked.layout.FileIndex, 12))
	t.Logf("original first file entries=%v", headFileEntries(original.layout.FileEntries, 5))
	t.Logf("repacked first file entries=%v", headFileEntries(repacked.layout.FileEntries, 5))
	t.Logf("original first file names=%v", headArchivePaths(original.layout.Files, 8))
	t.Logf("repacked first file names=%v", headArchivePaths(repacked.layout.Files, 8))
	t.Logf("original first chunk entries=%v", headChunkEntries(original.layout.Chunks, 5))
	t.Logf("repacked first chunk entries=%v", headChunkEntries(repacked.layout.Chunks, 5))
	t.Logf("original index mismatches from sequential=%d", countSequentialMismatches(original.layout.FileIndex))
	t.Logf("repacked index mismatches from sequential=%d", countSequentialMismatches(repacked.layout.FileIndex))
}

func writeFixtureFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertFileBytes(t *testing.T, leftPath string, rightPath string) {
	t.Helper()
	left, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatalf("read %s: %v", leftPath, err)
	}
	right, err := os.ReadFile(rightPath)
	if err != nil {
		t.Fatalf("read %s: %v", rightPath, err)
	}
	if !bytes.Equal(left, right) {
		t.Fatalf("file mismatch: %s != %s", leftPath, rightPath)
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, got stat error %v", path, err)
	}
}

func headInt32(values []int32, count int) []int32 {
	if len(values) < count {
		count = len(values)
	}
	out := make([]int32, count)
	copy(out, values[:count])
	return out
}

func headFileEntries(values []fileEntry, count int) []fileEntry {
	if len(values) < count {
		count = len(values)
	}
	out := make([]fileEntry, count)
	copy(out, values[:count])
	return out
}

func headChunkEntries(values []chunkEntry, count int) []chunkEntry {
	if len(values) < count {
		count = len(values)
	}
	out := make([]chunkEntry, count)
	copy(out, values[:count])
	return out
}

func countSequentialMismatches(values []int32) int {
	mismatches := 0
	for index, value := range values {
		if value != int32(index) {
			mismatches++
		}
	}
	return mismatches
}

func headArchivePaths(values []archiveFile, count int) []string {
	if len(values) < count {
		count = len(values)
	}
	out := make([]string, 0, count)
	for index := 0; index < count; index++ {
		out = append(out, values[index].ArchivePath)
	}
	return out
}
