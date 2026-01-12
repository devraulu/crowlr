package process

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type extractionResult struct {
	Outlinks []string
	Title    string
}

func ExtractLinks(body io.Reader, baseURL string) (*extractionResult, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, err
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	if newBaseStr := findBase(doc); newBaseStr != "" {
		if newBase, err := base.Parse(newBaseStr); err == nil {
			base = newBase
		}
	}

	links := extractAndResolve(doc, base)
	title := extractTitle(doc)

	return &extractionResult{
		Outlinks: links,
		Title:    title,
	}, nil
}

func findBase(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "base" {
		for _, attr := range n.Attr {
			if attr.Key == "href" {
				return attr.Val
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if res := findBase(c); res != "" {
			return res
		}
	}
	return ""
}

func extractAndResolve(n *html.Node, base *url.URL) []string {
	var links []string
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key == "href" {
				val := strings.TrimSpace(attr.Val)
				if val == "" {
					continue
				}

				resolved := resolve(val, base)
				if resolved != "" {
					links = append(links, resolved)
				}
				break
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		links = append(links, extractAndResolve(c, base)...)
	}
	return links
}

func resolve(ref string, base *url.URL) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}

	abs := base.ResolveReference(u)

	scheme := strings.ToLower(abs.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}

	return abs.String()
}

func extractTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		if n.FirstChild != nil {
			return n.FirstChild.Data
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := extractTitle(c); t != "" {
			return t
		}
	}
	return ""
}
