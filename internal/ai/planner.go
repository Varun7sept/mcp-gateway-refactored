package ai

import "strings"

type planStep struct {
	Description string
	ToolName    string
	Priority    int
}

type planner struct {
	patterns map[string][]planStep
}

func newPlanner() *planner {
	return &planner{
		patterns: map[string][]planStep{
			"weather+news": {
				{Description: "Fetch weather data", ToolName: "get_weather", Priority: 1},
				{Description: "Fetch news headlines", ToolName: "get_top_news", Priority: 2},
			},
			"github+weather": {
				{Description: "Fetch GitHub profile", ToolName: "get_user", Priority: 1},
				{Description: "Fetch weather data", ToolName: "get_weather", Priority: 2},
			},
		},
	}
}

func (p *planner) CreatePlan(userMessage string) []planStep {
	lower := strings.ToLower(userMessage)

	hasWeather := strings.Contains(lower, "weather") || strings.Contains(lower, "temperature") || strings.Contains(lower, "forecast")
	hasNews := strings.Contains(lower, "news") || strings.Contains(lower, "headline") || strings.Contains(lower, "happening")
	hasGitHub := strings.Contains(lower, "github") || strings.Contains(lower, "git") || strings.Contains(lower, "repo")
	hasCrypto := strings.Contains(lower, "bitcoin") || strings.Contains(lower, "crypto") || strings.Contains(lower, "ethereum")
	hasSearch := strings.Contains(lower, "search") || strings.Contains(lower, "find") || strings.Contains(lower, "look up")
	hasWiki := strings.Contains(lower, "wikipedia") || strings.Contains(lower, "who is") || strings.Contains(lower, "what is")

	if hasWeather && hasNews {
		return p.patterns["weather+news"]
	}
	if hasGitHub && hasWeather {
		return p.patterns["github+weather"]
	}
	if hasCrypto && hasSearch {
		return []planStep{
			{Description: "Fetch cryptocurrency price", ToolName: "get_crypto_price", Priority: 1},
			{Description: "Search for additional context", ToolName: "web_search", Priority: 2},
		}
	}
	if hasWiki && hasNews {
		return []planStep{
			{Description: "Get Wikipedia summary", ToolName: "wikipedia_summary", Priority: 1},
			{Description: "Fetch related news", ToolName: "search_news", Priority: 2},
		}
	}

	return nil
}
