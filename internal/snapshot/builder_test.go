//go:build luajit && !windows

package snapshot

import (
	"flag"
	"path/filepath"
	"runtime"
	"testing"

	"defgraph/internal/loader"
)

var compiledRootFlag = flag.String("compiled-root", "", "compiled snapshot root for luajit integration tests")

func TestBuildAndValidateSnapshot(t *testing.T) {
	repoRoot := testRepoRoot(t)
	if *compiledRootFlag == "" {
		t.Skip("set -compiled-root to run luajit integration tests")
	}
	world, err := loader.Load(repoRoot, *compiledRootFlag)
	if err != nil {
		t.Fatalf("load world: %v", err)
	}

	snapshotData, err := Build(world)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}

	if err := Validate(snapshotData); err != nil {
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
