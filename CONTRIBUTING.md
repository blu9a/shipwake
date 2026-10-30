# Contributing

Thank you for helping. Shipwake is built the way it asks projects to work: agree on the behavior first, then
build it, and let every change show what it did.

## Ways to help

- **Try it on your project** and tell us what didn't fit, in [Discussions](https://github.com/blu9a/shipwake/discussions).
- **Report a bug** with an issue. Security problems go privately, as [SECURITY.md](SECURITY.md) describes.
- **Add a converter** for a tool Shipwake can't read yet: see [docs/converters.md](docs/converters.md).
- **Change Shipwake itself** with a pull request, as below.

## Build and test

You need Git, Go (the version in `go.mod`), and Node with pnpm (the versions in `web/package.json`).

```sh
make                 # the web app and the CLI: bin/shipwake
make test            # Go tests
make lint            # vet, formatting, types and the web app's lint
make test-responsive # browser checks on the production build (install Chromium once:
                     #   cd web && pnpm exec playwright install chromium)
make test-history    # the timeline's order on this repository's own history and runs
make fixtures        # once, for the benchmarks that build a real repository's site
bin/shipwake serve . # this repository's own site, at http://127.0.0.1:7070
```

When you change `internal/model`, run `make schema`: it regenerates `spec/shipwake.schema.json` and the web
app's types. Never edit those files by hand. The tool's own text is in `web/src/messages.ts`, in every
language it has.

## The loop

1. **Propose the behavior first.** For a new feature, add scenarios to a `.feature` file tagged `@proposed`
   (or a `.metric` file for something measured), in the words of someone using Shipwake. Say what it does,
   not how; don't pin details nobody decided.
2. **A maintainer approves it** in Shipwake (`bin/shipwake serve .`, at the top of the timeline), changing it
   if needed. Approval removes `@proposed` in a commit with `Approved-by`.
3. **Build it,** with the tests that run the scenarios. CI records every run on this repository's results
   branch, so the change shows on Shipwake's own timeline: what passed, what got faster or slower, and the
   recordings.
4. **Release.** A maintainer releases from Shipwake's release page; the changelog is assembled from the
   commits and the requirements they changed.

Small fixes that change no behavior skip the first two steps.

## Commits and pull requests

- One topic per pull request. Titles and commit subjects follow
  [Conventional Commits](https://www.conventionalcommits.org): `feat:`, `fix:`, `perf:`, `docs:`, `test:`,
  `refactor:`, `chore:`, `spec:` for requirements.
- Each commit message says **what changed and why, how you verified it, and how to roll it back.** The history
  is what the next person, or agent, reads first.
- Add `Release-note: <one line for people using Shipwake>` to a commit they would notice, or
  `Release-note: none`.
- Pull requests are merged with a merge commit, so a branch's commits stay visible in its merge.

## Principles

- **Never push on its own.** Shipwake changes only the local repository; sending anything to a remote happens
  only when the person asks (`--push`, `push: true`).
- **Git and result files only.** No server and no hosting platform API in the core.
- **Deterministic verdicts.** Pass, fail and thresholds never depend on a model.
- **Nothing changes in a project's code** to adopt Shipwake.
- **Docs follow the code** in the same change: `docs/`, the agent skill and the README.

## Contributions made with AI

Welcome. You are responsible for what you submit, however it was written: read it, run it, and be ready to
explain and maintain it as your own work.

## Conduct

Everyone taking part follows the [Code of Conduct](CODE_OF_CONDUCT.md).
