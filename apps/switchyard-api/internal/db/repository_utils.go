package db

import "strings"

// normalizeGitURL normalizes a git repository URL for consistent matching
func normalizeGitURL(url string) string {
	// Remove trailing slashes
	url = strings.TrimSuffix(url, "/")
	// Ensure https:// prefix
	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
	}
	return url
}
