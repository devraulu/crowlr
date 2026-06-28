package generation

import (
	"fmt"
	"strings"

	"github.com/devraulu/crowlr/pkg/retrieval"
)

func BuildPrompt(query string, chunks []retrieval.RetrievedChunk) string {
	var sb strings.Builder
	sb.WriteString("Answer the question using only the context below. " + "If the context doesn't contain the answer, say you don't know.\n\n")
	for i, c := range chunks {
		fmt.Fprintf(&sb, "[%d] Source: %s\n%s\n\n", i+1, c.PageURL, c.Content)
	}
	fmt.Fprintf(&sb, "Question: %s\n\nAnswer (cite sources using [1], [2], etc.", query)

	return sb.String()
}
