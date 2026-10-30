# Changelog

## v0.1.0 (2026-10-31)

The first public alpha: check what your product does, not every line your agents wrote.

### What's in it

- **Approved behavior instead of diffs.** Agents propose scenarios in plain words; you edit and approve them in a local dashboard, and each approval is your own commit. Every change then shows the approved behavior it added, changed or broke, with the diff beside it.
- **Recordings.** Every scenario records itself, a box marks what it checked, and the run shows what it actually saw. Watch a whole feature in seconds.
- **Metrics and evaluations.** Tests, benchmarks and AI evaluations become trends, with limits you approve the same way.
- **Releases.** Checked, drafted from the changes, recorded in `CHANGELOG.md` and an annotated tag naming who approved it.
- **Local mode.** Try it on any repository without writing a single file into it; `shipwake adopt` when you're ready.
- **One prompt to start.** Paste it to your coding agent: see the README.
- **Playwright reporter.** `npm i -D shipwake-playwright`, for synced subtitles, Background steps and evidence.

This is an alpha: commands and formats may still change. Recorded results stay readable by every later version (see Compatibility).
