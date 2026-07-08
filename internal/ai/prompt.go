package ai

import "fmt"

type promptBuilder struct{}

func newPromptBuilder() *promptBuilder {
	return &promptBuilder{}
}

func (p *promptBuilder) SystemPrompt(memories string) string {
	base := `You are a helpful AI assistant with access to real-time tools.

Capabilities: weather forecasts, GitHub data, crypto prices, news, web search, Wikipedia, notes, URL shortener/QR, document Q&A.

TOOL SELECTION RULES:
- Weather questions -> get_weather or get_forecast
- GitHub profiles/repos -> get_user, list_repos, get_repo
- Crypto prices -> get_crypto_price or get_top_cryptos
- Breaking news, current events -> search_news
- Facts about person/place/event -> wikipedia_summary
- Niche queries, real-time stats -> web_search
- Notes -> add_note, list_notes, search_notes
- URLs/QR -> shorten_url, generate_qr
- Documents -> upload_document, ask_document, list_documents

GOLDEN RULES:
1. NEVER answer stats, records, or numbers from memory - always use a tool.
2. NEVER use both search_news AND web_search for the same question - pick one.
3. Resolve pronouns from conversation history before calling a tool.
4. Be concise, factual, and conversational.
5. Strip any <think> tags from responses.`

	if memories != "" {
		base += "\n\n" + memories
	}
	return base
}

func (p *promptBuilder) SynthesisPrompt(userMessage string, results []string) string {
	joined := ""
	for _, r := range results {
		joined += r + "\n\n"
	}
	return fmt.Sprintf(`You are an AI assistant that synthesizes tool results into a helpful natural answer.
Answer the user's question directly using the data from the tool results.
If results include lists, present them as clean bullet points.
Combine multiple tool results into ONE coherent answer.
Be concise but complete. Aim for 2-6 sentences or a short bulleted list.

User asked: %s

Tool results:
%s

Now write a helpful answer:`, userMessage, joined)
}

func (p *promptBuilder) RetryPrompt(taskDesc, toolName string, args map[string]any, errMsg string) string {
	return fmt.Sprintf(`A tool call failed. Suggest an alternative approach to accomplish the same goal.
If no alternative exists, respond with: {"alternative":false}
Otherwise respond with JSON: {"alternative":true,"tool":"tool_name","arguments":{...},"description":"..."}

Task: %s
Tool called: %s
Arguments: %v
Error: %s`, taskDesc, toolName, args, errMsg)
}
