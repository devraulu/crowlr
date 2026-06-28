package embed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func Batch(baseURL, model string, texts []string) ([][]float32, error) {
	body, _ := json.Marshal(embedRequest{model, texts})

	resp, err := http.Post(baseURL+"/api/embed", "application/json", bytes.NewReader(body))

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned %d", resp.StatusCode)
	}

	var er embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, err
	}

	return er.Embeddings, nil
}

func WithDocPrefix(texts []string) []string {
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = "search_document: " + t
	}

	return out
}

func Query(baseURL, model, query string) ([]float32, error) {
	vectors, err := Batch(baseURL, model, []string{"search_query: " + query})
	if err != nil {
		return nil, err
	}

	if len(vectors) == 0 {
		return nil, fmt.Errorf("no embedding returned for query")
	}

	return vectors[0], nil
}
