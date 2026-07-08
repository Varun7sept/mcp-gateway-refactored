# internal/ai

## Why

Single entry point for all LLM interactions: orchestrates tool calling,
stores conversation memory, plans multi-step tasks, and synthesises answers.

## Who calls it

- `internal/web/handlers/chat.go` — via `Manager.RunAgent`

## Responsibilities

- Convert natural language → tool calls via LLM
- Fallback across multiple LLM models
- In-memory keyword-scored memory store
- Lightweight task planning for multi-step queries
- Final answer synthesis from tool results

## Public entry points

```go
func New(apiKey string) *Manager
func (m *Manager) Memory() *MemoryStore
func (m *Manager) DecideAction(ctx context.Context, userMessage string, history []map[string]string) (*ToolCallResult, error)
func (m *Manager) SynthesizeAnswer(ctx context.Context, userMessage, toolName, toolCallID, toolResult string) (string, error)
func (m *Manager) RunAgent(ctx context.Context, userMessage string, history []map[string]string, callTool func(context.Context, string, map[string]any) (string, error)) (string, []common.ToolStep, error)
```
