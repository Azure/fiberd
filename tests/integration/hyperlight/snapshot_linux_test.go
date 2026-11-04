//go:build linux && integration

package hyperlighttest

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"

	"github.com/Azure/fiberd/pkg/artifact"
	hlbackend "github.com/Azure/fiberd/pkg/backend/hyperlight"
	"github.com/Azure/fiberd/pkg/core"
	"github.com/Azure/fiberd/pkg/runtime/host"
)

// TestTemplateSnapshotThroughTheHelper runs a template whose artifact
// carries the guest's snapshot end to end: built with the helper's facts
// as its platform, sealed, pushed, pulled by a home, opened for the warm
// as a read-only copy, handed to the helper as --template-dir, served by a
// fiber, and removed once the warm ends.
func TestTemplateSnapshotThroughTheHelper(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)
	jwk, err := artifact.GenerateSealKey()
	if err != nil {
		t.Fatal(err)
	}
	master, _ := artifact.SealKeyFromJWK(jwk)
	wrap, _ := artifact.DeriveWrapKey(master)

	tree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tree, "snapshot", "blobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "metadata.json"), []byte(`{"app":"hello"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "snapshot", "blobs", "aa"), []byte("guest pages"), 0o644); err != nil {
		t.Fatal(err)
	}
	version, err := exec.Command(helperBin, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	p, err := hlbackend.PlatformOf(string(version))
	if err != nil {
		t.Fatal(err)
	}
	p.Arch = runtime.GOARCH
	out := t.TempDir()
	if _, err := artifact.Build(ctx, artifact.BuildOptions{HyperlightSnapshot: tree, Out: out, Args: []string{"--init-ms", "20"}, SealTo: wrap.Public(), Platform: p}); err != nil {
		t.Fatalf("build: %v", err)
	}
	digest, err := artifact.Push(ctx, out, srv.URL+"/tmpl/hl:v1")
	if err != nil {
		t.Fatalf("push: %v", err)
	}

	name := fmt.Sprintf("hs%d", time.Now().UnixNano()%1_000_000)
	cache := filepath.Join(t.TempDir(), "templates")
	rt, err := host.New(host.Config{
		Backend:       hlbackend.New(hlbackend.Options{Helper: helperBin, Guest: guest}),
		Registry:      srv.URL + "/tmpl/hl",
		DeltaKeys:     artifact.Keys{Seal: master},
		CgroupRoot:    filepath.Join(cgRoot, name),
		RunDir:        filepath.Join("/tmp", "fz-"+name),
		DeltaDir:      filepath.Join(t.TempDir(), "deltas"),
		TemplateCache: cache,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rt.Close)
	g := core.Grant{UID: "g1", TemplateDigest: digest, FiberMax: 2, WBudgetBytes: 8 << 20}
	if err := rt.PrepareTemplate(ctx, g); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	snaps, _ := os.ReadDir(filepath.Join(cache, "snapshots"))
	if len(snaps) != 1 {
		t.Fatalf("%d snapshots opened for one warm", len(snaps))
	}
	h, err := rt.Clone(ctx, core.CloneSpec{Grant: g, Fence: core.Fence{GrantUID: "g1", Epoch: 1, Seq: 1}, Deadline: time.Second})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	if got := talk(t, h.Endpoint, "template"); got != `{"app":"hello"}` {
		t.Fatalf("the helper's template = %q, want the snapshot's metadata", got)
	}
	if err := rt.Release(ctx, h.ID, false); err != nil {
		t.Fatalf("release: %v", err)
	}
	rt.DropTemplate("g1")
	if snaps, _ := os.ReadDir(filepath.Join(cache, "snapshots")); len(snaps) != 0 {
		t.Fatalf("%d snapshots left after the warm ended", len(snaps))
	}
}
