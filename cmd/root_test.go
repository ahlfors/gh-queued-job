package cmd

import (
	"testing"
)

func TestResolveRepoFromFlag(t *testing.T) {
	owner, repo, err := resolveRepo("", "xxx/repo123")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "xxx" || repo != "repo123" {
		t.Fatalf("got %s/%s", owner, repo)
	}
}

func TestResolveRepoInvalid(t *testing.T) {
	if _, _, err := resolveRepo("", "not-a-repo"); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := resolveRepo("xxx", ""); err == nil {
		t.Fatal("expected --org alone to fail")
	}
}

func TestNewRootCmdFlags(t *testing.T) {
	c := newRootCmd("test")
	if c.Use != "gh-queued-job" {
		t.Fatalf("use=%s", c.Use)
	}
	if c.Version != "test" {
		t.Fatalf("version=%s", c.Version)
	}
	for _, name := range []string{"org", "repo", "interval", "json", "once"} {
		if c.Flags().Lookup(name) == nil {
			t.Errorf("missing flag --%s", name)
		}
	}
}
