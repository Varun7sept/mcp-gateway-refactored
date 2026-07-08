package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/varunbanda/mcp-gateway/internal/common"
)

type Manager struct {
	apiKey     string
	models     []string
	httpClient *http.Client
	memory     *MemoryStore
	prompt     *promptBuilder
	planner    *planner
	thinkRegex *regexp.Regexp
}

func New(apiKey string) *Manager {
	models := []string{"llama-3.3-70b-versatile", "qwen/qwen3-32b", "qwen/qwen3.6-27b"}
	if configured := strings.TrimSpace(os.Getenv("GROQ_MODELS")); configured != "" {
		models = nil
		for _, m := range strings.Split(configured, ",") {
			if m = strings.TrimSpace(m); m != "" {
				models = append(models, m)
			}
		}
	}
	return &Manager{
		apiKey:     apiKey,
		models:     models,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		memory:     NewMemoryStore(200),
		prompt:     newPromptBuilder(),
		planner:    newPlanner(),
		thinkRegex: regexp.MustCompile(`(?s)<think>.*?</think>\s*`),
	}
}

func (m *Manager) Memory() *MemoryStore {
	return m.memory
}

func (m *Manager) DecideAction(ctx context.Context, userMessage string, history []map[string]string) (*ToolCallResult, error) {
	memories := ""
	entries := m.memory.QueryRelevant(userMessage, 3)
	if len(entries) > 0 {
		var parts []string
		for i, e := range entries {
			parts = append(parts, fmt.Sprintf("Past interaction %d:\n  User asked: %s\n  I answered: %s\n  Tools used: %s",
				i+1, e.Query, truncate(e.Answer, 200), strings.Join(e.ToolsUsed, ", ")))
		}
		memories = "Here are relevant past conversations for context:\n\n" + strings.Join(parts, "\n\n")
	}

	messages := []llmMessage{
		{Role: "system", Content: m.prompt.SystemPrompt(memories)},
	}
	for _, h := range history {
		role := h["role"]
		if role == "ai" {
			role = "assistant"
		}
		messages = append(messages, llmMessage{Role: role, Content: h["content"]})
	}
	messages = append(messages, llmMessage{Role: "user", Content: userMessage})

	resp, err := m.callLLM(ctx, messages, m.toolDefs())
	if err != nil {
		return nil, err
	}
	choice := resp.Choices[0]

	if len(choice.Message.ToolCalls) > 0 {
		tc := choice.Message.ToolCalls[0]
		var args map[string]any
		if tc.Function.Arguments != "" {
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		if args == nil {
			args = map[string]any{}
		}
		return &ToolCallResult{
			NeedsTool: true, ToolName: tc.Function.Name,
			Arguments: args, ToolCallID: tc.ID,
		}, nil
	}

	return &ToolCallResult{
		NeedsTool: false, DirectAnswer: m.stripThink(choice.Message.Content),
	}, nil
}

func (m *Manager) SynthesizeAnswer(ctx context.Context, userMessage, toolName, toolCallID, toolResult string) (string, error) {
	messages := []llmMessage{
		{Role: "system", Content: m.prompt.SystemPrompt("")},
		{Role: "user", Content: userMessage},
		{
			Role: "assistant",
			ToolCalls: []toolCall{
				{ID: toolCallID, Type: "function", Function: functionCall{Name: toolName, Arguments: "{}"}},
			},
		},
		{Role: "tool", Content: toolResult, ToolCallID: toolCallID},
	}
	resp, err := m.callLLM(ctx, messages, nil)
	if err != nil {
		return toolResult, err
	}
	return m.stripThink(resp.Choices[0].Message.Content), nil
}

func (m *Manager) RunAgent(ctx context.Context, userMessage string, history []map[string]string, callTool func(context.Context, string, map[string]any) (string, error)) (string, []common.ToolStep, error) {
	const maxSteps = 5

	plan := m.planner.CreatePlan(userMessage)
	planGuide := ""
	if len(plan) > 0 {
		var steps []string
		for i, p := range plan {
			steps = append(steps, fmt.Sprintf("  %d. %s (use tool: %s)", i+1, p.Description, p.ToolName))
		}
		planGuide = "Suggested execution plan:\n" + strings.Join(steps, "\n") + "\nFollow this plan unless the user requests something different.\n"
	}

	messages := []llmMessage{
		{Role: "system", Content: m.prompt.SystemPrompt(planGuide)},
	}
	for _, h := range history {
		role := h["role"]
		if role == "ai" {
			role = "assistant"
		}
		messages = append(messages, llmMessage{Role: role, Content: h["content"]})
	}
	messages = append(messages, llmMessage{Role: "user", Content: userMessage})

	var steps []common.ToolStep

	docPattern := regexp.MustCompile(`(?i)([a-z0-9_().-]+\.(?:pdf|txt|md|csv|json|docx))`)
	if docMatch := docPattern.FindStringSubmatch(userMessage); len(docMatch) > 1 {
		args := map[string]any{"question": userMessage, "document_name": docMatch[1]}
		result, err := callTool(ctx, "ask_document", args)
		if err != nil {
			result = "Error: " + err.Error()
		}
		steps = append(steps, common.ToolStep{ToolName: "ask_document", Arguments: args, Result: result})
		messages = append(messages,
			llmMessage{Role: "assistant", ToolCalls: []toolCall{{ID: "forced_doc", Type: "function", Function: functionCall{Name: "ask_document", Arguments: "{}"}}}},
			llmMessage{Role: "tool", Content: result, ToolCallID: "forced_doc"},
			llmMessage{Role: "system", Content: "Answer using only the retrieved passages above."},
		)
	}

	for i := 0; i < maxSteps; i++ {
		resp, err := m.callLLM(ctx, messages, m.toolDefs())
		if err != nil {
			return "", steps, fmt.Errorf("step %d: %w", i+1, err)
		}
		choice := resp.Choices[0]

		if len(choice.Message.ToolCalls) == 0 {
			answer := m.stripThink(choice.Message.Content)
			if len(steps) > 0 && strings.TrimSpace(answer) == "" {
				answer = steps[len(steps)-1].Result
			}
			return answer, steps, nil
		}

		for _, tc := range choice.Message.ToolCalls {
			var args map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			result, err := callTool(ctx, tc.Function.Name, args)
			if err != nil {
				result = "Error calling tool: " + err.Error()
			}
			steps = append(steps, common.ToolStep{ToolName: tc.Function.Name, Arguments: args, Result: result})
			messages = append(messages, llmMessage{Role: "assistant", ToolCalls: []toolCall{tc}})
			messages = append(messages, llmMessage{Role: "tool", Content: result, ToolCallID: tc.ID})
		}
	}

	messages = append(messages, llmMessage{Role: "user", Content: "Summarize all gathered information into a final answer."})
	resp, err := m.callLLM(ctx, messages, nil)
	if err != nil {
		var combined string
		for _, s := range steps {
			combined += s.Result + "\n"
		}
		return combined, steps, nil
	}
	return m.stripThink(resp.Choices[0].Message.Content), steps, nil
}
