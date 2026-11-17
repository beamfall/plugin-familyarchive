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

### Requirement: Media labels

The bundle SHALL label its media "Video" for `en` and "Vidéo" for `fr`, SHALL give the
`home-video` media class the same labels, and SHALL keep the class ID `home-video`.

#### Scenario: A host labels an item

- **WHEN** a host reads the bundle's media label for the `en` locale
- **THEN** it receives "Video"

#### Scenario: A host labels a media class

- **WHEN** a host reads the label of the `home-video` class for the `fr` locale
- **THEN** it receives "Vidéo"

#### Scenario: Stored media keep their class

- **WHEN** the parser classifies a file
- **THEN** the class ID is `home-video`
