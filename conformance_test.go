package familyarchive_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"

	familyarchive "github.com/beamfall/plugin-familyarchive"
)

// TestParseConformance pins the Family Archive filename/tag parser against the
// SDK scan contract. A change that shifts any
// of these guesses reds CI before it ships.
func TestParseConformance(t *testing.T) {
	sdktest.RunConformance(t, familyarchive.New(), []sdktest.ParseVector{
		{
			// Title is derived from the parent directory; year from creation_time tag.
			Name: "dir context + creation_time tag",
			Path: "Summer Vacation 2018/IMG_0042.mp4",
			Probe: sdk.ParseProbe{
				Tags: map[string]string{"creation_time": "2018-07-15T10:30:00Z"},
			},
			Want:   sdk.ParsedMedia{Title: "Summer Vacation 2018", Class: "home-video", Year: 2018},
			WantOK: true,
		},
		{
			// No directory context: title from dotted filename stem, no tag → no year.
			Name:   "dotted filename stem, no dir",
			Path:   "Birthday.Party.mp4",
			Want:   sdk.ParsedMedia{Title: "Birthday Party", Class: "home-video"},
			WantOK: true,
		},
		{
			// Underscores in filename are converted to spaces.
			Name:   "underscored filename",
			Path:   "christmas_morning.mov",
			Want:   sdk.ParsedMedia{Title: "christmas morning", Class: "home-video"},
			WantOK: true,
		},
		{
			// Directory takes precedence over filename stem; date tag drives year.
			Name: "dir wins over filename, date tag",
			Path: "Road.Trip.2022/clip_001.mp4",
			Probe: sdk.ParseProbe{
				Tags: map[string]string{"date": "2022-08-01"},
			},
			Want:   sdk.ParsedMedia{Title: "Road Trip 2022", Class: "home-video", Year: 2022},
			WantOK: true,
		},
		{
			// A filename whose stem cleans to empty returns (ParsedMedia{}, false).
			// "_.mp4": stem = "_", cleanName("_") replaces _ with space → " " → Fields → "" → titleFromPath = "".
			// Dir "." is skipped. No sidecar → title = "" → WantOK false.
			Name:   "whitespace-only stem rejects",
			Path:   "_.mp4",
			Want:   sdk.ParsedMedia{},
			WantOK: false,
		},
		{
			// Empty path: filepath.Dir/Base collapse to ".", no stem, no
			// sidecar → (ParsedMedia{}, false), never a panic.
			Name:   "empty path rejects",
			Path:   "",
			Want:   sdk.ParsedMedia{},
			WantOK: false,
		},
		{
			// datetimeoriginal tag (EXIF) also drives year.
			Name: "datetimeoriginal tag drives year",
			Path: "Family/holiday.mp4",
			Probe: sdk.ParseProbe{
				Tags: map[string]string{"datetimeoriginal": "2015:12:25 14:00:00"},
			},
			Want:   sdk.ParsedMedia{Title: "Family", Class: "home-video", Year: 2015},
			WantOK: true,
		},
	})
}

// TestBundleConformance pins the declarative bundle + locale invariants.
func TestBundleConformance(t *testing.T) {
	sdktest.RunBundleConformance(t, familyarchive.Bundle())
}

func TestEntityKindVocabulary(t *testing.T) {
	sdktest.RequireEntityKindVocab(t, familyarchive.Bundle(), []sdktest.KindVocab{
		{Name: "event", Archetype: "container"},
		{Name: "person", Archetype: "person"},
		{Name: "place", Archetype: "category"},
		{Name: "trip", Archetype: "container"},
	})
}

// mediaFieldVector is one golden ParseMediaFields/ParseTypedMediaFields case.
// sdktest has no shared runner for sdk.MediaFieldParser/TypedMediaFieldParser
// (only FilenameParser/Scorer/Bundle), so this table pins the contract
// locally, mirroring the audiobook plugin's ParseMediaFields test shape.
type mediaFieldVector struct {
	Name  string
	Path  string
	Probe sdk.ParseProbe
	Want  []sdk.ParsedTextField
}

