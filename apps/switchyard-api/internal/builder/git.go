package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/sirupsen/logrus"
)

type GitService struct {
	workDir string
}

func NewGitService(workDir string) *GitService {
	return &GitService{
		workDir: workDir,
	}
}

type CloneResult struct {
	Path      string
	GitSHA    string
	Success   bool
	Error     error
	CleanupFn func() error
}

// CloneRepository clones a git repository at a specific commit SHA
func (g *GitService) CloneRepository(ctx context.Context, repoURL, gitSHA string) *CloneResult {
	result := &CloneResult{
		GitSHA: gitSHA,
	}

	// Create a temporary directory for the clone
	cloneDir := filepath.Join(g.workDir, fmt.Sprintf("build-%s", gitSHA[:7]))

	// Ensure work directory exists
	if err := os.MkdirAll(g.workDir, 0755); err != nil {
		result.Error = fmt.Errorf("failed to create work directory: %w", err)
		return result
	}

	logrus.Infof("Cloning repository %s at SHA %s", repoURL, gitSHA[:7])

	// Clone the repository
	repo, err := git.PlainCloneContext(ctx, cloneDir, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: nil, // We could add progress reporting here
	})
	if err != nil {
		result.Error = fmt.Errorf("failed to clone repository: %w", err)
		return result
	}

	// Get the worktree
	worktree, err := repo.Worktree()
	if err != nil {
		result.Error = fmt.Errorf("failed to get worktree: %w", err)
		return result
	}

	// Checkout the specific commit
	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(gitSHA),
	})
	if err != nil {
		// Try to resolve as a ref (branch/tag) if direct hash fails
		ref, refErr := repo.Reference(plumbing.ReferenceName(gitSHA), true)
		if refErr == nil {
			err = worktree.Checkout(&git.CheckoutOptions{
				Hash: ref.Hash(),
			})
		}

		if err != nil {
			result.Error = fmt.Errorf("failed to checkout commit %s: %w", gitSHA, err)
			return result
		}
	}

	logrus.Infof("Successfully cloned repository to %s", cloneDir)

	result.Path = cloneDir
	result.Success = true

	// Provide cleanup function
	result.CleanupFn = func() error {
		logrus.Infof("Cleaning up clone directory: %s", cloneDir)
		return os.RemoveAll(cloneDir)
	}

	return result
}

// CloneShallow performs a shallow clone (faster, but only gets the specific commit)
func (g *GitService) CloneShallow(ctx context.Context, repoURL, gitSHA string) *CloneResult {
	result := &CloneResult{
		GitSHA: gitSHA,
	}

	cloneDir := filepath.Join(g.workDir, fmt.Sprintf("build-%s", gitSHA[:7]))

	if err := os.MkdirAll(g.workDir, 0755); err != nil {
		result.Error = fmt.Errorf("failed to create work directory: %w", err)
		return result
	}

	logrus.Infof("Shallow cloning repository %s at SHA %s", repoURL, gitSHA[:7])

	// Shallow clone with depth 1
	_, err := git.PlainCloneContext(ctx, cloneDir, false, &git.CloneOptions{
		URL:           repoURL,
		Depth:         1,
		SingleBranch:  true,
		ReferenceName: plumbing.NewHash(gitSHA).String(),
		Progress:      nil,
	})

	if err != nil {
		// Fallback to full clone if shallow fails
		logrus.Warnf("Shallow clone failed, falling back to full clone: %v", err)
		return g.CloneRepository(ctx, repoURL, gitSHA)
	}

	logrus.Infof("Successfully shallow cloned to %s", cloneDir)

	result.Path = cloneDir
	result.Success = true
	result.CleanupFn = func() error {
		logrus.Infof("Cleaning up clone directory: %s", cloneDir)
		return os.RemoveAll(cloneDir)
	}

	return result
}

// ValidateRepository checks if a repository URL is accessible
func (g *GitService) ValidateRepository(ctx context.Context, repoURL string) error {
	// Try to list remote references
	_, err := git.ListRemotes(&git.ListRemotesOptions{
		URL: repoURL,
	})

	if err != nil {
		return fmt.Errorf("repository not accessible: %w", err)
	}

	return nil
}
