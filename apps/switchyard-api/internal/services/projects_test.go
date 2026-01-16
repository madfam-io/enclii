package services

import (
	"testing"
)

// Unit tests for pure functions (no DB required)

func Test_isValidSlug(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"test-project", true},
		{"my-app", true},
		{"app123", true},
		{"123app", true},
		{"test-app-123", true},
		{"Test-Project", false}, // uppercase
		{"test_project", false}, // underscore
		{"test project", false}, // space
		{"-test", false},        // starts with hyphen
		{"test-", false},        // ends with hyphen
		{"", false},
		{"ab", true}, // minimum length at boundary
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			if got := isValidSlug(tt.slug); got != tt.want {
				t.Errorf("isValidSlug(%q) = %v, want %v", tt.slug, got, tt.want)
			}
		})
	}
}

func Test_isValidGitRepo(t *testing.T) {
	tests := []struct {
		repo string
		want bool
	}{
		{"https://github.com/user/repo", true},
		{"http://gitlab.com/user/repo", true},
		{"git@github.com:user/repo.git", true},
		{"not-a-git-url", false},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.repo, func(t *testing.T) {
			if got := isValidGitRepo(tt.repo); got != tt.want {
				t.Errorf("isValidGitRepo(%q) = %v, want %v", tt.repo, got, tt.want)
			}
		})
	}
}

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"Test Project", "test-project"},
		{"My App", "my-app"},
		{"TEST-APP", "test-app"},
		{"App@123", "app123"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"---test---", "test"},
		{"a", "a-"},   // Will append timestamp
		{"ab", "ab-"}, // Will append timestamp
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug := GenerateSlug(tt.name)

			// For very short names, just check it's not empty and longer than input
			if len(tt.name) < 3 {
				if slug == "" {
					t.Error("GenerateSlug() returned empty string")
				}
				if len(slug) <= len(tt.name) {
					t.Errorf("GenerateSlug() should append timestamp for short names")
				}
				return
			}

			if slug != tt.expected {
				t.Errorf("GenerateSlug(%q) = %v, want %v", tt.name, slug, tt.expected)
			}
		})
	}
}
