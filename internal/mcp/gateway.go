package mcp

import "github.com/varunbanda/mcp-gateway/internal/common"

type Gateway struct {
	registry *Registry
}

func New(cfg *common.Config) *Gateway {
	return &Gateway{
		registry: NewRegistry(cfg),
	}
}

func (g *Gateway) Registry() *Registry {
	return g.registry
}
