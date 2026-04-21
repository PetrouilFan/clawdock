package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Discoverer defines the interface for discovering models from a provider.
type Discoverer interface {
	Discover() ([]string, error) // returns list of model keys
	HealthCheck() error
}

// OllamaDiscoverer implements discovery for Ollama API.
type OllamaDiscoverer struct {
	BaseURL string
}

func (d *OllamaDiscoverer) Discover() ([]string, error) {
	url := fmt.Sprintf("%s/api/tags", d.BaseURL)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	keys := make([]string, len(body.Models))
	for i, m := range body.Models {
		keys[i] = m.Name
	}
	return keys, nil
}

func (d *OllamaDiscoverer) HealthCheck() error {
	url := fmt.Sprintf("%s/api/tags", d.BaseURL)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// OpenAICompatibleDiscoverer works for OpenAI, Anthropic, Google style endpoints.
type OpenAICompatibleDiscoverer struct {
	BaseURL string
	APIKey  string
}

func (d *OpenAICompatibleDiscoverer) Discover() ([]string, error) {
	url := fmt.Sprintf("%s/v1/models", d.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+d.APIKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	keys := make([]string, len(body.Data))
	for i, m := range body.Data {
		keys[i] = m.ID
	}
	return keys, nil
}

func (d *OpenAICompatibleDiscoverer) HealthCheck() error {
	url := fmt.Sprintf("%s/v1/models", d.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+d.APIKey)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// OpenRouterDiscoverer uses /api/v1/models endpoint.
type OpenRouterDiscoverer struct {
	BaseURL string
	APIKey  string
}

func (d *OpenRouterDiscoverer) Discover() ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/models", d.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+d.APIKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openrouter returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	keys := make([]string, len(body.Data))
	for i, m := range body.Data {
		keys[i] = m.ID
	}
	return keys, nil
}

func (d *OpenRouterDiscoverer) HealthCheck() error {
	url := fmt.Sprintf("%s/api/v1/models", d.BaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+d.APIKey)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// NewDiscoverer creates a Discoverer based on provider auth_type.
// If auth_type is "none", returns OllamaDiscoverer.
// If auth_type is "api_key" returns OpenAICompatibleDiscoverer (unless special case).
// If auth_type is "bearer" returns OpenAICompatibleDiscoverer (OpenRouter uses 'bearer' with different endpoint, handled separately via provider ID? Actually we'll use OpenRouterDiscoverer for openrouter provider).
// We'll map based on provider ID for special cases.
func NewDiscoverer(providerID, baseURL, authType, apiKey string) (Discoverer, error) {
	switch authType {
	case "none":
		return &OllamaDiscoverer{BaseURL: baseURL}, nil
	case "api_key":
		// OpenAI, Anthropic, Google use OpenAI compatible endpoint
		return &OpenAICompatibleDiscoverer{BaseURL: baseURL, APIKey: apiKey}, nil
	case "bearer":
		// OpenRouter specifically uses different endpoint
		if providerID == "openrouter" {
			return &OpenRouterDiscoverer{BaseURL: baseURL, APIKey: apiKey}, nil
		}
		// Others treat bearer same as api_key
		return &OpenAICompatibleDiscoverer{BaseURL: baseURL, APIKey: apiKey}, nil
	default:
		return nil, fmt.Errorf("unsupported auth_type: %s", authType)
	}
}
