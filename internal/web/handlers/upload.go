package handlers

import (
	"io"
	"net/http"
	"time"

	"github.com/varunbanda/mcp-gateway/internal/mcp"
)

type UploadHandler struct {
	Gateway *mcp.Gateway
}

func (h *UploadHandler) HandleFileUpload(w http.ResponseWriter, r *http.Request) {
	server, err := h.Gateway.Registry().GetServer("documents")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Documents server not found"})
		return
	}

	proxyReq, err := http.NewRequest("POST", server.URL+"/upload", r.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create request"})
		return
	}
	for _, hdr := range []string{"Content-Type", "Content-Length", "Accept"} {
		if v := r.Header.Get(hdr); v != "" {
			proxyReq.Header.Set(hdr, v)
		}
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "documents server unreachable"})
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	w.Write(body)
}
