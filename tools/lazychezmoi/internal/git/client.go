package git

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

type SourceMode int

const (
	SourceModeWorkingTree SourceMode = iota
	SourceModeStaged
	SourceModeHead
)

func (m SourceMode) String() string {
	switch m {
	case SourceModeStaged:
		return "staged"
	case SourceModeHead:
		return "HEAD"
	default:
		return "working tree"
	}
}

func (m SourceMode) RequiresSnapshot() bool {
	return m != SourceModeWorkingTree
}

type Snapshot struct {
	RootDir   string
	SourceDir string
}

type Client struct {
	Binary    string
	SourceDir string
	Timeout   time.Duration
}

func New(binary, sourceDir string) *Client {
	if binary == "" {
		binary = "git"
	}
	return &Client{
		Binary:    binary,
		SourceDir: sourceDir,
	}
}

func (c *Client) Materialize(mode SourceMode) (Snapshot, error) {
	if !mode.RequiresSnapshot() {
		return Snapshot{SourceDir: c.SourceDir}, nil
	}

	repoRoot, relSourceDir, err := c.repoContext()
	if err != nil {
		return Snapshot{}, err
	}

	rootDir, err := os.MkdirTemp("", "lazychezmoi-source-*")
	if err != nil {
		return Snapshot{}, fmt.Errorf("create temp source: %w", err)
	}

	if err := c.writeSnapshot(rootDir, repoRoot, mode); err != nil {
		_ = os.RemoveAll(rootDir)
		return Snapshot{}, err
	}

	sourceDir := rootDir
	if relSourceDir != "." {
		sourceDir = filepath.Join(rootDir, relSourceDir)
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			_ = os.RemoveAll(rootDir)
			return Snapshot{}, fmt.Errorf("prepare source dir: %w", err)
		}
	}

	return Snapshot{
		RootDir:   rootDir,
		SourceDir: sourceDir,
	}, nil
}

func (c *Client) repoContext() (string, string, error) {
	if c.SourceDir == "" {
		return "", "", fmt.Errorf("source directory is empty")
	}

	sourceDir, err := filepath.Abs(c.SourceDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve source directory: %w", err)
	}
	sourceDir, err = filepath.EvalSymlinks(sourceDir)
	if err != nil {
		return "", "", fmt.Errorf("eval source directory: %w", err)
	}

	out, err := c.run(sourceDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("git rev-parse: %w", err)
	}

	repoRoot, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return "", "", fmt.Errorf("eval repo root: %w", err)
	}
	relSourceDir, err := filepath.Rel(repoRoot, sourceDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve repo-relative source dir: %w", err)
	}
	if relSourceDir == ".." || strings.HasPrefix(relSourceDir, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("source directory %s is outside repo root %s", sourceDir, repoRoot)
	}
	return repoRoot, relSourceDir, nil
}

func (c *Client) writeSnapshot(rootDir, repoRoot string, mode SourceMode) error {
	switch mode {
	case SourceModeHead:
		return c.writeHeadSnapshot(rootDir, repoRoot)
	case SourceModeStaged:
		return c.writeIndexSnapshot(rootDir, repoRoot)
	default:
		return nil
	}
}

func (c *Client) writeHeadSnapshot(rootDir, repoRoot string) error {
	archive, err := c.run(repoRoot, "archive", "--format=tar", "HEAD")
	if err != nil {
		return fmt.Errorf("git archive HEAD: %w", err)
	}

	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		targetPath, err := joinSnapshotPath(rootDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create dir %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create parent dir for %s: %w", targetPath, err)
			}
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", targetPath, err)
			}
			if _, err := io.Copy(file, reader); err != nil {
				_ = file.Close()
				return fmt.Errorf("write file %s: %w", targetPath, err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("close file %s: %w", targetPath, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create parent dir for %s: %w", targetPath, err)
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("create symlink %s: %w", targetPath, err)
			}
		default:
			return fmt.Errorf("unsupported tar entry type %v for %s", header.Typeflag, header.Name)
		}
	}
}

func (c *Client) writeIndexSnapshot(rootDir, repoRoot string) error {
	out, err := c.run(repoRoot, "ls-files", "--stage", "-z")
	if err != nil {
		return fmt.Errorf("git ls-files --stage: %w", err)
	}

	for _, item := range bytes.Split(out, []byte{0}) {
		if len(item) == 0 {
			continue
		}

		meta, path, ok := bytes.Cut(item, []byte{'\t'})
		if !ok {
			return fmt.Errorf("invalid ls-files entry: %q", item)
		}

		parts := strings.Fields(string(meta))
		if len(parts) != 3 {
			return fmt.Errorf("invalid ls-files metadata: %q", meta)
		}

		targetPath, err := joinSnapshotPath(rootDir, string(path))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create parent dir for %s: %w", targetPath, err)
		}

		content, err := c.run(repoRoot, "cat-file", "-p", parts[1])
		if err != nil {
			return fmt.Errorf("git cat-file %s: %w", parts[1], err)
		}

		switch parts[0] {
		case "100644":
			if err := os.WriteFile(targetPath, content, 0o644); err != nil {
				return fmt.Errorf("write file %s: %w", targetPath, err)
			}
		case "100755":
			if err := os.WriteFile(targetPath, content, 0o755); err != nil {
				return fmt.Errorf("write executable %s: %w", targetPath, err)
			}
		case "120000":
			if err := os.Symlink(string(content), targetPath); err != nil {
				return fmt.Errorf("write symlink %s: %w", targetPath, err)
			}
		default:
			return fmt.Errorf("unsupported git mode %s for %s", parts[0], path)
		}
	}

	return nil
}

func joinSnapshotPath(rootDir, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid snapshot path %q", name)
	}
	return filepath.Join(rootDir, clean), nil
}

func (c *Client) run(dir string, args ...string) ([]byte, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Binary, append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timed out after %s", timeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return stdout.Bytes(), nil
}
