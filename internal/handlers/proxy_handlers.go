package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"clawdock/internal/models"
)

// OpenAIRequest for /v1/chat/completions
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAICompletionResponse matches OpenAI format.
type OpenAICompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// OpenAIModelInfo for /v1/models list.
type OpenAIModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIModelList struct {
	Object string            `json:"object"`
	Data   []OpenAIModelInfo `json:"data"`
}

// ListOpenAICompatibleModels GET /v1/models
func (h *Handler) ListOpenAICompatibleModels(w http.ResponseWriter, r *http.Request) {
	providersList, err := h.providerRegistry.ListAllProviders()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var models []OpenAIModelInfo
	now := time.Now().Unix()

	for _, p := range providersList {
		if !p.Enabled {
			continue
		}
		pModels, _ := h.providerRegistry.ListModelsForProvider(p.ID)
		for _, m := range pModels {
			if m.Enabled {
				models = append(models, OpenAIModelInfo{
					ID:      m.ModelKey,
					Object:  "model",
					Created: now,
					OwnedBy: p.ID,
				})
			}
		}
	}

	customs, _ := h.providerRegistry.ListAllCustomModels()
	for _, c := range customs {
		if c.Enabled {
			models = append(models, OpenAIModelInfo{
				ID:      c.ID,
				Object:  "model",
				Created: now,
				OwnedBy: c.TargetProviderID,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(OpenAIModelList{Object: "list", Data: models})
}

// ChatCompletions POST /v1/chat/completions
func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	// Check proxy enabled
	enabled, _ := h.providerRegistry.IsChatProxyEnabled()
	if !enabled {
		http.Error(w, "chat proxy disabled", http.StatusServiceUnavailable)
		return
	}

	var req OpenAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		if defaultModel, _ := h.providerRegistry.GetDefaultModel(); defaultModel != "" {
			req.Model = defaultModel
		} else {
			http.Error(w, "model required", http.StatusBadRequest)
			return
		}
	}

	provider, model, err := h.providerRegistry.ResolveModel(req.Model)
	if err != nil {
		http.Error(w, fmt.Sprintf("model error: %v", err), http.StatusBadRequest)
		return
	}

	// Prepare authentication
	var apiKey string
	if provider.AuthType == "api_key" || provider.AuthType == "bearer" {
		apiKey, err = h.providerRegistry.DecryptProviderKey(provider.APIKeyEncrypted)
		if err != nil {
			http.Error(w, "cannot decrypt provider key", http.StatusInternalServerError)
			return
		}
	}

	// Build upstream request
	upstreamURL, payload := buildUpstreamRequest(provider, model, req)
	bodyBytes, _ := json.Marshal(payload)
	upstreamReq, err := http.NewRequest("POST", upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, "upstream request construction failed", http.StatusInternalServerError)
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if provider.ID == "anthropic" && apiKey != "" {
		upstreamReq.Header.Set("x-api-key", apiKey)
	}

	// Hard timeout to prevent hanging connections on upstream providers
	client := &http.Client{
		Timeout: 120 * time.Second,
	}
	upstreamResp, err := client.Do(upstreamReq)
	if err != nil {
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer upstreamResp.Body.Close()

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		switch provider.ID {
		case "ollama":
			translateOllamaStream(upstreamResp.Body, w, model.ModelKey)
		default:
			io.Copy(w, upstreamResp.Body)
		}
		return
	}

	// Non-streaming
	if provider.ID == "anthropic" {
		translateAnthropicResponse(upstreamResp.Body, w, model.ModelKey)
		return
	}
	w.WriteHeader(upstreamResp.StatusCode)
	io.Copy(w, upstreamResp.Body)
}

// buildUpstreamRequest returns URL and JSON payload for the provider.
func buildUpstreamRequest(provider *models.Provider, model *models.ProviderModel, req OpenAIRequest) (string, map[string]interface{}) {
	base := *provider.BaseURL
	payload := map[string]interface{}{
		"model":    model.ModelKey,
		"messages": req.Messages,
		"stream":   req.Stream,
	}
	if req.Temperature != 0 {
		payload["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	switch provider.ID {
	case "ollama":
		return base + "/api/chat", payload
	case "openrouter", "openai":
		return base + "/v1/chat/completions", payload
	case "anthropic":
		return base + "/v1/messages", payload
	default:
		return base + "/v1/chat/completions", payload
	}
}

// translateAnthropicResponse converts Anthropic format to OpenAI.
func translateAnthropicResponse(body io.Reader, w io.Writer, modelKey string) {
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Model      string `json:"model"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		fmt.Fprintf(w, `{"error":"parse error"}`)
		return
	}
	text := ""
	for _, c := range resp.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	out := map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   modelKey,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": convertAnthropicStopReason(resp.StopReason),
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     resp.Usage.InputTokens,
			"completion_tokens": resp.Usage.OutputTokens,
			"total_tokens":      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
	json.NewEncoder(w).Encode(out)
}

// translateOllamaStream converts NDJSON Ollama stream to SSE OpenAI format.
func translateOllamaStream(body io.Reader, w io.Writer, modelKey string) {
	decoder := json.NewDecoder(body)
	for {
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		openAIChunk := map[string]interface{}{
			"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   modelKey,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]string{
						"role":    "assistant",
						"content": chunk.Message.Content,
					},
					"finish_reason": nil,
				},
			},
		}
		if chunk.Done {
			openAIChunk["choices"].([]map[string]interface{})[0]["finish_reason"] = "stop"
		}
		data, _ := json.Marshal(openAIChunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		if chunk.Done {
			fmt.Fprint(w, "data: [DONE]\n\n")
			break
		}
	}
}

func convertAnthropicStopReason(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	default:
		return ""
	}
}
