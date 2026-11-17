## ADDED Requirements

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
