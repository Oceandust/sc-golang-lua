//go:build luajit && !windows

package app

import (
	"sc_cli/internal/defgraph"
)

type luajitDefgraphService struct{}

func newDefgraphService() defgraphService {
	return luajitDefgraphService{}
}

func (luajitDefgraphService) LoadSnapshot(repoRoot string, compiledRoot string) (*defgraph.Snapshot, error) {
	world, err := defgraph.LoadWorld(defgraph.NormalizeRepoRoot(repoRoot), defgraph.NormalizeCompiledRoot(compiledRoot))
	if err != nil {
		return nil, err
	}

	return defgraph.BuildSnapshot(world)
}
