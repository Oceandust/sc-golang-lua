//go:build luajit && !windows

package app

import (
	"defgraph/internal/loader"
	"defgraph/internal/snapshot"
	"defgraph/internal/types"
)

type luajitDefgraphService struct{}

func newDefgraphService() defgraphService {
	return luajitDefgraphService{}
}

func (luajitDefgraphService) LoadSnapshot(repoRoot string, compiledRoot string) (*types.Snapshot, error) {
	world, err := loader.Load(loader.NormalizeRepoRoot(repoRoot), loader.NormalizeCompiledRoot(compiledRoot))
	if err != nil {
		return nil, err
	}

	return snapshot.Build(world)
}