// TestParseMediaFieldsConformance pins ParseMediaFields — camera-make/model
// from probe tags and source-folder from the path, absent any sidecar file.
func TestParseMediaFieldsConformance(t *testing.T) {
	p := familyarchive.New()
	vectors := []mediaFieldVector{
		{
			Name: "camera tags present: make and model fields, plus source-folder",
			Path: "Trips/Yellowstone/clip.mp4",
			Probe: sdk.ParseProbe{
				Tags: map[string]string{"make": "Canon", "model": "EOS 80D"},
			},
			Want: []sdk.ParsedTextField{
				{Name: "source-folder", Text: "Yellowstone"},
				{Name: "camera-make", Text: "Canon"},
				{Name: "camera-model", Text: "EOS 80D"},
			},
		},
		{
			// com.apple.quicktime.make/model are the QuickTime-tag fallback names.
			Name: "quicktime camera tags fall back correctly",
			Path: "clip.mov",
			Probe: sdk.ParseProbe{
				Tags: map[string]string{"com.apple.quicktime.make": "Apple", "com.apple.quicktime.model": "iPhone 14"},
			},
			Want: []sdk.ParsedTextField{
				{Name: "camera-make", Text: "Apple"},
				{Name: "camera-model", Text: "iPhone 14"},
			},
		},
		{
			Name:  "no tags, no sidecar, root-relative path: nil",
			Path:  "clip.mp4",
			Probe: sdk.ParseProbe{},
			Want:  nil,
		},
		{
			Name:  "whitespace-only camera tags treated as absent",
			Path:  "clip.mp4",
			Probe: sdk.ParseProbe{Tags: map[string]string{"make": "   ", "model": ""}},
			Want:  nil,
		},
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			got := p.ParseMediaFields(v.Path, v.Probe)
			if !reflect.DeepEqual(got, v.Want) {
				t.Errorf("ParseMediaFields(...) =\n  %+v\nwant\n  %+v", got, v.Want)
			}
			got2 := p.ParseMediaFields(v.Path, v.Probe)
			if !reflect.DeepEqual(got2, got) {
				t.Errorf("ParseMediaFields(...) not deterministic: %+v then %+v", got, got2)
			}
		})
	}
}

