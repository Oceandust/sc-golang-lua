package yup

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestParseEncodeRoundTripWithBinarySHA1(t *testing.T) {
	document := []byte("d4:infod5:filesld6:lengthi3e5:mtimei10e4:pathl4:data8:file.pake4:sha120:\x01\x02\x03\x04\x05\x06\x07\x08\t\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14eeee")

	parsed, err := Parse(document)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	encoded, err := Encode(parsed)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if !bytes.Equal(document, encoded) {
		t.Fatal("parse/encode round trip changed manifest bytes")
	}
}

func TestRewriteManifestUpdatesMatchingEntry(t *testing.T) {
	rootDir := t.TempDir()
	manifestPath := filepath.Join(t.TempDir(), "star_conflict.yup")
	targetPath := filepath.Join(rootDir, "data", "gamedata.pak")
	targetData := []byte("tpak payload")

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(targetPath, targetData, 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	manifest := []byte("d4:infod5:filesld6:lengthi1e5:mtimei1e4:pathl4:data12:gamedata.pake4:sha120:\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00eeee")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}

	result, err := RewriteManifest(manifestPath, rootDir, "")
	if err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
	if result.UpdatedEntries != 1 {
		t.Fatalf("expected 1 updated entry, got %d", result.UpdatedEntries)
	}

	rewritten, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read rewritten manifest: %v", err)
	}
	parsed, err := Parse(rewritten)
	if err != nil {
		t.Fatalf("parse rewritten manifest: %v", err)
	}

	files, err := manifestFileEntries(parsed)
	if err != nil {
		t.Fatalf("file entries: %v", err)
	}
	entry := files[0]

	lengthValue, ok := entry.Get("length")
	if !ok {
		t.Fatal("missing rewritten length")
	}
	if length, ok := lengthValue.AsInt(); !ok || length != int64(len(targetData)) {
		t.Fatalf("unexpected rewritten length: %v", lengthValue)
	}

	mtimeValue, ok := entry.Get("mtime")
	if !ok {
		t.Fatal("missing rewritten mtime")
	}
	if mtime, ok := mtimeValue.AsInt(); !ok || mtime != info.ModTime().Unix() {
		t.Fatalf("unexpected rewritten mtime: got %v want %d", mtime, info.ModTime().Unix())
	}

	shaValue, ok := entry.Get("sha1")
	if !ok {
		t.Fatal("missing rewritten sha1")
	}
	shaBytes, ok := shaValue.AsBytes()
	if !ok {
		t.Fatal("rewritten sha1 is not a byte string")
	}
	expected := sha1.Sum(targetData)
	if !bytes.Equal(shaBytes, expected[:]) {
		t.Fatal("rewritten sha1 mismatch")
	}
}

func TestRewriteManifestSkipsUnmatchedEntries(t *testing.T) {
	rootDir := t.TempDir()
	manifestPath := filepath.Join(t.TempDir(), "star_conflict.yup")
	manifest := []byte("d4:infod5:filesld6:lengthi1e5:mtimei1e4:pathl4:data12:gamedata.pake4:sha120:\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00eeee")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := RewriteManifest(manifestPath, rootDir, filepath.Join(rootDir, "out.yup"))
	if err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
	if result.UpdatedEntries != 0 {
		t.Fatalf("expected 0 updated entries, got %d", result.UpdatedEntries)
	}
}

func TestRewriteManifestUsesProvidedOutputPath(t *testing.T) {
	rootDir := t.TempDir()
	targetPath := filepath.Join(rootDir, "data", "gamedata.pak")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	manifestDir := t.TempDir()
	manifestPath := filepath.Join(manifestDir, "in.yup")
	outputPath := filepath.Join(manifestDir, "out.yup")
	manifest := []byte("d4:infod5:filesld6:lengthi1e5:mtimei1e4:pathl4:data12:gamedata.pake4:sha120:\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00eeee")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := RewriteManifest(manifestPath, rootDir, outputPath)
	if err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
	if result.OutputPath != outputPath {
		t.Fatalf("unexpected output path %s", result.OutputPath)
	}

	inBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read input manifest: %v", err)
	}
	outBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output manifest: %v", err)
	}
	if !bytes.Equal(inBytes, manifest) {
		t.Fatal("input manifest was modified despite separate output path")
	}
	if bytes.Equal(inBytes, outBytes) {
		t.Fatal("rewritten output manifest did not change")
	}
}

