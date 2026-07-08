package common

import (
	"sync"
	"time"
)

type RequestLog struct {
	ID         int           `json:"id"`
	Timestamp  time.Time     `json:"timestamp"`
	Method     string        `json:"method"`
	ToolName   string        `json:"tool_name"`
	ServerName string        `json:"server_name"`
	Username   string        `json:"username"`
	Latency    time.Duration `json:"latency_ms"`
	Status     string        `json:"status"`
	Error      string        `json:"error,omitempty"`
}

type Stats struct {
	TotalRequests    int            `json:"total_requests"`
	SuccessCount     int            `json:"success_count"`
	ErrorCount       int            `json:"error_count"`
	AvgLatencyMs     int64          `json:"avg_latency_ms"`
	RequestsByTool   map[string]int `json:"requests_by_tool"`
	RequestsByServer map[string]int `json:"requests_by_server"`
}

type Logger struct {
	mu       sync.RWMutex
	logs     []RequestLog
	maxLogs  int
	nextID   int
}

func NewLogger(maxLogs int) *Logger {
	return &Logger{
		logs:    make([]RequestLog, 0, maxLogs),
		maxLogs: maxLogs,
		nextID:  1,
	}
}

func (l *Logger) Log(method, toolName, serverName, username, status, errMsg string, latency time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := RequestLog{
		ID: l.nextID, Timestamp: time.Now(), Method: method,
		ToolName: toolName, ServerName: serverName, Username: username,
		Latency: latency, Status: status, Error: errMsg,
	}
	l.nextID++
	l.logs = append(l.logs, entry)
	if len(l.logs) > l.maxLogs {
		trimmed := make([]RequestLog, l.maxLogs)
		copy(trimmed, l.logs[len(l.logs)-l.maxLogs:])
		l.logs = trimmed
	}
}

func (l *Logger) Recent(n int, username string) []RequestLog {
	l.mu.RLock()
	defer l.mu.RUnlock()
	var filtered []RequestLog
	for i := len(l.logs) - 1; i >= 0 && len(filtered) < n; i-- {
		if username == "" || l.logs[i].Username == username {
			filtered = append(filtered, l.logs[i])
		}
	}
	return filtered
}

func (l *Logger) GetStats(username string) Stats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	stats := Stats{
		RequestsByTool:   make(map[string]int),
		RequestsByServer: make(map[string]int),
	}
	var totalLatency time.Duration
	for _, log := range l.logs {
		if username != "" && log.Username != username {
			continue
		}
		stats.TotalRequests++
		if log.Status == "success" {
			stats.SuccessCount++
		} else {
			stats.ErrorCount++
		}
		totalLatency += log.Latency
		if log.ToolName != "" {
			stats.RequestsByTool[log.ToolName]++
		}
		if log.ServerName != "" {
			stats.RequestsByServer[log.ServerName]++
		}
	}
	if stats.TotalRequests > 0 {
		stats.AvgLatencyMs = (totalLatency / time.Duration(stats.TotalRequests)).Milliseconds()
	}
	return stats
}
