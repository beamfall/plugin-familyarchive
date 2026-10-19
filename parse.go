// Package familyarchive is the Family Archive filename/tag parser hook. It
// implements the scan-facing parser seams via the plugin SDK (github.com/
// beamfall/plugin-sdk) plus the standard library.
//
// Sidecar JSON is optional and lives beside media as the same base name with a
// .json extension. The supported object fields are title, summary,
// captureDate, cameraMake, and cameraModel; legacy fallback keys are limited to
// capture_date, capture-date, date, creation_time, camera_make, and
// camera_model.
//
// Capture dates with an explicit timezone are kept as instants. Capture dates
// without timezone data are treated as local calendar dates, not UTC instants,
// and are pinned to noon UTC after dropping sub-day time. Missing, unreadable,
// or non-JSON sidecars and missing files are silently ignored so one bad local
// asset cannot fail a scan; a sidecar that parses as JSON but has an unexpected
// type on one field only drops that field, not the whole sidecar. GPS,
// face-recognition, and other biometric/location PII are intentionally never
// extracted from sidecars or tags.
package familyarchive

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/beamfall/plugin-sdk"
)

// PluginID and classHomeVideo are the two identifiers this plugin hands to
// core: PluginID must match Bundle().ID (core's name-keyed plugin routing),
// and classHomeVideo must match the MediaClassLabels key in bundle.go (the
// UI's class label lookup). Sharing one const block, pinned together by
// TestPluginIDMatchesBundleID and TestClassHomeVideoMatchesBundleLabel in
// parse_test.go, means a rename of either can't silently break its pairing.
const (
	PluginID        = "familyarchive"
	classHomeVideo  = "home-video"
	maxSidecarBytes = 1 << 20
)

// Parser implements scan.FilenameParser plus typed and legacy Media field seams.
type Parser struct{}

// New returns a ready Parser.
func New() *Parser { return &Parser{} }

type sidecarMetadata struct {
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	CaptureDate string `json:"captureDate"`
	CameraMake  string `json:"cameraMake"`
	CameraModel string `json:"cameraModel"`
}

// Parse derives a Family Archive Media title and class from a sibling sidecar,
// then from local path context. Capture year is filled when a local date parses.
func (p *Parser) Parse(path string, probe sdk.ParseProbe) (sdk.ParsedMedia, bool) {
	sidecar := readSidecar(path)
	title := firstNonEmpty(sidecar.Title, titleFromPath(path))
	if title == "" {
		return sdk.ParsedMedia{}, false
	}
	parsed := sdk.ParsedMedia{Title: title, Class: classHomeVideo}
	if captured, ok := captureDate(path, probe, sidecar); ok {
		parsed.Year = captured.Year()
	}
	return parsed, true
}

// ParseMediaFields derives text-only field hints from local sources.
func (p *Parser) ParseMediaFields(path string, probe sdk.ParseProbe) []sdk.ParsedTextField {
	sidecar := readSidecar(path)
	return parseMediaFields(path, probe, sidecar)
}

