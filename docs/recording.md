# Recording runs

A **run** is one execution of something on one commit: a test suite, an eval, a load test, a benchmark.
`shipwake add` reads a tool's output and stores it as a run.

```sh
shipwake add -f <format> --file <path> -n "<name>" [--context key=value]... [--push]
```

| Flag | |
|---|---|
| `-f, --format` | the tool's output format (below) |
| `--file` | the output file; attachments it refers to are copied with the run |
| `-n, --name` | the run's name. Runs with the same name form a series |
| `--kind` | your word for the kind of run: `test`, `eval`, `load test`… (default from the format) |
| `--context` | what the run used: `runner=ci-linux`, `model=…`, `load=50 users`… |
| `--commit` | the commit tested (default `HEAD`) |
| `--branch` | the branch (default the current one, or CI's: GitHub, GitLab, Buildkite, CircleCI) |
| `-r, --results` | where to store it: `git:<branch>` or a directory |
| `--push` | push the results branch; concurrent jobs are rebased and retried |

## Formats

| Format | Kind | Tool |
|---|---|---|
| `junit` | test | JUnit XML: pytest, Jest, Vitest, Maven, Gradle, cargo-nextest, .NET, PHPUnit and most others |
| `go-test` | test | `go test -json` |
| `playwright` | test | Playwright's JSON reporter |
| `go-bench` | benchmark | `go test -bench` output |
| `k6` | load test | `k6 run --summary-export=summary.json` |
| `cucumber-json` | test | Cucumber JSON from any Gherkin runner: cucumber-js, Cucumber-JVM, godog, Reqnroll… |
| `jest-json` | test | Jest `--json`, Vitest `--reporter=json` |
| `pytest-json` | test | pytest with pytest-json-report |
| `json` | any | Shipwake's own format, for everything else |

For any other tool, a [converter](converters.md) in your repository describes its report in a few
lines of YAML: no code, checked against a real report, and against the tool's own counts.

For Playwright, start with its own JSON reporter (`-f playwright`): scenarios, steps, errors, screenshots and
videos. The [`@shipwake/playwright`](../packages/playwright/README.md) reporter adds subtitles in step with
each recording, Gherkin Background steps in their place, and helpers that mark what a result step checked.

## Shipwake's own format

Anything without a reader can write this. The full schema is `shipwake schema` (the `run` property).

```json
{
  "name": "Agent evals",
  "kind": "eval",
  "context": { "model": "model-2026-10", "dataset": "suggestions v3 (40 cases)", "runner": "eval-runner" },
  "metrics": [
    { "id": "quality", "suite": "Agent quality", "name": "Suggestion quality", "unit": "score", "direction": "higher", "values": [0.91, 0.89, 0.92] },
    { "id": "latency", "suite": "Agent cost and speed", "name": "Reply time p95", "unit": "ms", "direction": "lower", "values": [2977] }
  ]
}
```

- `values` are repeated samples of one measurement; Shipwake shows their mean and spread.
- `direction` says which way is better: `higher` or `lower` (default).
- `unit` is free text, printed after the value; `ns`, `ms`, `%` and `score` have numeric formatting.
  Binary storage units (`B`, `KiB`, `MiB`, `GiB`, `TiB`, `PiB`, `EiB`) scale upward by 1024 for display;
  for example, 62943 KiB is shown as 61.5 MiB. Stored values, deltas and thresholds keep their recorded unit.
- A file may name its `commit` and `branch`; flags override them.
- `schemaVersion` is optional: a file without it is format 1. See [Compatibility](compatibility.md).
- Tests use the same shape as every reader produces: `{"tests": [{"feature", "name", "target", "status", "duration", "steps", "attachments"}]}`.

An agent can usually write the converter from your eval runner or load tool to this format in one go; see
the [agent skill](../skill/shipwake/SKILL.md).

## Context and comparisons

Measurements are compared only within a series: the same name and `runner` context. A run on
another runner is shown but not compared ("Other runner"), because a laptop and a CI machine don't
measure the same thing. Other context keys, such as `model`, `prompt` or `dataset`, are shown on the run
and marked in the run's details with their previous value when they change.

Each measurement can be compared with two baselines:

- **the run at the last release**: how far things moved since what users have. This is the default;
- **the previous run** in the series that is an ancestor in history: what this change did. Used when
  there is no run at the last release, or when you pick it on the run page.

With neither, there is no comparison, and only level thresholds can judge.

Thresholds turn the comparisons into OK, Warning and Blocking. See [Configuration](config.md#metrics).

## Where results are stored

The default is the branch `shipwake-results` in the same repository. Each run is a JSON file under
`runs/`, with its attachments under `files/<run id>/`. A directory works as well (`-r ../results`), for
example a CI artifact, a mounted volume or another repository's checkout.
