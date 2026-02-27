package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"
)

// mockRESTClient is a manual mock for RESTClient.
type mockRESTClient struct {
	ctrl       *gomock.Controller
	getCalls   []string
	patchCalls []struct {
		path string
		body string
	}
	putCalls []struct {
		path string
		body string
	}
	getErr   error
	patchErr error
	putErr   error
}

func newMockRESTClient(ctrl *gomock.Controller) *mockRESTClient {
	return &mockRESTClient{ctrl: ctrl}
}

func (m *mockRESTClient) Get(path string, resp interface{}) error {
	m.getCalls = append(m.getCalls, path)
	return m.getErr
}

func (m *mockRESTClient) Patch(path string, body io.Reader, resp interface{}) error {
	data, _ := io.ReadAll(body)
	m.patchCalls = append(m.patchCalls, struct {
		path string
		body string
	}{path, string(data)})
	return m.patchErr
}

func (m *mockRESTClient) Put(path string, body io.Reader, resp interface{}) error {
	data, _ := io.ReadAll(body)
	m.putCalls = append(m.putCalls, struct {
		path string
		body string
	}{path, string(data)})
	return m.putErr
}

func TestReplaceInFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a file with repo references.
	content := "See cisagov/skeleton-python for details.\nAlso skeleton-python is great.\n"
	filePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := replaceInFiles(dir, "cisagov", "skeleton-python", "myorg", "my-repo"); err != nil {
		t.Fatalf("replaceInFiles failed: %v", err)
	}

	result, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(result)
	if strings.Contains(got, "cisagov/skeleton-python") {
		t.Errorf("org/repo reference not replaced: %s", got)
	}
	if strings.Contains(got, "skeleton-python") {
		t.Errorf("repo reference not replaced: %s", got)
	}
	if !strings.Contains(got, "myorg/my-repo") {
		t.Errorf("expected myorg/my-repo in output: %s", got)
	}
	if !strings.Contains(got, "my-repo") {
		t.Errorf("expected my-repo in output: %s", got)
	}
}

func TestReplaceInFilesSkipsGitDir(t *testing.T) {
	dir := t.TempDir()

	// Create a file in .git that should NOT be replaced.
	gitDir := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitFile := filepath.Join(gitDir, "config")
	content := "url = git@github.com:cisagov/skeleton-python.git\n"
	if err := os.WriteFile(gitFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := replaceInFiles(dir, "cisagov", "skeleton-python", "myorg", "my-repo"); err != nil {
		t.Fatalf("replaceInFiles failed: %v", err)
	}

	result, err := os.ReadFile(gitFile)
	if err != nil {
		t.Fatal(err)
	}
	// The .git file should remain unchanged.
	if string(result) != content {
		t.Errorf(".git file was modified: got %s", string(result))
	}
}

func TestReplaceInFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version.txt")
	if err := os.WriteFile(path, []byte("version: 1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := replaceInFile(path, "1.2.3", "0.0.1"); err != nil {
		t.Fatalf("replaceInFile failed: %v", err)
	}

	result, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "0.0.1") {
		t.Errorf("expected 0.0.1 in file: %s", string(result))
	}
	if strings.Contains(string(result), "1.2.3") {
		t.Errorf("old version still present: %s", string(result))
	}
}