// TestParseTypedMediaFieldsConformance pins ParseTypedMediaFields: the same
// text fields as ParseMediaFields, plus a typed capture-date field parsed via
// parseCaptureDate. The EXIF-no-TZ vector locks in the
// timezone handling — a no-timezone "2006:01:02 15:04:05" tag near a year boundary must
// keep its calendar date (Dec 31, not shifted to Jan 1) since it is stored
// date-only at noon UTC rather than parsed as a UTC instant.
func TestParseTypedMediaFieldsConformance(t *testing.T) {
	p := familyarchive.New()

	t.Run("no tags, no sidecar: nil", func(t *testing.T) {
		got := p.ParseTypedMediaFields("clip.mp4", sdk.ParseProbe{})
		if got != nil {
			t.Errorf("ParseTypedMediaFields(...) = %+v, want nil", got)
		}
	})

	t.Run("datetimeoriginal EXIF no-TZ tag near year boundary keeps local calendar date", func(t *testing.T) {
		got := p.ParseTypedMediaFields("Family/eve.mp4", sdk.ParseProbe{
			Tags: map[string]string{"datetimeoriginal": "2015:12:31 23:30:00"},
		})
		want := []sdk.ParsedField{
			{Name: "source-folder", Text: "Family"},
			{Name: "capture-date", Date: time.Date(2015, 12, 31, 12, 0, 0, 0, time.UTC)},
		}
		assertParsedFieldsEqual(t, got, want)
	})

	t.Run("RFC3339 creation_time tag keeps its explicit offset instant", func(t *testing.T) {
		got := p.ParseTypedMediaFields("clip.mp4", sdk.ParseProbe{
			Tags: map[string]string{"creation_time": "2020-06-15T09:00:00Z"},
		})
		want := []sdk.ParsedField{
			{Name: "capture-date", Date: time.Date(2020, 6, 15, 9, 0, 0, 0, time.UTC)},
		}
		assertParsedFieldsEqual(t, got, want)
	})

	t.Run("unparseable date tag yields text fields only, no capture-date", func(t *testing.T) {
		got := p.ParseTypedMediaFields("Family/clip.mp4", sdk.ParseProbe{
			Tags: map[string]string{"date": "not-a-date"},
		})
		want := []sdk.ParsedField{
			{Name: "source-folder", Text: "Family"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ParseTypedMediaFields(...) =\n  %+v\nwant\n  %+v", got, want)
		}
	})
}

func TestParseTypedMediaFieldsMTimeFallback(t *testing.T) {
	p := familyarchive.New()
	dir := t.TempDir()
	mtime := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)

	vectors := []struct {
		Name string
		Path string
		Want []sdk.ParsedField
	}{
		{
			Name: "no sidecar or tag date uses filesystem mtime",
			Path: filepath.Join(dir, "clip.mp4"),
			Want: []sdk.ParsedField{
				{Name: "source-folder", Text: filepath.Base(dir)},
				{Name: "capture-date", Date: mtime},
			},
		},
	}

	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			if err := os.WriteFile(v.Path, []byte("not a real video"), 0o644); err != nil {
				t.Fatalf("write media fixture: %v", err)
			}
			if err := os.Chtimes(v.Path, mtime, mtime); err != nil {
				t.Fatalf("set media fixture mtime: %v", err)
			}
			got := p.ParseTypedMediaFields(v.Path, sdk.ParseProbe{})
			assertParsedFieldsEqual(t, got, v.Want)
		})
	}
}

// assertParsedFieldsEqual compares ParsedField slices field-by-field, using
// time.Time.Equal for Date (reflect.DeepEqual on time.Time compares wall/
// monotonic internals and is not reliably portable) and plain equality for
// the rest.
func assertParsedFieldsEqual(t *testing.T, got, want []sdk.ParsedField) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ParseTypedMediaFields(...) =\n  %+v\nwant\n  %+v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Text != want[i].Text || !got[i].Date.Equal(want[i].Date) {
			t.Errorf("field[%d] =\n  %+v\nwant\n  %+v", i, got[i], want[i])
		}
	}
}

