package crawler

import "testing"

func TestNewLink(t *testing.T) {
	l, _ := NewLink("http://example.com/a", WithReferrer("http://example.com/"))

	testCases := []struct {
		desc string
		got  string
		want string
	}{
		{
			desc: "original",
			got:  l.Original,
			want: "http://example.com/a",
		},
		{
			desc: "referrer",
			got:  l.Referrer,
			want: "http://example.com/",
		},
		{
			desc: "normalized",
			got:  l.Normalized,
			want: "http://example.com/a",
		},
		{
			desc: "host",
			got:  l.Host,
			want: "example.com",
		},
	}
	for _, c := range testCases {
		t.Run(c.desc, func(t *testing.T) {
			if c.got != c.want {
				t.Fatalf("got %v, but wanted %v", c.got, c.want)
			}
		})
	}
}

func TestNewLinkShouldErrorOnBadURL(t *testing.T) {
	_, err := NewLink("not an url :(")

	if err == nil {
		t.Fatalf("got no error, but wanted one")
	}
}

func TestLinkShouldNormalizeLinks(t *testing.T) {
	testCases := []struct {
		desc  string
		input string
		want  string
	}{
		{
			desc:  "Remove default port",
			input: "http://example.com:80/",
			want:  "http://example.com/",
		},

		{
			desc:  "Remove dot-segments",
			input: "http://example.com/foo/./bar/baz/../qux",
			want:  "http://example.com/foo/bar/qux",
		},

		{
			desc:  "Decode unreserved characters",
			input: "http://example.com/%7Efoo",
			want:  "http://example.com/~foo",
		},

		{
			desc:  "Scheme and host to lowercase",
			input: "HTTP://User@Example.COM/Foo",
			want:  "http://User@example.com/Foo",
		},
	}

	for _, c := range testCases {
		t.Run(c.desc, func(t *testing.T) {
			f, err := NewLink(c.input)
			if err != nil {
				t.Fatalf("got error %v", err)
			}

			got := f.Normalized
			if got != c.want {
				t.Fatalf("got %v but wanted %v", got, c.want)
			}
		})
	}
}
