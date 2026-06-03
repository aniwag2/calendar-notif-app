package handlers

import (
	"net/url"
	"regexp"
	"strings"
)

var slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

// slugify normalizes a facility code to lowercase, hyphen-separated form.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugInvalid.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func urlEncode(s string) string {
	return url.QueryEscape(s)
}
