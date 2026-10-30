# AGENTS.md

Instructions for coding agents working on Shipwake.

## Principles

- **Never push on its own.** Shipwake changes only the local repository: commits, tags, the results branch. Sending anything to a remote happens only when the person asks for it explicitly (`--push`, `push: true`); no default, suggestion or copied command pushes for them.
- **Git and result files only.** No hosting platform APIs in the core.
- **Deterministic results.** Verdicts, measurements and checks never call a model.
- **The model is the source of truth.** Change `internal/model`, then run `make schema` to regenerate `spec/shipwake.schema.json` and `web/src/model.gen.ts`. Never edit the generated files.
- **Converters before code.** A new format is a mapping in `internal/readers/formats` (the format users
  write too, see docs/converters.md) with a real report as its sample and a reviewed expected run;
  `go test ./internal/readers` checks them (`-update` rewrites, then review the diff). Write a Go reader
  only when a mapping can't express the format, one file in `internal/readers` with a sample in `readers_test.go`.
- **Samples are real.** Produce them by running the tool; remove absolute paths and anything private.
- **The page has no sidebar.** Navigation is the top bar: project, branch, settings.
- **Keep responsive invariants.** Follow `DESIGN.md`; reuse the shared measurement-row layout. For layout changes run `make test-responsive` and inspect the affected pages with real data on phone and desktop widths.
- **One verdict.** `shipwake check` and the release page judge a commit the same way (the newest comparable run of each series); change both together.
- **Docs follow the code.** `docs/`, `skill/shipwake/SKILL.md` and `action.yml` describe commands and formats; update them in the same change.

## Commands

- `make` builds the web app and the CLI; `make test`, `make lint`.
- `bin/shipwake serve <repo>` builds into a temporary folder and serves it.

## Text

The tool's own text lives in `web/src/messages.ts`; every language has every key. Project text (features, scenarios, steps) comes from the project's translation files, falling back to the original.
