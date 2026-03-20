package chezmoi

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"lazychezmoi/internal/model"
)

type Client struct {
	Binary      string
	Source      string
	Destination string
	Exclude     []string
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

func (c *Client) appendSourceDest(args *[]string) {
	if c.Source != "" {
		*args = append(*args, "--source", c.Source)
	}
	if c.Destination != "" {
		*args = append(*args, "--destination", c.Destination)
	}
}

func (c *Client) run(args ...string) ([]byte, error) {
	cmd := exec.Command(c.Binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return stdout.Bytes(), nil
}