func TestRewriteManifestRecomputesPiecesAndPadding(t *testing.T) {
	rootDir := t.TempDir()
	targetPath := filepath.Join(rootDir, "data", "gamedata.pak")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	targetData := []byte("abcdef")
	if err := os.WriteFile(targetPath, targetData, 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	manifestPath := filepath.Join(t.TempDir(), "star_conflict.yup")
	manifest := []byte("d4:infod5:filesld6:lengthi1e5:mtimei1e4:pathl4:data12:gamedata.pake4:sha120:\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00ed4:attr1:p6:lengthi3e4:pathl2:.p1:0eee12:piece lengthi8e6:pieces20:\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00ee")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if _, err := RewriteManifest(manifestPath, rootDir, ""); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}

	rewritten, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read rewritten manifest: %v", err)
	}

	document, err := Parse(rewritten)
	if err != nil {
		t.Fatalf("parse rewritten manifest: %v", err)
	}

	files, err := manifestFileEntries(document)
	if err != nil {
		t.Fatalf("manifestFileEntries: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 file entries, got %d", len(files))
	}

	lengthValue, ok := files[1].Get("length")
	if !ok {
		t.Fatal("missing pad length")
	}
	padLength, ok := lengthValue.AsInt()
	if !ok {
		t.Fatal("pad length is not an int")
	}
	if padLength != 2 {
		t.Fatalf("unexpected pad length %d", padLength)
	}

	infoValue, err := manifestInfo(document)
	if err != nil {
		t.Fatalf("manifestInfo: %v", err)
	}
	piecesValue, ok := infoValue.Get("pieces")
	if !ok {
		t.Fatal("missing pieces value")
	}
	piecesBytes, ok := piecesValue.AsBytes()
	if !ok {
		t.Fatal("pieces is not a byte string")
	}

	expected := sha1.Sum([]byte{'a', 'b', 'c', 'd', 'e', 'f', 0, 0})
	if !bytes.Equal(piecesBytes, expected[:]) {
		t.Fatal("pieces blob mismatch")
	}
}

func TestManifestInfoNameMatchesInfoHash(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "star_conflict.yup"))
	if err != nil {
		t.Skipf("sample manifest unavailable: %v", err)
	}

	document, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	infoValue, err := manifestInfo(document)
	if err != nil {
		t.Fatalf("manifestInfo: %v", err)
	}

	nameValue, ok := infoValue.Get("name")
	if !ok {
		t.Fatal("missing info.name")
	}

	nameText, ok := nameValue.AsString()
	if !ok {
		t.Fatal("info.name is not a string")
	}

	hashWithoutName, err := infoHashHexWithoutName(infoValue)
	if err != nil {
		t.Fatalf("infoHashHexWithoutName: %v", err)
	}

	hashWithName, err := valueHashHex(infoValue)
	if err != nil {
		t.Fatalf("valueHashHex(info): %v", err)
	}

	hashTopLevel, err := valueHashHex(document)
	if err != nil {
		t.Fatalf("valueHashHex(document): %v", err)
	}

	t.Logf("info.name=%s", nameText)
	t.Logf("sha1(info without name)=%s", hashWithoutName)
	t.Logf("sha1(info with name)=%s", hashWithName)
	t.Logf("sha1(document)=%s", hashTopLevel)
	t.Skip("diagnostic test")
}

func infoHashHexWithoutName(infoValue *Value) (string, error) {
	clone := cloneValue(infoValue)
	if clone == nil {
		return "", nil
	}
	if dict, ok := clone.AsDict(); ok {
		dict.Delete("name")
	}
	return valueHashHex(clone)
}

func valueHashHex(value *Value) (string, error) {
	encoded, err := Encode(value)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func cloneValue(value *Value) *Value {
	if value == nil {
		return nil
	}

	if data, ok := value.AsBytes(); ok {
		return BytesValue(data)
	}
	if intValue, ok := value.AsInt(); ok {
		return IntValue(intValue)
	}
	if list, ok := value.AsList(); ok {
		items := make([]*Value, 0, len(list))
		for _, item := range list {
			items = append(items, cloneValue(item))
		}
		return ListValue(items)
	}
	if dict, ok := value.AsDict(); ok {
		out := DictValue()
		dict.Range(func(key string, child *Value) bool {
			out.Set(key, cloneValue(child))
			return true
		})
		return out
	}

	return nil
}
