# Explain Why the Executor Was Removed

## What was the Executor?

In the original project, `executor.go` was responsible for:

1. **Executing a Plan**: Take the plan from `planner.go` (a DAG of tasks) and execute tasks in dependency order
2. **Parallel execution**: Tasks with no dependencies ran concurrently in goroutines
3. **Retry logic**: On failure, call `suggestAlternative()` which asked another LLM model for a workaround
4. **Result collection**: Aggregate results from all goroutines

## Why it was removed

### 1. Not needed for chatbot use case

The Executor was designed for complex multi-step workflows with branching dependencies. The actual usage pattern is:
- User asks a question
- LLM calls 1-3 tools in sequence
- LLM synthesises the answer

That's a simple loop, not a DAG execution.

### 2. Added complexity without value

```go
// Original approach
Plan → TopologicalSort → ParallelGroups → ExecuteGroup → Collect → RetryFailed → SuggestAlternative

// Refactored approach
for i := 0; i < 5; i++ {
    resp := callLLM(messages, tools)
    for each tool_call {
        result := execute(tool_call)
        messages = append(messages, result)
    }
}
```

### 3. Parallel execution introduced race conditions

Running 3 tool calls in parallel sounded good, but:
- Rate limits were hit faster
- Results arrived in unpredictable order
- Context management was more complex
- Each tool result needed careful merging

### 4. The retry-with-alternative was over-engineered

`suggestAlternative()` called a *different* LLM model to suggest a workaround when a tool failed. This doubled LLM costs for error paths and rarely produced useful alternatives for deterministic tools like weather or crypto prices.

### 5. Self-correction lived in the wrong place

The original architecture had self-correction in the Executor, but it should be in the agent loop. The refactored version keeps it simple: if a tool call fails, the error text is returned to the LLM, which can choose to retry or explain the failure.

## What was preserved

| Concept | Where it went |
|---------|--------------|
| Multiple tool calls in one response | Still handled in `RunAgent` loop |
| Tool call retry | LLM receives error text, may retry |
| Result collection | Accumulated in `steps []common.ToolStep` |
| Final synthesis | `SynthesizeAnswer` in manager.go |
