Ticket: CORE-2236 (`queue:core:main`). Status: proposed.

## Why

The plugin shows up in Beamfall's library type picker as "Family Archive". Owners use it for any
loose collection of personal video, not only family footage, and the name reads as a restriction
the plugin does not have.

## What Changes

- The bundle's display name becomes "Archive" (`en`) and "Archives" (`fr`).

## Non-goals

- The plugin ID (`familyarchive`), module path, repository name and Go package name stay as they
  are, so existing libraries, activation overrides and core's pin keep resolving.
- The media label ("Home Video"), the entity kinds and the parser are unchanged.
- Prose in README, CONTRIBUTING and AGENTS keeps the project name "Beamfall Family Archive".

## Failure modes

- A host that keys anything on the display name instead of the plugin ID would lose the match.
  Core keys on the ID; the label is presentation only.
- Core test fixtures and docs that spell "Family Archive" are independent strings and do not
  follow this change.

## Acceptance evidence

- `TestBundleDisplayName` pins both locales.
- `script/gate.sh` green on the landing head.
- After core repins the release, the `plugins` query on a running server returns
  `displayName: "Archive"` for `familyarchive`.

## Rollback

Revert the commit and release the next patch tag; core repins it. No stored data depends on the
label.
