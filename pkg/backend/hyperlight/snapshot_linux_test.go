//go:build linux

package hyperlight

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Azure/fiberd/pkg/artifact"
	"github.com/Azure/fiberd/pkg/backend"
)

// TestTemplateSnapshot checks that a usable helper says it restores
// Hyperlight snapshots and an unusable one none, and that a template's
// snapshot reaches the helper as --template-dir, after the helper's own
// arguments and before the template's.
func TestTemplateSnapshot(t *testing.T) {
	t.Run("the format a helper restores", func(t *testing.T) {
		if got := newFake(t, Options{Helper: "helper"}, says(kvmIntel)).ImagesFormat(); got != artifact.ImagesFormatHyperlight {
			t.Errorf("a usable helper restores %q", got)
		}
		if got := newFake(t, Options{Helper: "helper"}, &fakeStarter{versionErr: errors.New("exit status 1")}).ImagesFormat(); got != "" {
			t.Errorf("an unusable helper restores %q", got)
		}
	})
	t.Run("the helper is given the snapshot", func(t *testing.T) {
		snap := t.TempDir()
		engine := filepath.Join(t.TempDir(), "engine.sock")
		s := says(kvmIntel)
		be := newFake(t, Options{Helper: "helper"}, s)
		sp := warmSpec(filepath.Join(t.TempDir(), "run", "g1"))
		sp.EnginePath = engine
		sp.Template = backend.Template{Digest: "sha256:t", Argv: []string{"", "--port-range", "1-2"}, SnapshotDir: snap}
		if _, err := be.Warm(warmCtx(t), sp); err != nil {
			t.Fatalf("Warm: %v", err)
		}
		want := []string{"--engine", engine, "--template-dir", snap, "--port-range", "1-2"}
		if got := s.started()[0].args; !reflect.DeepEqual(got, want) {
			t.Fatalf("helper args = %q, want %q", got, want)
		}
	})
}

// TestTemplateArgsNeverNameTheSnapshot checks that a template's own
// arguments, which its author writes, cannot pass --template-dir, --guest
// or --engine, in any spelling a flag parser takes, so no helper takes
// what the author named in place of what fiberd gave it.
func TestTemplateArgsNeverNameTheSnapshot(t *testing.T) {
	for _, a := range []string{"--template-dir", "-template-dir", "--template-dir=/etc", "-template-dir=/x", "--guest", "-guest=/etc/passwd", "--engine", "--engine=/run/other.sock",
		"--templ=/x", "--gu", "--eng=/run/other.sock"} {
		t.Run(a, func(t *testing.T) {
			s := says(kvmIntel)
			be := newFake(t, Options{Helper: "helper"}, s)
			sp := warmSpec(filepath.Join(t.TempDir(), "run", "g1"))
			sp.Template = backend.Template{Digest: "sha256:t", Argv: []string{"", a, "/elsewhere"}, Dir: "/cache/t", SnapshotDir: t.TempDir()}
			if _, err := be.Warm(warmCtx(t), sp); err == nil || !strings.Contains(err.Error(), "fiberd's alone") {
				t.Fatalf("Warm = %v, want the argument refused", err)
			}
			if n := len(s.started()); n != 0 {
				t.Fatalf("%d helpers started", n)
			}
		})
	}
}

// TestARegistryTemplateNeedsItsSnapshot checks that a registry template
// without a snapshot is refused before a helper starts, since the helper
// would warm its own guest under the template's digest.
func TestARegistryTemplateNeedsItsSnapshot(t *testing.T) {
	s := says(kvmIntel)
	be := newFake(t, Options{Helper: "helper"}, s)
	sp := warmSpec(filepath.Join(t.TempDir(), "run", "g1"))
	sp.Template = backend.Template{Digest: "sha256:t", Argv: []string{"/cache/t/zygote"}, Dir: "/cache/t", ZygoteSHA256: strings.Repeat("a", 64)}
	if _, err := be.Warm(warmCtx(t), sp); err == nil || !strings.Contains(err.Error(), "without a Hyperlight snapshot") {
		t.Fatalf("Warm = %v, want the template refused", err)
	}
	if n := len(s.started()); n != 0 {
		t.Fatalf("%d helpers started", n)
	}
}

// TestAnOperatorsTemplateMayPassHelperFlags checks that the refusal is for
// a registry template's author alone: an operator's own -template command
// passes what it likes.
func TestAnOperatorsTemplateMayPassHelperFlags(t *testing.T) {
	s := says(kvmIntel)
	be := newFake(t, Options{Helper: "helper"}, s)
	sp := warmSpec(filepath.Join(t.TempDir(), "run", "g1"))
	sp.Template = backend.Template{Digest: "sha256:local", Argv: []string{"guest", "--engine", "/run/e.sock"}}
	if _, err := be.Warm(warmCtx(t), sp); err != nil {
		t.Fatalf("Warm: %v", err)
	}
}

func TestHelperFlag(t *testing.T) {
	for arg, want := range map[string]bool{
		"--template-dir": true, "-template-dir=/x": true, "--templ": true, "--te=/x": true, "--guest": true, "---guest": true, "--gu": true, "--engine=/e": true,
		"--g": true, "--e=/e": true, "--t": true, "guest": false, "-e": false, "-gu": false, "--port-range": false, "--heap-mb": false, "--init-ms": false, "-": false, "--": false,
	} {
		if got := HelperFlag(arg); got != want {
			t.Errorf("HelperFlag(%q) = %v, want %v", arg, got, want)
		}
	}
}
