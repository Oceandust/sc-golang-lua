//go:build !luajit || windows

package app

import (
	"log"

	"sc_cli/internal/defgraph"
)

type unavailableDefgraphService struct{}

func newDefgraphService() defgraphService {
	return unavailableDefgraphService{}
}

func (unavailableDefgraphService) LoadSnapshot(_, _ string) (*defgraph.Snapshot, error) {
	log.Fatalln("go fix luajit windows idk i dont care")
	return nil, nil
}
