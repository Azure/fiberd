package artifact_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/Azure/fiberd/pkg/artifact"
)

// pushTyped pushes the files of dir as a zygote artifact to reg, each
// layer under the media type types names, as a hand-made manifest that
// Pack would never write.
func pushTyped(t *testing.T, reg, dir string, types map[string]string) {
	t.Helper()
	ctx := context.Background()
	fs, err := file.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fs.Close() }()
	var layers []ocispec.Descriptor
	for _, n := range []string{"zygote", "config.json", "images.tar"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			continue
		}
		d, err := fs.Add(ctx, n, types[n], n)
		if err != nil {
			t.Fatal(err)
		}
		layers = append(layers, d)
	}
	desc, err := oras.PackManifest(ctx, fs, oras.PackManifestVersion1_1, artifact.ArtifactType, oras.PackManifestOptions{Layers: layers})
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.Tag(ctx, desc, "t"); err != nil {
		t.Fatal(err)
	}
	repo, err := remote.NewRepository(strings.TrimPrefix(reg, "http://") + "/z")
	if err != nil {
		t.Fatal(err)
	}
	repo.PlainHTTP = true
	if _, err := oras.Copy(ctx, fs, "t", repo, "t", oras.DefaultCopyOptions); err != nil {
		t.Fatal(err)
	}
}

// TestPullHoldsTheImagesTypeToTheFormat checks both ways that an images
// layer's media type and its config's format must agree on pull, so a
// home never opens CRIU images as a snapshot, nor a snapshot as CRIU
// images, from a manifest Pack would not write.
func TestPullHoldsTheImagesTypeToTheFormat(t *testing.T) {
	criu, _, _ := imagesArtifact(t)
	snap, _, _ := hyperlightArtifact(t, snapshotTree(t))
	cases := []struct {
		name  string
		dir   string
		types map[string]string
	}{
		{name: "CRIU images under the Hyperlight media type", dir: criu,
			types: map[string]string{"zygote": artifact.MediaTypeBinary, "config.json": artifact.MediaTypeConfig, "images.tar": artifact.MediaTypeHyperlight}},
		{name: "a Hyperlight snapshot under the CRIU media type", dir: snap,
			types: map[string]string{"config.json": artifact.MediaTypeConfig, "images.tar": artifact.MediaTypeImages}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := newRegistry(t)
			pushTyped(t, reg, tc.dir, tc.types)
			_, err := artifact.Pull(context.Background(), reg+"/z:t", filepath.Join(t.TempDir(), "pulled"))
			if err == nil || !strings.Contains(err.Error(), "its config says format") {
				t.Fatalf("Pull = %v, want the media type and format refused", err)
			}
		})
	}
}
