# Compatibility

Results are history: a run recorded today should still be read in five years, by whatever Shipwake
is current then. These are the promises Shipwake keeps, and the few it doesn't make yet.

## Recorded runs

Every run file on a results branch names its format: `"schemaVersion": 1`. Runs recorded before the
field existed are format 1.

- **Every Shipwake reads every earlier format.** Results you recorded are never stranded by an upgrade.
- **Changes within a format are additive:** new optional fields, never a field removed or given a new
  meaning. An older Shipwake reading a newer run of the same format ignores what it doesn't know.
- **A change that isn't additive is a new format.** Shipwake then reads the older formats by converting
  them as it reads them. It never rewrites stored results: they stay as they were recorded.
- **A run in a format newer than the Shipwake reading it is refused by name,** with the file and the
  version to upgrade to, rather than misread.

The same holds for [Shipwake's JSON format](recording.md#shipwakes-own-format) that converters and
scripts write: its `schemaVersion` is optional, and a file without it is format 1.

## What you write

`.shipwake.yaml`, `.feature` and `.metric` files, converters, the commands and the GitHub Action's inputs.

- **Before 1.0,** a release may change how something you wrote is read. Such a change is listed first in
  that release's notes, under breaking changes, with what to change. `shipwake doctor` points at what
  needs it.
- **From 1.0,** only a major version does.

## Not covered

- The site Shipwake builds (`data/*.json`): it is made again from Git and the results on every build.
- The page's look and layout.
