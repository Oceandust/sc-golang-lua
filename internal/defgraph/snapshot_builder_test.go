//go:build luajit && !windows

package defgraph

import (
	"flag"
	"path/filepath"
	"runtime"
	"testing"
)

var compiledRootFlag = flag.String("compiled-root", "", "compiled snapshot root for luajit integration tests")

func TestBuildAndValidateSnapshot(t *testing.T) {
	repoRoot := testRepoRoot(t)
	if *compiledRootFlag == "" {
		t.Skip("set -compiled-root to run luajit integration tests")
	}
	world, err := LoadWorld(repoRoot, *compiledRootFlag)
	if err != nil {
		t.Fatalf("load world: %v", err)
	}

	snapshotData, err := BuildSnapshot(world)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}

	if err := ValidateSnapshot(snapshotData); err != nil {
		t.Fatalf("validate snapshot: %v", err)
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
