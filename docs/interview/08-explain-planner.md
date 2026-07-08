# Explain Why the Planner Exists

## The Problem

Without a planner, the LLM must decide the execution strategy purely from its training data. For multi-step queries like "What's the weather in London and the latest tech news?", the LLM might:
- Call both tools in one response
- Call one, forget the other
- Combine them incorrectly

## The Solution

The lightweight planner (`internal/ai/planner.go`) provides an explicit execution hint before the LLM sees the conversation:

```go
type planStep struct {
    Description string
    ToolName    string  // suggested tool
    Priority    int
}
```

## How it works

1. `CreatePlan(userMessage)` checks for keyword patterns:
   - "weather" + "news" → [{weather}, {news}]
   - "github" + "weather" → [{github}, {weather}]
   - "crypto" + "search" → [{crypto}, {search}]
   - "wikipedia" + "news" → [{wiki}, {news}]

2. The plan is injected into the system prompt:
   ```
   Suggested execution plan:
     1. Fetch weather data (use tool: get_weather)
     2. Fetch news headlines (use tool: get_top_news)
   Follow this plan unless the user requests something different.
   ```

3. The LLM can follow or override the plan

## Why not a full DAG-based planner?

The original project had a complete planner with:
- `DecomposeGoal()` — breaks questions into dependency graphs
- Topological sort for execution order
- Parallel execution groups
- Retry with `suggestAlternative()`

We removed this because:
- Chat queries rarely have complex dependencies
- The LLM is already good at choosing tools
- The simple keyword planner adds value without the complexity
- Execution remains inside the Manager (no separate Executor)

## Interview answer

"The planner is a lightweight addition that provides execution hints to the LLM. It uses simple keyword matching to suggest an order for multi-step queries. It's intentionally minimal — we rejected a full DAG-based planner because the LLM handles tool selection well on its own for most queries."
