package familyarchive

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSidecarPathCleansTraversal pins the filepath.Clean defense-in-depth in
// sidecarPath(): a ".."-bearing path must resolve to the cleaned sidecar path,
// not the raw traversal path.
func TestSidecarPathCleansTraversal(t *testing.T) {
	got := sidecarPath("a/b/../c/IMG_0042.mp4")
	want := "a/c/IMG_0042.json"
	if got != want {
		t.Fatalf("sidecarPath traversal: got %q, want %q", got, want)
	}
}

// TestReadSidecarLegacyKeys pins the legacy-key fallback on well-typed sidecar
// JSON: the strict camelCase decode succeeds while matching none of the legacy
// keys (capture_date, capture-date, date, creation_time, camera_make,
// camera_model), so the per-key raw pass must still read them — previously it
// ran only when the strict decode errored, leaving a well-typed legacy-only
// sidecar silently empty. Canonical keys must still win when both are present.
func TestReadSidecarLegacyKeys(t *testing.T) {
	vectors := []struct {
		name    string
		sidecar string
		want    sidecarMetadata
	}{
		{
			name:    "legacy-only keys populate metadata",
			sidecar: `{"creation_time":"2019-05-04","camera_make":"Sony","camera_model":"FDR-AX53"}`,
			want:    sidecarMetadata{CaptureDate: "2019-05-04", CameraMake: "Sony", CameraModel: "FDR-AX53"},
		},
		{
			name:    "canonical keys win over legacy counterparts",
			sidecar: `{"captureDate":"2020-01-02","date":"1999-01-01","cameraMake":"Canon","camera_make":"Sony"}`,
			want:    sidecarMetadata{CaptureDate: "2020-01-02", CameraMake: "Canon"},
		},
		{
			name:    "kebab-case capture-date is read",
			sidecar: `{"capture-date":"2018-07-15"}`,
			want:    sidecarMetadata{CaptureDate: "2018-07-15"},
		},
	}
	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "clip.json"), []byte(v.sidecar), 0o644); err != nil {
				t.Fatalf("write sidecar fixture: %v", err)
			}
			if got := readSidecar(filepath.Join(dir, "clip.mp4")); got != v.want {
				t.Fatalf("readSidecar(...) = %+v, want %+v", got, v.want)
			}
		})
	}
}

func TestReadSidecarIgnoresOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.json")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxSidecarBytes + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readSidecar(filepath.Join(dir, "clip.mp4")); got != (sidecarMetadata{}) {
		t.Fatalf("oversized sidecar metadata = %#v, want empty", got)
	}
}

// TestPluginIDMatchesBundleID pins PluginID against Bundle().ID: core resolves this plugin's parser via PluginID and its
// declarative bundle via Bundle().ID, keyed by the same name. A rename of
// either without the other would silently break that name-keyed routing.
func TestPluginIDMatchesBundleID(t *testing.T) {
	if got := Bundle().ID; got != PluginID {
		t.Fatalf("Bundle().ID = %q, want PluginID %q", got, PluginID)
	}
}

// TestClassHomeVideoMatchesBundleLabel pins classHomeVideo (the Class value
// Parse assigns) against the MediaClassLabels key in Bundle(): the UI looks up a class's label by this key, so a rename of
// either without the other would silently break the class label.
func TestClassHomeVideoMatchesBundleLabel(t *testing.T) {
	if _, ok := Bundle().MediaClassLabels[classHomeVideo]; !ok {
		t.Fatalf("Bundle().MediaClassLabels has no entry for classHomeVideo %q", classHomeVideo)
	}
}
