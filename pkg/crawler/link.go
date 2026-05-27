package crawler

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/purell"
)

type Link struct {
	Original   string
	Referrer   string
	Normalized string
	Host       string
}

type Option func(*Link)

func WithReferrer(referrer string) Option {
	return func(l *Link) {
		l.Referrer = referrer
	}
}

func NewLink(original string, opts ...Option) (Link, error) {
	l := &Link{
		Original: original,
	}

	for _, opt := range opts {
		opt(l)
	}

	normalized, err := normalizeURL(original)
	if err != nil {
		return Link{}, err
	}
	l.Normalized = normalized

	host, err := getHost(normalized)
	if err != nil {
		return Link{}, err
	}
	l.Host = host

	return *l, nil
}

func normalizeURL(url string) (string, error) {
	flags := purell.FlagLowercaseScheme |
		purell.FlagLowercaseHost |
		purell.FlagRemoveDefaultPort |
		purell.FlagRemoveFragment |
		purell.FlagDecodeUnnecessaryEscapes |
		purell.FlagSortQuery |
		purell.FlagRemoveDuplicateSlashes |
		purell.FlagRemoveDotSegments

	return purell.NormalizeURLString(url, flags)
}

func getHost(str string) (string, error) {
	u, err := url.Parse(str)
	if err != nil {
		return "", err
	}
	return strings.ToLower(u.Hostname()), nil
}

func (l Link) String() string {
	return l.Normalized
}
