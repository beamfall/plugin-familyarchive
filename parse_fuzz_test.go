package familyarchive

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	fuzzTZLayouts   = []string{time.RFC3339}
	fuzzNoTZLayouts = []string{
		"2006-01-02",
		"2006:01:02 15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
)

type sidecarFuzzDocument struct {
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	CaptureDate string `json:"captureDate"`
	CameraMake  string `json:"cameraMake"`
	CameraModel string `json:"cameraModel"`
}

func expectedCaptureDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	for _, layout := range fuzzTZLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, true
		}
	}
	for _, layout := range fuzzNoTZLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return time.Date(
				parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, time.UTC,
			), true
		}
	}
	return time.Time{}, false
}

func captureDateOracleMatches(raw string, got time.Time, ok bool) bool {
	want, wantOK := expectedCaptureDate(raw)
	return ok == wantOK && got.Equal(want)
}

func TestCaptureDateFuzzOracleRejectsZeroFalse(t *testing.T) {
	for _, raw := range []string{
		"2020-06-15T09:00:00Z",
		"2019-05-04",
		"2015:12:31 23:30:00",
		"2020-06-15 09:00:00",
		"2020-06-15T09:00:00",
	} {
		if captureDateOracleMatches(raw, time.Time{}, false) {
			t.Fatalf("capture-date oracle accepted planted zero,false for %q", raw)
		}
	}
}

// FuzzParseCaptureDate exercises the tag/sidecar date boundary. Successful
// parses must retain the exact instant for zoned values and normalize unzoned
// values to the declared date-only representation.
func FuzzParseCaptureDate(f *testing.F) {
	for _, seed := range []string{
		"",
		"2020-06-15T09:00:00Z",
		"2020-06-15T09:00:00-04:00",
		"2019-05-04",
		"2015:12:31 23:30:00",
		"2020-06-15 09:00:00",
		"2020-06-15T09:00:00",
		"not-a-date",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		got, ok := parseCaptureDate(raw)
		again, againOK := parseCaptureDate(raw)
		if ok != againOK || !got.Equal(again) {
			t.Fatalf("parseCaptureDate(%q) is not deterministic", raw)
		}
		if !captureDateOracleMatches(raw, got, ok) {
			want, wantOK := expectedCaptureDate(raw)
			t.Fatalf(
				"parseCaptureDate(%q) = (%v, %t), want (%v, %t)",
				raw, got, ok, want, wantOK,
			)
		}
	})
}

func expectedSidecarMetadata(data []byte) (sidecarMetadata, bool) {
	var canonical sidecarFuzzDocument
	if json.Unmarshal(data, &canonical) != nil {
		canonical = sidecarFuzzDocument{}
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return sidecarMetadata{}, false
	}
	rawText := func(key string) string {
		var value string
		_ = json.Unmarshal(raw[key], &value)
		return value
	}
	firstText := func(values ...string) string {
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
		return ""
	}
	legacyText := func(keys ...string) string {
		for _, key := range keys {
			if value := firstText(rawText(key)); value != "" {
				return value
			}
		}
		return ""
	}
	return sidecarMetadata{
		Title:   firstText(canonical.Title, rawText("title")),
		Summary: firstText(canonical.Summary, rawText("summary")),
		CaptureDate: firstText(
			canonical.CaptureDate,
			legacyText("captureDate", "capture_date", "capture-date", "date", "creation_time"),
		),
		CameraMake:  firstText(canonical.CameraMake, legacyText("cameraMake", "camera_make")),
		CameraModel: firstText(canonical.CameraModel, legacyText("cameraModel", "camera_model")),
	}, true
}

func sidecarOracleMatches(data []byte, got sidecarMetadata) bool {
	want, valid := expectedSidecarMetadata(data)
	if !valid {
		want = sidecarMetadata{}
	}
	return got == want
}

func TestSidecarFuzzOracleRejectsZero(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"title":"Family Trip","summary":"Summer"}`),
		[]byte(`{"captureDate":"2020-06-15","cameraMake":"Canon"}`),
		[]byte(`{"creation_time":"2019-05-04","camera_model":"FDR-AX53"}`),
	} {
		want, valid := expectedSidecarMetadata(data)
		if !valid || want == (sidecarMetadata{}) {
			t.Fatalf("sidecar oracle fixture is vacuous: %s", data)
		}
		if sidecarOracleMatches(data, sidecarMetadata{}) {
			t.Fatalf("sidecar oracle accepted planted zero metadata for %s", data)
		}
	}
}

// FuzzReadSidecar drives arbitrary bounded bytes through the actual file-backed
// JSON parser. File I/O is intentional: the production size limit and sibling
// path resolution are part of this untrusted-input boundary.
func FuzzReadSidecar(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"title":"Family Trip","summary":"Summer","captureDate":"2020-06-15",` +
			`"cameraMake":"Canon","cameraModel":"EOS"}`),
		[]byte(`{"title":"Grandma Bday","captureDate":2019}`),
		[]byte(`{"creation_time":"2019-05-04","camera_make":"Sony","camera_model":"FDR-AX53"}`),
		[]byte(`{"gps":{"latitude":43.6},"faces":["person"],"description":"private"}`),
		[]byte(`not-json`),
	} {
		f.Add(seed)
	}

	root := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		// Keep scheduled fuzz iterations I/O-efficient. The max-size boundary
		// itself has a dedicated unit regression at maxSidecarBytes+1.
		if len(data) > 64<<10 {
			return
		}
		dir, err := os.MkdirTemp(root, "case-")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		mediaPath := filepath.Join(dir, "clip.mp4")
		if err := os.WriteFile(filepath.Join(dir, "clip.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		got := readSidecar(mediaPath)
		again := readSidecar(mediaPath)
		if got != again {
			t.Fatalf("readSidecar is not deterministic: first=%#v second=%#v", got, again)
		}
		if !sidecarOracleMatches(data, got) {
			want, _ := expectedSidecarMetadata(data)
			t.Fatalf("readSidecar metadata = %#v, want %#v", got, want)
		}
	})
}
