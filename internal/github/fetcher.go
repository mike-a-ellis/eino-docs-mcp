package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"strings"

	"github.com/google/go-github/v81/github"
)

// Repository configuration constants
const (
	DefaultOwner    = "cloudwego"
	DefaultRepo     = "cloudwego.github.io"
	DefaultBasePath = "content/en/docs/eino"
)

// FetchedDoc represents a markdown document fetched from GitHub
type FetchedDoc struct {
	Path       string // Relative path within docs directory
	Content    string // Full markdown content
	SHA        string // File's Git blob SHA
	URL        string // GitHub raw URL
}

// Fetcher handles fetching documentation from GitHub repositories
type Fetcher struct {
	client   *Client
	owner    string
	repo     string
	basePath string
}

// NewFetcher creates a new document fetcher
func NewFetcher(client *Client, owner, repo, basePath string) *Fetcher {
	return &Fetcher{
		client:   client,
		owner:    owner,
		repo:     repo,
		basePath: basePath,
	}
}

// ListDocs recursively lists all markdown files in the repository directory
func (f *Fetcher) ListDocs(ctx context.Context) ([]string, error) {
	return f.listDocsRecursive(ctx, f.basePath, "")
}

// listDocsRecursive recursively traverses directories to find all .md files
func (f *Fetcher) listDocsRecursive(ctx context.Context, fullPath, relativePath string) ([]string, error) {
	var docs []string

	// Get directory contents
	_, dirContents, _, err := f.client.Repositories.GetContents(
		ctx,
		f.owner,
		f.repo,
		fullPath,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get contents of %s: %w", fullPath, err)
	}

	// Process each item in the directory
	for _, item := range dirContents {
		if item.Type == nil || item.Name == nil {
			continue
		}

		itemRelPath := path.Join(relativePath, *item.Name)

		switch *item.Type {
		case "file":
			// Only include markdown files
			if strings.HasSuffix(*item.Name, ".md") {
				docs = append(docs, itemRelPath)
			}

		case "dir":
			// Recursively process subdirectories
			itemFullPath := path.Join(fullPath, *item.Name)
			subDocs, err := f.listDocsRecursive(ctx, itemFullPath, itemRelPath)
			if err != nil {
				return nil, err
			}
			docs = append(docs, subDocs...)
		}
	}

	return docs, nil
}

// FetchDoc fetches the content of a specific markdown file
func (f *Fetcher) FetchDoc(ctx context.Context, relativePath string) (*FetchedDoc, error) {
	fullPath := path.Join(f.basePath, relativePath)

	// Get file content from GitHub
	fileContent, _, _, err := f.client.Repositories.GetContents(
		ctx,
		f.owner,
		f.repo,
		fullPath,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get content of %s: %w", fullPath, err)
	}

	if fileContent == nil {
		return nil, fmt.Errorf("no file content returned for %s", fullPath)
	}

	// Decode base64 content
	content, err := base64.StdEncoding.DecodeString(*fileContent.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode content of %s: %w", fullPath, err)
	}

	// Build GitHub raw URL
	rawURL := fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/main/%s",
		f.owner,
		f.repo,
		fullPath,
	)

	return &FetchedDoc{
		Path:    relativePath,
		Content: string(content),
		SHA:     *fileContent.SHA,
		URL:     rawURL,
	}, nil
}

// GetLatestCommitSHA retrieves the SHA of the most recent commit affecting the docs directory
func (f *Fetcher) GetLatestCommitSHA(ctx context.Context) (string, error) {
	commits, _, err := f.client.Repositories.ListCommits(
		ctx,
		f.owner,
		f.repo,
		&github.CommitsListOptions{
			Path: f.basePath,
			ListOptions: github.ListOptions{
				PerPage: 1,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get latest commit: %w", err)
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found for path %s", f.basePath)
	}

	if commits[0].SHA == nil {
		return "", fmt.Errorf("commit SHA is nil")
	}

	return *commits[0].SHA, nil
}
