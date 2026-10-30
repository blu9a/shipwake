# Releases

A release is a section in the changelog file and an annotated Git tag. `CHANGELOG.md` holds every
release's notes, newest first; the tag marks the released commit and holds the same notes and who
approved them. Both live in the repository, work with any hosting and travel with every clone. A
release shows its section from the trunk's newest `CHANGELOG.md`, or its tag's message when the file
has no section for it (`changelog: none` in `.shipwake.yaml` keeps notes in tags only).

Shipwake reads the common ways of writing a release's heading: `## [1.2.0] - 2026-10-30` (as
[Keep a Changelog](https://keepachangelog.com) describes, with an `## [Unreleased]` section),
`## [1.2.0](…) (2026-10-30)` with a compare link, and a bare `## 1.2.0`.

## Preparing a release

Open **New release** on the trunk. The page shows:

- a suggested version: a minor version when a feature was merged since the last release, a patch otherwise;
- **checks** on the newest trunk commit: tests that ran on it, measurements against the last release,
  the version, and notes in each configured language;
- the **notes**, one tab per language, and a preview.

**Assemble draft** writes the changelog under your own text, sorting commits by their
[Conventional Commits](https://www.conventionalcommits.org) type:

| Section | From |
|---|---|
| ⚠ Breaking changes | a `!` after the type, or a `BREAKING CHANGE:` footer (its text) |
| Requirements | scenarios and metrics approved, changed or removed in the release (no convention exists for these) |
| Features | `feat` |
| Bug fixes | `fix` |
| Performance improvements | `perf` |
| Reverts | `revert` |
| Other | commits that don't follow the convention |

`chore`, `test`, `ci`, `build`, `style`, `refactor`, `docs` and `spec` commits are work for the team and
are left out. Each item ends with its commit. Assembling again replaces the sections and keeps your text
above them. Edit the result into what people using the product need to know: what you approve is the
tag's message, and the release shows exactly that, everywhere.

**Approve release** is enabled when nothing blocks. Under `shipwake serve` it records the release, as you:
it adds the section to `CHANGELOG.md` (created on the first release; `CHANGELOG.<lang>.md` for each other
language), commits it as `chore(release): v0.2.0` on top of the released commit, which must be the
newest, and tags that commit. The release commit only adds the notes. Nothing is pushed. When everything
on the trunk is released, the page says so and offers nothing to tag.

A published site is static, so it gives the command that creates the tag; it doesn't write the
changelog file, so release under `shipwake serve` to keep it. Neither pushes: send the tag to a remote
yourself when you want to (`git push origin v0.2.0`).

```sh
git tag -a v0.2.0 <commit> -F notes.txt
```

## The tag message

```text
Smarter suggestions

- Suggestions explain why they were made.
- Undo works for every decision.

--- zh

更聪明的建议

- 每条建议都说明理由。
- 每个决定都可以撤销。

Approved-by: Ada <ada@example.com>
```

The first language comes first; each other language follows a `--- <code>` line. Trailers such as
`Approved-by:` go last. The first line is the title when it stands alone.

## Notes in commits

Write user-facing notes where the change is made, as trailers:

```text
feat: explain suggestions

Release-note: Suggestions explain why they were made.
Release-note-zh: 每条建议都说明理由。
```

A note replaces the commit's subject in the changelog; `Release-note: none` leaves an internal change
out. They are only collected; nothing is published until the release is approved.
