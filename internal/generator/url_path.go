package generator

import (
	"net/url"
	"strings"
)

func siteURLPath(baseURL, rawPath string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "#") || strings.HasPrefix(path, "//") {
		return path
	}

	parsed, err := url.Parse(path)
	if err == nil && (parsed.Scheme != "" || parsed.Host != "") {
		return path
	}

	basePath := assetURLBasePath(baseURL)
	if basePath == "" {
		return ensureLeadingSlash(path)
	}
	return joinURLPath(basePath, path)
}

func ensureLeadingSlash(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}
