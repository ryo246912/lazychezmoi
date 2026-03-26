package diff

import (
	"testing"
)

func TestApplyWithConflicts_clean(t *testing.T) {
	src := []byte("line1\nexport EDITOR=vim\nline3\n")
	// diff: rendered == source (no template vars), target changed EDITOR
	rendered := []byte("line1\nexport EDITOR=vim\nline3\n")
	target := []byte("line1\nexport EDITOR=nvim\nline3\n")
	patch := Compute("rendered", rendered, "target", target)

	got, hasConflicts := ApplyWithConflicts(src, patch)
	if hasConflicts {
		t.Errorf("expected no conflicts, got conflicts")
	}
	want := "line1\nexport EDITOR=nvim\nline3\n"
	if string(got) != want {
		t.Errorf("want %q, got %q", want, string(got))
	}
}

func TestApplyWithConflicts_conflict(t *testing.T) {
	// source has template variable, rendered has substituted value
	src := []byte("export PATH={{ .homeDir }}/bin:$PATH\nexport EDITOR=vim\n")
	rendered := []byte("export PATH=/home/user/bin:$PATH\nexport EDITOR=vim\n")
	target := []byte("export PATH=/home/user/bin:/opt/foo/bin:$PATH\nexport EDITOR=vim\n")
	patch := Compute("rendered", rendered, "target", target)

	got, hasConflicts := ApplyWithConflicts(src, patch)
	if !hasConflicts {
		t.Errorf("expected conflicts, got none")
	}
	// The conflicting line should be wrapped with markers
	gotStr := string(got)
	if !contains(gotStr, "<<<<<<< source (template)") {
		t.Errorf("expected conflict marker in output:\n%s", gotStr)
	}
	if !contains(gotStr, "{{ .homeDir }}") {
		t.Errorf("expected template syntax preserved in output:\n%s", gotStr)
	}
	if !contains(gotStr, "/opt/foo/bin") {
		t.Errorf("expected target content in output:\n%s", gotStr)
	}
	// Non-conflicting line should be unchanged
	if !contains(gotStr, "export EDITOR=vim") {
		t.Errorf("expected unchanged line preserved:\n%s", gotStr)
	}
}

func TestApplyWithConflicts_mixed(t *testing.T) {
	// Some lines conflict, some apply cleanly
	src := []byte("export PATH={{ .homeDir }}/bin:$PATH\nexport EDITOR=vim\nexport FOO=bar\n")
	rendered := []byte("export PATH=/home/user/bin:$PATH\nexport EDITOR=vim\nexport FOO=bar\n")
	target := []byte("export PATH=/home/user/bin:/opt/foo/bin:$PATH\nexport EDITOR=nvim\nexport FOO=bar\n")
	patch := Compute("rendered", rendered, "target", target)

	got, hasConflicts := ApplyWithConflicts(src, patch)
	if !hasConflicts {
		t.Errorf("expected conflicts, got none")
	}
	gotStr := string(got)
	// PATH line conflicts (template var)
	if !contains(gotStr, "<<<<<<< source (template)") {
		t.Errorf("expected conflict marker:\n%s", gotStr)
	}
	// EDITOR line applies cleanly
	if !contains(gotStr, "export EDITOR=nvim") {
		t.Errorf("expected clean EDITOR change applied:\n%s", gotStr)
	}
}

func TestApplyWithConflicts_noDiff(t *testing.T) {
	src := []byte("no changes\n")
	got, hasConflicts := ApplyWithConflicts(src, "(no differences)\n")
	if hasConflicts {
		t.Errorf("expected no conflicts")
	}
	if string(got) != string(src) {
		t.Errorf("expected src unchanged")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
