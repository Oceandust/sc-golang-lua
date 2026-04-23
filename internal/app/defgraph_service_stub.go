//go:build !luajit || windows

package app

import (
	"log"

	"defgraph/internal/types"
)

type unavailableDefgraphService struct{}

func newDefgraphService() defgraphService {
	return unavailableDefgraphService{}
}

func (unavailableDefgraphService) LoadSnapshot(_, _ string) (*types.Snapshot, error) {
	log.Fatalln("go fix luajit windows idk i dont care")
	return nil, nil
}
