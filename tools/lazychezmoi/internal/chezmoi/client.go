package chezmoi

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ryo246912/lazychezmoi/tools/lazychezmoi/internal/model"
)

const defaultTimeout = 10 * time.Second

type Client struct {
	Binary      string
	Source      string
	Destination string
	Exclude     []string
	Timeout     time.Duration // per-command timeout; 0 uses defaultTimeout
}

func New(binary, source, destination string, exclude []string) *Client {
	if binary == "" {
		binary = "chezmoi"
	}
	return &Client{
		Binary:      binary,
		Source:      source,
		Destination: destination,
		Exclude:     exclude,
	}
}

func (c *Client) Status() ([]model.Entry, error) {
	args := []string{"status", "--include", "files", "--path-style", "absolute"}
	if len(c.Exclude) > 0 {
		args = append(args, "--exclude", strings.Join(c.Exclude, ","))
	}
	c.appendSourceDest(&args)
	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("chezmoi status: %w", err)
	}
	return ParseStatus(out), nil
}

func (c *Client) Unmanaged() ([]model.Entry, error) {
	args := []string{"unmanaged", "--path-style", "absolute", "--nul-path-separator"}
	if len(c.Exclude) > 0 {
		args = append(args, "--exclude", strings.Join(c.Exclude, ","))
	}
	c.appendSourceDest(&args)
	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("chezmoi unmanaged: %w", err)
	}
	return ParseUnmanaged(out), nil
}

func (c *Client) SourceDir() (string, error) {
	out, err := c.run("source-path")
	if err != nil {
		return "", fmt.Errorf("chezmoi source-path: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) SourcePath(targetPath string) (string, error) {
	args := []string{"source-path", targetPath}
	if c.Source != "" {
		args = append(args, "--source", c.Source)
	}
	out, err := c.run(args...)
	if err != nil {
		return "", fmt.Errorf("chezmoi source-path: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Client) Cat(targetPath string) ([]byte, error) {
	args := []string{"cat", targetPath}
	c.appendSourceDest(&args)
	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("chezmoi cat: %w", err)
	}
	return out, nil
}

func (c *Client) Apply(targetPath string) error {
	args := []string{"apply", targetPath}
	c.appendSourceDest(&args)
	_, err := c.run(args...)
	if err != nil {
		return fmt.Errorf("chezmoi apply: %w", err)
	}
	return nil
}

func (c *Client) Add(targetPath string) error {
	args := []string{"add", targetPath}
	c.appendSourceDest(&args)
	_, err := c.run(args...)
	if err != nil {
		return fmt.Errorf("chezmoi add: %w", err)
	}
	return nil
}

func (c *Client) WithSource(source string) *Client {
	clone := *c
	clone.Source = source
	return &clone
}

func (c *Client) appendSourceDest(args *[]string) {
	if c.Source != "" {
		*args = append(*args, "--source", c.Source)
	}
	if c.Destination != "" {
		*args = append(*args, "--destination", c.Destination)
	}
}

func (c *Client) run(args ...string) ([]byte, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.Binary, args...)
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
