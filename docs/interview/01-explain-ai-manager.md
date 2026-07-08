# Explain the AI Manager

## What it is

The AI Manager (`internal/ai/manager.go`) is the single entry point for all LLM interactions. It orchestrates the entire process of converting a user's natural language request into tool calls and synthesising a final answer.

## Files

| File | Lines | Responsibility |
|------|-------|---------------|
| manager.go | ~200 | Orchestration: `RunAgent`, `DecideAction`, `SynthesizeAnswer` |
| llm.go | ~100 | HTTP client, model fallback, request/response types |
| tool_executor.go | ~100 | Tool definitions sent to LLM |
| planner.go | ~60 | Lightweight task breakdown |
| memory.go | ~100 | In-memory store with keyword scoring |
| prompt.go | ~70 | System prompt, synthesis prompt, retry prompt |
| helpers.go | ~15 | Utility functions |

## Key methods

```go
func New(apiKey string) *Manager
func (m *Manager) Memory() *MemoryStore
func (m *Manager) DecideAction(ctx, userMessage, history) (*ToolCallResult, error)
func (m *Manager) SynthesizeAnswer(ctx, userMessage, toolName, toolCallID, toolResult) (string, error)
func (m *Manager) RunAgent(ctx, userMessage, history, callTool) (string, []ToolStep, error)
```

## Agent Loop

1. Planner analyses the user message for keyword patterns and suggests an ordered task list
2. System prompt is built from PromptBuilder (rules, available tools, memories)
3. Loop (max 5 iterations):
   - `callLLM` sends messages + 20 tool definitions to Groq API
   - If LLM returns tool calls → execute via `callTool` callback
   - If LLM returns text → answer is complete
4. On exhaustion, force a final "summarise" call

## Why was the Orchestrator removed?

The original project had 6 files (agent, brain, executor, orchestrator, planner, memory) with complex DAG-based planning and parallel execution. After refactoring, we merged everything into the Manager because:
- The simple chatbot use case didn't need topological sort or parallel execution
- Single-agent loop with max 5 steps was sufficient
- Reduced code by ~40% while preserving all functionality
