package tpak

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	expectedFiles := []string{
		archivePathFromRelative("scripts/a.lua"),
		archivePathFromRelative("textures/alpha.txt"),
	}
	for _, expected := range expectedFiles {
		if !inspectionContainsFile(inspection, expected) {
			t.Fatalf("inspection is missing file %s", expected)
		}
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

func TestPackDirectoryUsesExtensionCompressionPolicy(t *testing.T) {
	inputRoot := t.TempDir()
	outputRoot := t.TempDir()

	repeated := bytes.Repeat([]byte("compressible payload\n"), 256)
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "effects", "spark.psys"), repeated)
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "audio", "voice.ogg"), repeated)
	writeFixtureFile(t, filepath.Join(inputRoot, "gamedata", "scripts", "mixed.lua"), repeated)

	if _, err := PackDirectory(inputRoot, outputRoot); err != nil {
		t.Fatalf("pack directory: %v", err)
	}

	inspection, err := InspectArchive(filepath.Join(outputRoot, "gamedata.pak"))
	if err != nil {
		t.Fatalf("inspect archive: %v", err)
	}

	assertChunkCompressed(t, inspection, `audio\voice.ogg`, false)
	assertChunkCompressed(t, inspection, `effects\spark.psys`, true)
	assertChunkCompressed(t, inspection, `scripts\mixed.lua`, true)
}

func TestBuildFileIndexSortsLookupReferences(t *testing.T) {
	files := []sourceFile{
		{ArchivePath: `zeta\last.lua`},
		{ArchivePath: `alpha\first.lua`},
		{ArchivePath: `middle.lua`},
	}

	fileIndex := buildFileIndex(files)
	expected := []int32{1, 2, 0}
	if !slices.Equal(fileIndex, expected) {
		t.Fatalf("file index = %v, expected %v", fileIndex, expected)
	}
}

func TestRawDeflateRoundTrip(t *testing.T) {
	source := bytes.Repeat([]byte("raw deflate roundtrip payload\n"), 32)
	compressed, err := rawDeflate(source)
	if err != nil {
		t.Fatalf("raw deflate: %v", err)
	}
	inflated, err := rawInflate(compressed, len(source))
	if err != nil {
		t.Fatalf("raw inflate: %v", err)
	}
	if !bytes.Equal(inflated, source) {
		t.Fatal("raw deflate roundtrip changed payload")
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

func TestRealArchiveFreshRoundTripPreservesContents(t *testing.T) {
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
	reunpackOutput := t.TempDir()

	originalBytes, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("read original sample: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unpackInput, "gamedata.pak"), originalBytes, 0o644); err != nil {
		t.Fatalf("write unpack input: %v", err)
	}

	if _, err := UnpackDirectory(unpackInput, unpackOutput); err != nil {
		t.Fatalf("unpack original sample: %v", err)
	}
	if _, err := PackDirectory(unpackOutput, repackOutput); err != nil {
		t.Fatalf("repack unpacked sample: %v", err)
	}
	if _, err := UnpackDirectory(repackOutput, reunpackOutput); err != nil {
		t.Fatalf("unpack repacked sample: %v", err)
	}

	assertDirectoryBytes(t, filepath.Join(unpackOutput, "gamedata"), filepath.Join(reunpackOutput, "gamedata"))
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

func assertChunkCompressed(t *testing.T, inspection *ArchiveInfo, archivePath string, expected bool) {
	t.Helper()
	for _, file := range inspection.Files {
		if file.ArchivePath != archivePath {
			continue
		}
		if file.ChunkCount != 1 {
			t.Fatalf("%s chunk count = %d", archivePath, file.ChunkCount)
		}
		chunk := inspection.Chunks[file.ChunkIndex]
		compressed := chunk.CompressedSize != chunk.UncompressedSize
		if compressed != expected {
			t.Fatalf("%s compressed = %v, expected %v; chunk=%#v", archivePath, compressed, expected, chunk)
		}
		return
	}
	t.Fatalf("archive path %s not found", archivePath)
}

func inspectionContainsFile(inspection *ArchiveInfo, archivePath string) bool {
	for _, file := range inspection.Files {
		if file.ArchivePath == archivePath {
			return true
		}
	}
	return false
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, got stat error %v", path, err)
	}
}

func assertDirectoryBytes(t *testing.T, leftRoot string, rightRoot string) {
	t.Helper()
	if err := filepath.WalkDir(leftRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(leftRoot, path)
		if err != nil {
			return err
		}
		assertFileBytes(t, path, filepath.Join(rightRoot, relativePath))
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", leftRoot, err)
	}

	if err := filepath.WalkDir(rightRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(rightRoot, path)
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(leftRoot, relativePath)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", rightRoot, err)
	}
}
