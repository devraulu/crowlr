package process

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

func ExtractText(body io.Reader) (string, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	extractTextNodes(doc, &sb)

	text := sb.String()
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text), nil
}

func extractTextNodes(n *html.Node, sb *strings.Builder) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "noscript", "iframe", "svg":
			return
		}
	}

	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		sb.WriteString(" ")
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractTextNodes(c, sb)
	}
}