// TestSidecarPresent exercises the sidecar-read path across Parse,
// ParseMediaFields, and ParseTypedMediaFields with a real .json file on disk:
// sidecar Title/Summary/CaptureDate/CameraMake/
// CameraModel must win over path- and tag-derived guesses, and the sidecar's
// date-only captureDate must parse through the no-TZ layout.
func TestSidecarPresent(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "clip.mp4")
	sidecarJSON := `{
		"title": "Grandma's 80th Birthday",
		"summary": "Cake and candles in the backyard",
		"captureDate": "2019-05-04",
		"cameraMake": "Sony",
		"cameraModel": "FDR-AX53"
	}`
	if err := os.WriteFile(filepath.Join(dir, "clip.json"), []byte(sidecarJSON), 0o644); err != nil {
		t.Fatalf("write sidecar fixture: %v", err)
	}
	// Files that must not be picked up as the sidecar for mediaPath.
	if err := os.WriteFile(mediaPath, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write media fixture: %v", err)
	}

	p := familyarchive.New()

	t.Run("Parse: sidecar title wins over dir-derived title", func(t *testing.T) {
		got, ok := p.Parse(mediaPath, sdk.ParseProbe{})
		if !ok {
			t.Fatalf("Parse(...) ok = false, want true")
		}
		if got.Title != "Grandma's 80th Birthday" {
			t.Errorf("Title = %q, want sidecar title", got.Title)
		}
		if got.Class != "home-video" {
			t.Errorf("Class = %q, want home-video", got.Class)
		}
		if got.Year != 2019 {
			t.Errorf("Year = %d, want 2019 (from sidecar captureDate)", got.Year)
		}
	})

	t.Run("ParseMediaFields: sidecar text fields present, tags fill gaps", func(t *testing.T) {
		got := p.ParseMediaFields(mediaPath, sdk.ParseProbe{})
		want := []sdk.ParsedTextField{
			{Name: "source-folder", Text: filepath.Base(dir)},
			{Name: "sidecar-title", Text: "Grandma's 80th Birthday"},
			{Name: "sidecar-summary", Text: "Cake and candles in the backyard"},
			{Name: "camera-make", Text: "Sony"},
			{Name: "camera-model", Text: "FDR-AX53"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ParseMediaFields(...) =\n  %+v\nwant\n  %+v", got, want)
		}
	})

	t.Run("ParseTypedMediaFields: adds capture-date from sidecar date-only string", func(t *testing.T) {
		got := p.ParseTypedMediaFields(mediaPath, sdk.ParseProbe{})
		if len(got) == 0 {
			t.Fatalf("ParseTypedMediaFields(...) = empty, want fields")
		}
		last := got[len(got)-1]
		// Date-only sidecar strings go through the same no-TZ dateOnlyUTC path
		// as EXIF tags: calendar date kept, pinned to noon UTC.
		wantDate := time.Date(2019, 5, 4, 12, 0, 0, 0, time.UTC)
		if last.Name != "capture-date" || !last.Date.Equal(wantDate) {
			t.Errorf("last field = %+v, want capture-date %v", last, wantDate)
		}
	})

	t.Run("no sidecar file: fields fall back to path/tag-only sources", func(t *testing.T) {
		noSidecarPath := filepath.Join(dir, "nosidecar.mp4")
		got, ok := p.Parse(noSidecarPath, sdk.ParseProbe{})
		if !ok {
			t.Fatalf("Parse(...) ok = false, want true")
		}
		if got.Title != filepath.Base(dir) {
			t.Errorf("Title = %q, want dir-derived title %q", got.Title, filepath.Base(dir))
		}
	})
}

// TestSidecarMixedTypeFieldDropsOnlyThatField pins the fallback decode's
// per-field tolerance: a sidecar with one field of an unexpected JSON type
// (captureDate as a number instead of a string) must still yield the valid
// string fields, not discard the whole sidecar.
func TestSidecarMixedTypeFieldDropsOnlyThatField(t *testing.T) {
	dir := t.TempDir()
	mediaPath := filepath.Join(dir, "clip.mp4")
	sidecarJSON := `{"title":"Grandma Bday","captureDate":2019}`
	if err := os.WriteFile(filepath.Join(dir, "clip.json"), []byte(sidecarJSON), 0o644); err != nil {
		t.Fatalf("write sidecar fixture: %v", err)
	}
	if err := os.WriteFile(mediaPath, []byte("not a real video"), 0o644); err != nil {
		t.Fatalf("write media fixture: %v", err)
	}

	p := familyarchive.New()
	got, ok := p.Parse(mediaPath, sdk.ParseProbe{})
	if !ok {
		t.Fatalf("Parse(...) ok = false, want true")
	}
	if got.Title != "Grandma Bday" {
		t.Errorf("Title = %q, want %q", got.Title, "Grandma Bday")
	}
	// captureDate's wrong JSON type (number, not string) must be skipped, not
	// crash or discard the sidecar; with no sidecar/tag date the typed field
	// falls back to filesystem mtime rather than the (unread) 2019 value.
	fields := p.ParseMediaFields(mediaPath, sdk.ParseProbe{})
	for _, f := range fields {
		if f.Name == "sidecar-title" && f.Text != "Grandma Bday" {
			t.Errorf("sidecar-title = %q, want %q", f.Text, "Grandma Bday")
		}
	}
	if got.Year == 2019 {
		t.Errorf("Year = %d, want mtime-derived year, not the wrong-typed sidecar captureDate value", got.Year)
	}
}
