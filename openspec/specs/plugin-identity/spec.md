# plugin-identity Specification

## Purpose

The identity the plugin presents to a host: the display name people see in library type pickers
and the plugin ID hosts key on.

## Requirements

### Requirement: Display name

The bundle SHALL declare the display name "Archive" for `en` and "Archives" for `fr`, and SHALL
keep the plugin ID `familyarchive`.

#### Scenario: A host lists library types

- **WHEN** a host reads the bundle's display name for the `en` locale
- **THEN** it receives "Archive"

#### Scenario: A French host lists library types

- **WHEN** a host reads the bundle's display name for the `fr` locale
- **THEN** it receives "Archives"

#### Scenario: Existing libraries keep resolving

- **WHEN** a host looks the bundle up by the ID `familyarchive`
- **THEN** it finds this bundle
