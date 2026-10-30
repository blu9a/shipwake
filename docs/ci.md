# CI

Record each run right after it happens, on every branch. The results branch collects them; any job can
then build the site or gate on `shipwake check`.

## GitHub Actions

```yaml
permissions:
  contents: write   # to push the results branch

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - run: go test -json ./... > go-test.json
        continue-on-error: true
      - uses: blu9a/shipwake@v0
        if: always()
        with:
          format: go-test
          file: go-test.json
          name: Go tests
          context: runner=github-ubuntu
          push: true        # keep the run: send the results branch to origin
```

The action installs Shipwake and records the run. On pull requests it records the branch's own commit,
not the temporary merge commit. Inputs: `format`, `file`, `name`, `kind`, `context` (one `key=value` per
line), `push`, `version`.

Shipwake never pushes on its own: `push` is `false` unless you set it. Without it the run is recorded in the
job's checkout only and is gone when the job ends, so CI that should keep results sets `push: true`.

### Publish the site on GitHub Pages

```yaml
on:
  workflow_run: { workflows: [CI], types: [completed] }
permissions: { contents: read, pages: write, id-token: write }
jobs:
  pages:
    runs-on: ubuntu-latest
    environment: github-pages
    steps:
      - uses: actions/checkout@v7
        with: { fetch-depth: 0 }      # the history is the timeline
      - uses: blu9a/shipwake@v0       # install only
      - run: shipwake build -o site
      - uses: actions/upload-pages-artifact@v5
        with: { path: site }
      - uses: actions/deploy-pages@v5
```

### Gate merges and releases

```yaml
      - run: shipwake check --against previous   # in a PR: what this change did
      - run: shipwake check                      # before a release: against the last release
```

`check` exits 1 when a test run on the commit has failures, or a measurement crosses a blocking threshold.
Warnings are printed and don't fail. `--json` prints the verdict for other tools.

## GitLab CI

```yaml
test:
  script:
    - pytest --junitxml=pytest.xml || status=$?
    - shipwake add -f junit --file pytest.xml -n "Python tests" --context runner=gitlab --push
    - exit ${status:-0}
```

The job needs a token that can push, for example a project access token with `write_repository` set as the
remote: `git remote set-url origin "https://oauth2:${SHIPWAKE_TOKEN}@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git"`.

## Any other CI

Three things are needed: the Shipwake binary, a checkout that can push, and the branch name, which Shipwake
reads from `GITHUB_HEAD_REF`, `GITHUB_REF_NAME`, `CI_COMMIT_REF_NAME`, `BUILDKITE_BRANCH` or
`CIRCLE_BRANCH`, or from `--branch`.

If pushing from CI isn't possible, store results in a directory instead (`-r path`) and keep that
directory wherever your CI keeps artifacts.

## Runners

Give every run a `runner` context. Measurements are only compared with runs on the same runner, so a
change of machine never looks like a regression. Tests don't need it, but it is shown on the run.

## Performance and evals

Run them where the numbers are stable: a dedicated runner, the same load, several samples (`values`).
Nightly or on merges to the trunk is usually enough; the timeline shows each run on the commit it tested.
