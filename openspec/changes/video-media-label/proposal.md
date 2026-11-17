Ticket: CORE-2273 (`queue:core:main`). Status: proposed.

## Why

CORE-2236 renamed the plugin to "Archive" because owners use it for any loose collection of
personal video. Its items are still labelled "Home Video", which carries the same restriction the
old name did.

## What Changes

- The bundle's media label becomes "Video" (`en`) and "Vidéo" (`fr`).
- The label of the `home-video` media class becomes the same.

## Non-goals

- The media class ID `home-video` stays, so parsed and stored media keep their class.
- The plugin ID, display name, entity kinds and parser are unchanged.

## Failure modes

- A host that keys on a label instead of the class ID would lose the match. Core keys on IDs;
  labels are presentation only.

## Acceptance evidence

- `TestBundleMediaLabels` pins both locales for the media label and the class label.
- `script/gate.sh` green on the landing head.
- After core repins the release, the `plugins` query on a running server returns
  `mediaLabel: "Video"` for `familyarchive`.

## Rollback

Revert the commit and release the next patch tag; core repins it. No stored data depends on the
labels.
