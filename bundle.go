// Package familyarchive is the first-party Family Archive Domain Plugin:
// household home videos and personal archive Media organized by
// Event, Person, Place, and Trip Entity Kinds. The bundle is purely
// declarative, adds no schema or query strings, and performs no network
// scraping by default.
package familyarchive

import sdk "github.com/beamfall/plugin-sdk"

// Bundle returns the Family Archive plugin's declarative contribution.
func Bundle() sdk.Bundle {
	return sdk.Bundle{
		ID:                   PluginID,
		Version:              "0.1.4",
		SDKFloor:             "0.2.0",
		SDKCeiling:           "0.3.2",
		DisplayName:          sdk.LocaleString{"en": "Family Archive", "fr": "Archives familiales"},
		MediaLabel:           sdk.LocaleString{"en": "Home Video", "fr": "Vidéo familiale"},
		MediaClasses:         []string{"video"},
		IdentificationPolicy: sdk.IdentificationPolicyQueue,
		Accent:               "#4B8A7A",
		MediaFields: []sdk.FieldSpec{
			{Name: "class", Type: "text",
				Label: sdk.LocaleString{"en": "Class", "fr": "Classe"}},
			{Name: "capture-date", Type: "date", Sortable: true,
				Label: sdk.LocaleString{"en": "Captured", "fr": "Date de capture"}},
			// PII disclosure: source-folder is the parent
			// directory's name, which commonly contains personal identifiers
			// (people, places, events). See sourceFolder in parse.go.
			{Name: "source-folder", Type: "text", Filterable: true,
				Label: sdk.LocaleString{"en": "Source Folder", "fr": "Dossier source"}},
			{Name: "sidecar-title", Type: "text", Searchable: true,
				Label: sdk.LocaleString{"en": "Sidecar Title", "fr": "Titre du fichier compagnon"}},
			{Name: "sidecar-summary", Type: "text", Searchable: true,
				Label: sdk.LocaleString{"en": "Sidecar Summary", "fr": "Résumé du fichier compagnon"}},
			{Name: "camera-make", Type: "text",
				Label: sdk.LocaleString{"en": "Camera Make", "fr": "Marque de l'appareil"}},
			{Name: "camera-model", Type: "text",
				Label: sdk.LocaleString{"en": "Camera Model", "fr": "Modèle de l'appareil"}},
		},
		MediaClassLabels: map[string]sdk.LocaleString{
			classHomeVideo: {"en": "Home Video", "fr": "Vidéo familiale"},
		},
		EntityKinds: []sdk.EntityKind{
			{
				Name:            "event",
				DisplayName:     sdk.LocaleString{"en": "Event", "fr": "Événement"},
				Archetype:       "container",
				OrderedChildren: true,
				Fields: []sdk.FieldSpec{
					{Name: "date", Type: "date", Sortable: true,
						Label: sdk.LocaleString{"en": "Date", "fr": "Date"}},
					{Name: "summary", Type: "text", Searchable: true,
						Label: sdk.LocaleString{"en": "Summary", "fr": "Résumé"}},
				},
				Relations: []sdk.RelationSpec{
					{Name: "contains", ToKind: sdk.MediaKindName},
					{Name: "at-place", ToKind: "place"},
					{Name: "features", ToKind: "person"},
				},
			},
			{
				// Person is manual-only: "appears-in" is a
				// user-curated relation. This plugin performs no face
				// recognition or other automated biometric tagging, and adding
				// any would be a new privacy-sensitive capability needing its
				// own design review, not an incremental change here.
				Name:        "person",
				DisplayName: sdk.LocaleString{"en": "Person", "fr": "Personne"},
				Archetype:   "person",
				Fields: []sdk.FieldSpec{
					// PII disclosure: free-text biographic notes,
					// Searchable means the host's full-text index will contain
					// this personal data; hosts should encrypt the index at rest.
					{Name: "notes", Type: "text", Searchable: true,
						Label: sdk.LocaleString{"en": "Notes", "fr": "Notes"}},
				},
				Relations: []sdk.RelationSpec{
					{Name: "appears-in", ToKind: sdk.MediaKindName},
				},
			},
			{
				Name:        "place",
				DisplayName: sdk.LocaleString{"en": "Place", "fr": "Lieu"},
				Archetype:   "category",
				Fields: []sdk.FieldSpec{
					// PII disclosure: see person.notes above —
					// same searchable-index-at-rest caveat applies.
					{Name: "notes", Type: "text", Searchable: true,
						Label: sdk.LocaleString{"en": "Notes", "fr": "Notes"}},
				},
				Relations: []sdk.RelationSpec{
					{Name: "located-media", ToKind: sdk.MediaKindName},
				},
			},
			{
				Name:            "trip",
				DisplayName:     sdk.LocaleString{"en": "Trip", "fr": "Voyage"},
				Archetype:       "container",
				OrderedChildren: true,
				Fields: []sdk.FieldSpec{
					{Name: "start-date", Type: "date", Sortable: true,
						Label: sdk.LocaleString{"en": "Start Date", "fr": "Date de début"}},
					{Name: "end-date", Type: "date", Sortable: true,
						Label: sdk.LocaleString{"en": "End Date", "fr": "Date de fin"}},
					{Name: "summary", Type: "text", Searchable: true,
						Label: sdk.LocaleString{"en": "Summary", "fr": "Résumé"}},
				},
				Relations: []sdk.RelationSpec{
					{Name: "contains", ToKind: sdk.MediaKindName},
					{Name: "visits", ToKind: "place"},
					{Name: "features", ToKind: "person"},
				},
			},
		},
		Layouts: []sdk.LayoutSpec{{ID: "home", Role: "browse", Sections: []sdk.SectionSpec{
			{ID: "recently-added", Kind: "shelf",
				Title:       sdk.LocaleString{"en": "Recently Added", "fr": "Ajouts récents"},
				ArtworkSlot: "thumb", Source: sdk.SourceSpec{Kind: "recently-added"}},
			{ID: "events", Kind: "grid",
				Title:       sdk.LocaleString{"en": "Events", "fr": "Événements"},
				ArtworkSlot: "poster", Source: sdk.SourceSpec{Kind: "entity-items", EntityKind: "event"}},
			{ID: "trips", Kind: "grid",
				Title:       sdk.LocaleString{"en": "Trips", "fr": "Voyages"},
				ArtworkSlot: "poster", Source: sdk.SourceSpec{Kind: "entity-items", EntityKind: "trip"}},
			{ID: "people", Kind: "grid",
				Title:       sdk.LocaleString{"en": "People", "fr": "Personnes"},
				ArtworkSlot: "portrait", Source: sdk.SourceSpec{Kind: "entity-items", EntityKind: "person"}},
			{ID: "places", Kind: "grid",
				Title:       sdk.LocaleString{"en": "Places", "fr": "Lieux"},
				ArtworkSlot: "poster", Source: sdk.SourceSpec{Kind: "entity-items", EntityKind: "place"}},
			{ID: "all-videos", Kind: "grid",
				Title:       sdk.LocaleString{"en": "All Videos", "fr": "Toutes les vidéos"},
				ArtworkSlot: "thumb", Source: sdk.SourceSpec{Kind: "all-items"}},
		}}},
	}
}
