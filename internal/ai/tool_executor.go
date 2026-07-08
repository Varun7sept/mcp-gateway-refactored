package ai

type toolDef struct {
	Type     string      `json:"type"`
	Function functionDef `json:"function"`
}

type functionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ToolCallResult struct {
	NeedsTool    bool
	ToolName     string
	Arguments    map[string]any
	ToolCallID   string
	DirectAnswer string
}

func (m *Manager) toolDefs() []toolDef {
	return []toolDef{
		makeToolDef("get_weather", "Get real-time weather for any city", map[string]any{
			"city": map[string]any{"type": "string", "description": "City name"},
		}, []string{"city"}),
		makeToolDef("get_forecast", "Get 3-day weather forecast", map[string]any{
			"city": map[string]any{"type": "string", "description": "City name"},
		}, []string{"city"}),
		makeToolDef("get_user", "Get GitHub user profile", map[string]any{
			"username": map[string]any{"type": "string", "description": "GitHub username"},
		}, []string{"username"}),
		makeToolDef("list_repos", "List GitHub user repos by stars", map[string]any{
			"username": map[string]any{"type": "string", "description": "GitHub username"},
		}, []string{"username"}),
		makeToolDef("get_repo", "Get GitHub repo details", map[string]any{
			"owner": map[string]any{"type": "string", "description": "Repo owner"},
			"repo":  map[string]any{"type": "string", "description": "Repo name"},
		}, []string{"owner", "repo"}),
		makeToolDef("add_note", "Save a note to the database", map[string]any{
			"title":   map[string]any{"type": "string", "description": "Note title"},
			"content": map[string]any{"type": "string", "description": "Note content"},
		}, []string{"title", "content"}),
		makeToolDef("list_notes", "List all saved notes", map[string]any{}, nil),
		makeToolDef("search_notes", "Search notes by keyword", map[string]any{
			"query": map[string]any{"type": "string", "description": "Search keyword"},
		}, []string{"query"}),
		makeToolDef("get_crypto_price", "Get live crypto price", map[string]any{
			"coin": map[string]any{"type": "string", "description": "Coin ID e.g. bitcoin"},
		}, []string{"coin"}),
		makeToolDef("get_top_cryptos", "Get top 10 cryptocurrencies", map[string]any{}, nil),
		makeToolDef("get_top_news", "Get top news headlines by topic", map[string]any{
			"topic": map[string]any{"type": "string", "description": "general, technology, business, sports, science, health"},
		}, nil),
		makeToolDef("search_news", "Search news articles", map[string]any{
			"query": map[string]any{"type": "string", "description": "Search keyword"},
		}, []string{"query"}),
		makeToolDef("shorten_url", "Shorten a URL", map[string]any{
			"url": map[string]any{"type": "string", "description": "Full URL to shorten"},
		}, []string{"url"}),
		makeToolDef("generate_qr", "Generate QR code image URL", map[string]any{
			"text": map[string]any{"type": "string", "description": "Text or URL to encode"},
		}, []string{"text"}),
		makeToolDef("web_search", "Search the internet for real-time info", map[string]any{
			"query": map[string]any{"type": "string", "description": "Search query"},
		}, []string{"query"}),
		makeToolDef("wikipedia_summary", "Get Wikipedia summary", map[string]any{
			"topic": map[string]any{"type": "string", "description": "Topic name"},
		}, []string{"topic"}),
		makeToolDef("upload_document", "Upload document to knowledge base", map[string]any{
			"name":    map[string]any{"type": "string", "description": "Document name"},
			"content": map[string]any{"type": "string", "description": "Document text"},
		}, []string{"name", "content"}),
		makeToolDef("ask_document", "Ask questions about uploaded documents", map[string]any{
			"question":      map[string]any{"type": "string", "description": "Your question"},
			"document_name": map[string]any{"type": "string", "description": "Optional specific document"},
		}, []string{"question"}),
		makeToolDef("list_documents", "List all uploaded documents", map[string]any{}, nil),
	}
}

func makeToolDef(name, desc string, properties map[string]any, required []string) toolDef {
	params := map[string]any{
		"type": "object", "properties": properties, "additionalProperties": false,
	}
	if required != nil {
		params["required"] = required
	}
	return toolDef{
		Type: "function",
		Function: functionDef{
			Name: name, Description: desc, Parameters: params,
		},
	}
}
