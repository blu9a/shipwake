# Getting started

Shipwake needs two things: a Git repository, and the result files your tests already write. It keeps the
results on a branch of the same repository, so there is no database, bucket or account to set up.

## 1. Look at your repository

```sh
shipwake init
```

`init` changes nothing. It recognizes the test setup (Go, Playwright, Vitest, Jest, pytest, Maven, Gradle,
cargo-nextest, Go benchmarks, `.feature` files) and prints the exact commands to add to CI.
`shipwake init --write` also adds a commented `.shipwake.yaml`; every setting in it is optional.

## 2. Record a run

After the tests run, record their output with the commit they tested:

```sh
go test -json ./... > go-test.json
shipwake add -f go-test --file go-test.json -n "Go tests"
```

The run is committed to the branch `shipwake-results` (an orphan branch: it shares no history with your
code, and never shows up as an open branch). Shipwake never pushes on its own; in CI, add `--push` to keep the runs. See [CI](ci.md).

Anything else you measure, such as eval scores, load tests and benchmarks, is recorded the same way. See
[Recording runs](recording.md).

## 3. Look at it

```sh
shipwake serve        # http://127.0.0.1:7070
```

The trunk is a timeline: merges with their branch's commits, plain commits, releases, and the runs on each.
Every commit has its page with the files and scenarios it changed; every run has its page with the tests
or measurements, compared with the previous run or the last release.

## 4. Publish and gate

```sh
shipwake build -o site     # a static folder: GitHub Pages, any web server, a CI artifact
shipwake check             # exit 1 when a test fails or a measurement crosses a blocking threshold
```

## Trends from day one

Most projects keep no results, so the first build has one run per suite. Trends fill in as CI records each
change. To have them now, record your recent releases once: your usual command, on each release, against that
release's commit.

```sh
for tag in $(git tag --sort=-v:refname | head -10); do
  dir=$(mktemp -d)
  git worktree add -q --detach "$dir" "$tag"
  (cd "$dir" && go test -json ./... > "$dir.json")      # install dependencies first if the project needs them
  shipwake add -f go-test --file "$dir.json" -n "Go tests" --commit "$(git rev-parse "$tag^{commit}")" --context runner=laptop
  git worktree remove --force "$dir"; rm -f "$dir.json"
done
```

Any command and format work the same way. Use the names your CI uses, so the runs sit together, and a runner name
of their own: runs from your machine are compared with each other, not with CI's.

## Several repositories

One site can show several projects; each is picked from the top bar.

```sh
shipwake build ../app ../api ../agent -o site
```

## With an AI agent

Shipwake is deterministic: what passes, fails or crosses a threshold never depends on a model. Agents are
useful around it, at setup and writing time: wiring it into CI, converting an unusual tool's output to
Shipwake's format, drafting release notes. The [agent skill](../skill/shipwake/SKILL.md) gives an agent what
it needs for these jobs.
