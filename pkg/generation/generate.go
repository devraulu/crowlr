package generation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func Generate(baseURL, model, prompt string) (string, error) {
	body, _ := json.Marshal(generateRequest{Model: model, Prompt: prompt, Stream: false})
	resp, err := http.Post(baseURL+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned %d", resp.StatusCode)
	}

	var gr generateResponse

	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return "", err
	}

	return gr.Response, nil

}

func GenerateStream(ctx context.Context, baseURL, model, prompt string, onToken func(string)) (string, error) {
	body, _ := json.Marshal(generateRequest{Model: model, Prompt: prompt, Stream: true})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned %d", resp.StatusCode)
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		var chunk generateResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			return full.String(), fmt.Errorf("decode chunk: %w", err)
		}
		if chunk.Response != "" {
			full.WriteString(chunk.Response)
			onToken(chunk.Response)
		}
		if chunk.Done {
			break
		}
	}

	return full.String(), scanner.Err()
}