// parseMediaFields derives text-only field hints from an already-read sidecar.
func parseMediaFields(path string, probe sdk.ParseProbe, sidecar sidecarMetadata) []sdk.ParsedTextField {
	fields := make([]sdk.ParsedTextField, 0, 5)
	addField := func(name, text string) {
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			fields = append(fields, sdk.ParsedTextField{Name: name, Text: trimmed})
		}
	}

	addField("source-folder", sourceFolder(path))
	addField("sidecar-title", sidecar.Title)
	addField("sidecar-summary", sidecar.Summary)
	addField("camera-make", firstNonEmpty(sidecar.CameraMake, tagValue(probe.Tags, "make", "camera_make", "com.apple.quicktime.make")))
	addField("camera-model", firstNonEmpty(sidecar.CameraModel, tagValue(probe.Tags, "model", "camera_model", "com.apple.quicktime.model")))
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// ParseTypedMediaFields derives typed local hints. Capture dates stay date
// fields, not text fields, so date Sources and sorts can use them.
func (p *Parser) ParseTypedMediaFields(path string, probe sdk.ParseProbe) []sdk.ParsedField {
	sidecar := readSidecar(path)
	fields := make([]sdk.ParsedField, 0, 6)
	for _, field := range parseMediaFields(path, probe, sidecar) {
		fields = append(fields, sdk.ParsedField{Name: field.Name, Text: field.Text})
	}
	if captured, ok := captureDate(path, probe, sidecar); ok {
		fields = append(fields, sdk.ParsedField{Name: "capture-date", Date: captured})
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// readSidecar returns supported metadata from the sibling JSON sidecar.
func readSidecar(path string) sidecarMetadata {
	var metadata sidecarMetadata
	f, err := os.Open(sidecarPath(path))
	if err != nil {
		// Intentional: a missing/unreadable sidecar is the common case (most
		// media has no sidecar at all), not a fault. Callers fall back to
		// path-derived title/date, so an empty metadata is the correct result.
		return metadata
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, maxSidecarBytes+1))
	if err != nil || len(b) > maxSidecarBytes {
		return metadata
	}
	if err := json.Unmarshal(b, &metadata); err != nil {
		// A wrong-typed canonical field fails the strict decode part-way
		// through; discard the partial result and let the per-key pass below
		// re-read every field, so one wrong-typed field drops only that field
		// rather than the whole sidecar.
		metadata = sidecarMetadata{}
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(b, &raw) != nil {
		// Intentional: the sidecar exists but is not valid JSON in either
		// shape we understand. Treated as "no sidecar metadata" rather than
		// a hard failure so one malformed sidecar can't break a scan.
		return metadata
	}
	// Aliases below are deliberately narrow: "description"
	// and bare "make"/"model" are common third-party schema keys (Google/Apple
	// Photos exports, generic camera dumps) unrelated to this plugin's fields,
	// and silently adopting them would absorb unrelated sidecar content into
	// searchable Media fields. Only Family-Archive-shaped keys are accepted.
	// GPS coordinates and face-recognition metadata are intentionally never
	// extracted from the sidecar, in any key shape.
	//
	// rawString reads only fields whose JSON value is itself a string; a
	// field with an unexpected type (e.g. captureDate as a JSON number)
	// fails to decode into a string and is skipped. This pass runs on every
	// sidecar — not only when the strict decode fails — because the strict
	// decode matches none of the legacy keys, so a well-typed sidecar carrying
	// only legacy keys would otherwise silently yield nothing. Canonical
	// camelCase values, when present, always win over their legacy fallbacks.
	rawString := func(key string) string {
		var s string
		if json.Unmarshal(raw[key], &s) != nil {
			return ""
		}
		return s
	}
	metadata.Title = firstNonEmpty(metadata.Title, rawString("title"))
	metadata.Summary = firstNonEmpty(metadata.Summary, rawString("summary"))
	metadata.CaptureDate = firstNonEmpty(metadata.CaptureDate, rawString("captureDate"), rawString("capture_date"), rawString("capture-date"), rawString("date"), rawString("creation_time"))
	metadata.CameraMake = firstNonEmpty(metadata.CameraMake, rawString("cameraMake"), rawString("camera_make"))
	metadata.CameraModel = firstNonEmpty(metadata.CameraModel, rawString("cameraModel"), rawString("camera_model"))
	return metadata
}

// sidecarPath maps a media path to its cleaned sibling JSON sidecar path.
func sidecarPath(path string) string {
	path = filepath.Clean(path)
	ext := filepath.Ext(path)
	if ext == "" {
		return path + ".json"
	}
	return strings.TrimSuffix(path, ext) + ".json"
}

// titleFromPath derives a title from the parent folder, then the file stem.
func titleFromPath(path string) string {
	dir := filepath.Dir(path)
	if dir != "." && dir != "/" {
		if title := cleanName(filepath.Base(dir)); title != "" {
			return title
		}
	}
	return cleanName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

// sourceFolder surfaces the parent directory's name as a filterable Media
// field. This is PII by design, not by accident: home-video
// import folders are commonly named after people, places, or events (e.g.
// "Grandma's 80th", "Sarah_Kindergarten"), and that is the label users expect
// to filter/browse by. Callers/hosts that index this field should treat it as
// personal data, same as sidecar-title/sidecar-summary.
func sourceFolder(path string) string {
	dir := filepath.Dir(path)
	if dir == "." || dir == "/" {
		return ""
	}
	return cleanName(filepath.Base(dir))
}

// captureDate returns the best local capture date from sidecar, tags, or mtime.
func captureDate(path string, probe sdk.ParseProbe, sidecar sidecarMetadata) (time.Time, bool) {
	if date := firstNonEmpty(sidecar.CaptureDate, tagValue(probe.Tags, "creation_time", "date", "datetimeoriginal", "date_time_original", "com.apple.quicktime.creationdate")); date != "" {
		return parseCaptureDate(date)
	}
	info, err := os.Stat(path)
	if err != nil {
		// Intentional: no sidecar/tag date and the file itself can't be
		// stat'd (e.g. transient scan-time race). Fall back to "not found"
		// rather than erroring, matching Parse's tolerance of unresolvable
		// capture dates.
		return time.Time{}, false
	}
	// Filesystem mtime is import/copy time, not true capture time; use it only
	// as a best-effort fallback when no sidecar or embedded date exists.
	return info.ModTime().UTC(), true
}

// tzLayouts carry an explicit zone token: their offset is authoritative, so the
// instant is kept as parsed. noTZLayouts have no zone; time.Parse would silently
// stamp them UTC, shifting the calendar date by the user's offset for a
// Sortable primary browse axis. We instead drop the (untrustworthy) clock time
// and keep only the local calendar date at a fixed UTC hour.
var (
	tzLayouts   = []string{time.RFC3339}
	noTZLayouts = []string{
		"2006-01-02",
		"2006:01:02 15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
)

// parseCaptureDate parses known capture-date layouts and preserves date-only local dates.
func parseCaptureDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range tzLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	for _, layout := range noTZLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return dateOnlyUTC(t), true
		}
	}
	return time.Time{}, false
}

// dateOnlyUTC keeps only the local calendar date the EXIF layout carried,
// discarding the sub-day clock time (honestly dropping ordering we cannot place
// on the timeline without a zone) and pinning it to noon UTC so the date never
// straddles a midnight boundary under later zone conversion.
func dateOnlyUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.UTC)
}

// tagValue returns the first non-empty tag value for the requested names.
func tagValue(tags map[string]string, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(tags[name]); value != "" {
			return value
		}
	}
	return ""
}

// cleanName turns path-ish names into compact user-visible labels.
func cleanName(raw string) string {
	if raw == "" || raw == "." || raw == "/" {
		return ""
	}
	replaced := strings.NewReplacer("_", " ", ".", " ").Replace(raw)
	return strings.Join(strings.Fields(replaced), " ")
}

// firstNonEmpty returns the first non-empty trimmed string.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
