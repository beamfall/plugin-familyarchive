package familyarchive

import (
	"slices"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// wantBundleVersion is the release semver this head carries: the release step
// tags the head `v`+wantBundleVersion, so a bundle Version left behind by a
// tag bump fails here instead of shipping a stale identity.
const wantBundleVersion = "0.1.4"

// CORE-2364: the scanner uses this declaration for both admission and
// retirement of previously catalogued images (CORE-2361).
func TestBundlePreservesArchivePhotos(t *testing.T) {
	classes := Bundle().MediaClasses
	for _, class := range []string{"video", "image"} {
		if !slices.Contains(classes, class) {
			t.Errorf("archive classes %v exclude %q content", classes, class)
		}
	}
}

// TestBundleVersionMatchesReleaseTag pins Bundle().Version to the semver of
// the release tag applied to this head.
func TestBundleVersionMatchesReleaseTag(t *testing.T) {
	if got := Bundle().Version; got != wantBundleVersion {
		t.Fatalf("Bundle().Version = %q, want %q", got, wantBundleVersion)
	}
}

// TestBundleCompatibleWithPinnedSDK pins the declared [SDKFloor, SDKCeiling]
// range against the SDK identity this module pins, so a bundle whose range
// excludes its own pinned SDK fails here rather than at core load time
// (see sdk.CompatibleWithSDK).
func TestBundleCompatibleWithPinnedSDK(t *testing.T) {
	b := Bundle()
	if !sdk.CompatibleWithSDK(b, sdk.Version) {
		t.Fatalf("Bundle() compat range [%s, %s] excludes pinned SDK %s", b.SDKFloor, b.SDKCeiling, sdk.Version)
	}
}

// TestBundleDisplayName pins the name hosts show for this plugin and the ID
// they key on: the label is "Archive" while the ID stays familyarchive, so a
// rename of one never moves the other.
func TestBundleDisplayName(t *testing.T) {
	b := Bundle()
	if b.ID != "familyarchive" {
		t.Fatalf("Bundle().ID = %q, want %q", b.ID, "familyarchive")
	}
	for locale, want := range map[string]string{"en": "Archive", "fr": "Archives"} {
		if got := b.DisplayName[locale]; got != want {
			t.Errorf("Bundle().DisplayName[%q] = %q, want %q", locale, got, want)
		}
	}
}

// TestBundleMediaLabels pins what hosts call this plugin's media: the media
// label and the home-video class label both read "Video", while the class ID
// stored on parsed media stays home-video.
func TestBundleMediaLabels(t *testing.T) {
	b := Bundle()
	for locale, want := range map[string]string{"en": "Video", "fr": "Vidéo"} {
		if got := b.MediaLabel[locale]; got != want {
			t.Errorf("Bundle().MediaLabel[%q] = %q, want %q", locale, got, want)
		}
		if got := b.MediaClassLabels["home-video"][locale]; got != want {
			t.Errorf("Bundle().MediaClassLabels[home-video][%q] = %q, want %q", locale, got, want)
		}
	}
}
