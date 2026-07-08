// Package mcp implements the MCP gateway: server registry, tool discovery, request forwarding, and health checks.
package mcp

import (
	"fmt"
	"sync"
	"time"

	"github.com/varunbanda/mcp-gateway/internal/common"
)

type ServerStatus string

const (
	StatusOnline  ServerStatus = "online"
	StatusOffline ServerStatus = "offline"
	StatusUnknown ServerStatus = "unknown"
)

type Tool struct {
	Name       string `json:"name"`
	Desc       string `json:"description"`
	ServerName string `json:"server_name"`
}

type ConnectedServer struct {
	Name      string
	URL       string
	Status    ServerStatus
	Tools     []Tool
	LastCheck time.Time
	Latency   time.Duration
}

type Registry struct {
	mu      sync.RWMutex
	servers map[string]*ConnectedServer
}

func NewRegistry(cfg *common.Config) *Registry {
	r := &Registry{servers: make(map[string]*ConnectedServer)}
	for _, sc := range cfg.Servers {
		if !sc.Enabled {
			continue
		}
		r.servers[sc.Name] = &ConnectedServer{
			Name: sc.Name, URL: sc.URL,
			Status: StatusUnknown, Tools: []Tool{},
		}
	}
	return r
}

func (r *Registry) ListServers() []ConnectedServer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ConnectedServer, 0, len(r.servers))
	for _, s := range r.servers {
		result = append(result, *s)
	}
	return result
}

func (r *Registry) ListTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var all []Tool
	for _, s := range r.servers {
		if s.Status == StatusOnline {
			all = append(all, s.Tools...)
		}
	}
	return all
}

func (r *Registry) FindServerByTool(toolName string) (ConnectedServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.servers {
		for _, t := range s.Tools {
			if t.Name == toolName {
				return *s, nil
			}
		}
	}
	return ConnectedServer{}, fmt.Errorf("no server for tool %q", toolName)
}

func (r *Registry) GetServer(name string) (ConnectedServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.servers[name]
	if !ok {
		return ConnectedServer{}, fmt.Errorf("server %q not found", name)
	}
	return *s, nil
}

func (r *Registry) UpdateStatus(name string, status ServerStatus, tools []Tool, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.servers[name]; ok {
		s.Status = status
		s.LastCheck = time.Now()
		s.Latency = latency
		if tools != nil {
			s.Tools = tools
		}
	}
}
