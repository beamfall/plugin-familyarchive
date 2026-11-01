<p align="center">
  <img src=".github/brand/beamfall-logo.png" alt="Beamfall" width="96" height="96">
</p>

# Beamfall Family Archive

Family Archive is the
[Beamfall](https://github.com/beamfall/core) plugin for household home videos and personal
archive footage. It turns a folder of clips into a library you can browse by event, trip, person,
and place, with titles and capture dates read from the files you already have.

The plugin reads local files only. It makes no network calls, runs no face recognition and does
not extract location data.

## Features

- The home screen has shelves for Recently Added, Events, Trips, People, Places, and All Videos.
- Clips are organized four ways:
  - Events and Trips are ordered collections with dates and a summary. Each can feature people,
    and a trip can record the places it visits.
  - People and Places carry free-text notes and link to the clips they appear in.
- A clip takes its title from its folder, such as
  `Summer Vacation 2018/IMG_0042.mp4` becoming "Summer Vacation 2018", or from the file name when
  it sits at the top level. Underscores and dots become spaces.
- The plugin reads the capture date from a sidecar file,
  then from the video's embedded tags (including Apple QuickTime creation dates), then falls back
  to the file's modified time. Dates without a timezone keep their calendar day, so a clip shot
  on 31 December never drifts into January.
- Optional JSON sidecars: put `clip.json` next to `clip.mp4` to set `title`, `summary`,
  `captureDate`, `cameraMake`, and `cameraModel`. A missing, oversized, or malformed sidecar is
  skipped without failing the scan, and a single bad field drops only that field.
- You can sort by capture date, filter by source folder, search
  sidecar titles and summaries, and see the camera make and model.
- Privacy:
  - People are linked to clips by hand. There is no face recognition or other biometric tagging.
  - GPS and location metadata are never extracted.
  - Sidecar keys from photo-library exports, such as `description` or bare `make` and `model`,
    are deliberately ignored, so unrelated metadata does not end up in your search index.
- All labels are available in English and French.

## How it fits into Beamfall

Family Archive ships with [Beamfall Core](https://github.com/beamfall/core), the self-hosted media
server: when you run Core, family archives are already available. Core is free and open source
under AGPL-3.0, needs no account, and has no telemetry.

The plugin is built on the [Beamfall plugin SDK](https://github.com/beamfall/plugin-sdk). It
provides two pieces:

- `Bundle()` in [`bundle.go`](bundle.go) declares the library: the Entity Kinds, the home layout,
  and the Media fields.
- `Parser` in [`parse.go`](parse.go) runs during a scan and derives titles, capture dates, and
  field values from paths, embedded tags, and sidecars.

The SDK's [plugin-authoring guide](https://github.com/beamfall/plugin-sdk/blob/main/docs/plugin-authoring.md)
explains how Core loads plugins and which extension points they can implement.

## Develop

You need Go (the version is set in [`go.mod`](go.mod)), Python 3, and `golangci-lint`.

```sh
go test ./...        # unit tests and the SDK conformance harness
./script/gate.sh     # gofmt, go vet, tests with a coverage floor, golangci-lint
```

[`conformance_test.go`](conformance_test.go) runs the SDK's `sdktest` harness over golden parser
vectors and the declared bundle, so any change to how a clip is titled or dated shows up as a
failing test. CI also runs the tests under the race detector, and a daily scheduled job fuzzes the
sidecar reader and the capture-date parser.

## Contributing

Contributions are welcome from contributors with a signed Contributor License Agreement. Read
[CONTRIBUTING.md](CONTRIBUTING.md) and [CLA.md](CLA.md) before opening a pull request.

## Security

Please report vulnerabilities privately. [SECURITY.md](SECURITY.md) explains how.

## License

Family Archive is licensed under the GNU Affero General Public License, version 3. See
[LICENSE](LICENSE).
