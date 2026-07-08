package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/varunbanda/mcp-gateway/servers/internal/mcpcommon"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

var tools = []map[string]any{
	{"name": "get_weather", "description": "Get current real-time weather for any city", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string", "description": "City name"}},
		"required": []string{"city"},
	}},
	{"name": "get_forecast", "description": "Get 3-day weather forecast", "inputSchema": map[string]any{
		"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string", "description": "City name"}},
		"required": []string{"city"},
	}},
}

type wttrData struct {
	CurrentCondition []struct {
		TempC, TempF, Humidity, WindspeedK, FeelsLikeC string
		Desc []struct{ Value string } `json:"weatherDesc"`
	} `json:"current_condition"`
	Weather []struct {
		Date                          string `json:"date"`
		MaxTempC string `json:"maxtempC"`
		MinTempC string `json:"mintempC"`
		Hourly                        []struct {
			TempC string `json:"tempC"`
			Desc  []struct{ Value string } `json:"weatherDesc"`
		} `json:"hourly"`
	} `json:"weather"`
}

func Start(port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /mcp/message", handleMCP)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	log.Printf("Weather MCP Server on :%s", port)
	return http.ListenAndServe(":"+port, mux)
}

func handleMCP(w http.ResponseWriter, r *http.Request) {
	enc := json.NewEncoder(w)
	var req mcpcommon.MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mcpcommon.SendError(enc, nil, -32700, "Parse error")
		return
	}
	switch req.Method {
	case "initialize":
		mcpcommon.SendResult(enc, req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "weather-server", "version": "1.0.0"},
		})
	case "tools/list":
		mcpcommon.SendResult(enc, req.ID, map[string]any{"tools": tools})
	case "tools/call":
		handleTool(enc, req)
	default:
		mcpcommon.SendError(enc, req.ID, -32601, "Method not found")
	}
}

func handleTool(enc *json.Encoder, req mcpcommon.MCPRequest) {
	name, _ := req.Params["name"].(string)
	args, _ := req.Params["arguments"].(map[string]any)
	switch name {
	case "get_weather":
		city, _ := args["city"].(string)
		if city == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: city required", true); return }
		r, err := getWeather(city)
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		mcpcommon.SendToolResult(enc, req.ID, r, false)
	case "get_forecast":
		city, _ := args["city"].(string)
		if city == "" { mcpcommon.SendToolResult(enc, req.ID, "Error: city required", true); return }
		r, err := getForecast(city)
		if err != nil { mcpcommon.SendToolResult(enc, req.ID, "Error: "+err.Error(), true); return }
		mcpcommon.SendToolResult(enc, req.ID, r, false)
	default:
		mcpcommon.SendToolResult(enc, req.ID, "Unknown tool: "+name, true)
	}
}

func getWeather(city string) (string, error) {
	resp, err := httpClient.Get(fmt.Sprintf("https://wttr.in/%s?format=j1", url.QueryEscape(city)))
	if err != nil { return "", err }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data wttrData
	if err := json.Unmarshal(body, &data); err != nil || len(data.CurrentCondition) == 0 {
		return "", fmt.Errorf("no data for %q", city)
	}
	c := data.CurrentCondition[0]
	desc := "Unknown"
	if len(c.Desc) > 0 { desc = c.Desc[0].Value }
	return fmt.Sprintf("Weather in %s:\n  Temp: %s°C\n  Condition: %s\n  Humidity: %s%%\n  Wind: %s km/h", city, c.TempC, desc, c.Humidity, c.WindspeedK), nil
}

func getForecast(city string) (string, error) {
	resp, err := httpClient.Get(fmt.Sprintf("https://wttr.in/%s?format=j1", url.QueryEscape(city)))
	if err != nil { return "", err }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data wttrData
	if err := json.Unmarshal(body, &data); err != nil || len(data.Weather) == 0 {
		return "", fmt.Errorf("no forecast for %q", city)
	}
	result := fmt.Sprintf("3-Day Forecast for %s:\n", city)
	for _, day := range data.Weather {
		desc := "Unknown"
		if len(day.Hourly) > 4 && len(day.Hourly[4].Desc) > 0 { desc = day.Hourly[4].Desc[0].Value }
		result += fmt.Sprintf("  %s: %s°C to %s°C — %s\n", day.Date, day.MinTempC, day.MaxTempC, desc)
	}
	return result, nil
}
