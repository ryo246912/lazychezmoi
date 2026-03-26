package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMaterializeUsesRepoRelativeSourceDir(t *testing.T) {
	repo := initGitRepo(t)
	sourceDir := filepath.Join(repo, "src")

	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "dot_zshrc"), []byte("head\n"), 0o644); err != nil {
		t.Fatalf("write head file: %v", err)
	}
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-m", "init")

	if err := os.WriteFile(filepath.Join(sourceDir, "dot_zshrc"), []byte("staged\n"), 0o644); err != nil {
		t.Fatalf("write staged file: %v", err)
	}
	gitRun(t, repo, "add", "src/dot_zshrc")

	if err := os.WriteFile(filepath.Join(sourceDir, "dot_zshrc"), []byte("working-tree\n"), 0o644); err != nil {
		t.Fatalf("write working tree file: %v", err)
	}

	client := New("", sourceDir)

	stagedSnapshot, err := client.Materialize(SourceModeStaged)
	if err != nil {
		t.Fatalf("materialize staged snapshot: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(stagedSnapshot.RootDir) })

	headSnapshot, err := client.Materialize(SourceModeHead)
	if err != nil {
		t.Fatalf("materialize head snapshot: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(headSnapshot.RootDir) })

	stagedData, err := os.ReadFile(filepath.Join(stagedSnapshot.SourceDir, "dot_zshrc"))
	if err != nil {
		t.Fatalf("read staged snapshot: %v", err)
	}
	if string(stagedData) != "staged\n" {
		t.Fatalf("staged snapshot = %q, want staged", stagedData)
	}

	headData, err := os.ReadFile(filepath.Join(headSnapshot.SourceDir, "dot_zshrc"))
	if err != nil {
		t.Fatalf("read head snapshot: %v", err)
	}
	if string(headData) != "head\n" {
		t.Fatalf("head snapshot = %q, want head", headData)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	gitRun(t, repo, "init")
	gitRun(t, repo, "config", "user.name", "Codex")
	gitRun(t, repo, "config", "user.email", "codex@example.com")
	return repo
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
