package diff

import (
	"strings"
	"testing"
)

func TestComputeModified(t *testing.T) {
	from := []byte("line1\nline2\nline3\n")
	to := []byte("line1\nline2 modified\nline3\n")
	result := Compute("source", from, "target", to)
	if !strings.Contains(result, "-line2") {
		t.Errorf("expected diff to contain removed line, got:\n%s", result)
	}
	if !strings.Contains(result, "+line2 modified") {
		t.Errorf("expected diff to contain added line, got:\n%s", result)
	}
}

func TestComputeIdentical(t *testing.T) {
	content := []byte("same content\n")
	result := Compute("a", content, "b", content)
	if result != "(no differences)\n" {
		t.Errorf("expected no differences, got: %q", result)
	}
}

func TestComputeBinary(t *testing.T) {
	binary := []byte{0x00, 0x01, 0x02, 0x03}
	result := Compute("a", binary, "b", binary)
	if !strings.Contains(result, "Binary") {
		t.Errorf("expected binary notice, got: %q", result)
	}
}
