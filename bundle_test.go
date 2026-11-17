package familyarchive

import (
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// wantBundleVersion is the release semver this head carries: the release step
// tags the head `v`+wantBundleVersion, so a bundle Version left behind by a
// tag bump fails here instead of shipping a stale identity.
const wantBundleVersion = "0.1.4"

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
