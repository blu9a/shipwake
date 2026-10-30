---
name: shipwake
description: Set up and use Shipwake in a repository. Use it to record test, eval, performance or benchmark results with the commit they tested, wire Shipwake into CI, convert a tool's output to Shipwake's run format, propose scenarios, metrics and thresholds for the person to approve, translate project text, explain what a change did to tests and measurements, or draft release notes.
---

# Shipwake

Shipwake turns Git history plus result files into a static site: the trunk as a timeline of merges,
commits, releases and the runs on them. Results are stored on the orphan branch `shipwake-results`.
Verdicts are deterministic. Your job is around them: setup, converters, drafts and explanations. You never
decide a verdict.

Commands: `shipwake init | add | build | serve | check | doctor | formats | schema`. Run
`shipwake <command> --help` for flags. Docs: `docs/` in the Shipwake repository.

## Local mode

If the person's prompt names a workspace, the project is in local mode: Shipwake writes nothing into the
repository. Write `.shipwake.yaml`, `.feature` and `.metric` files and converters in the workspace, on the branch
named like the repository's checked-out one, and commit them there; never add them to the repository. Test code
(step definitions, test config) goes in the repository as usual, reading features from the workspace's path.
Record runs with `shipwake add` from the repository as usual: it stores them in the workspace. See
`docs/local-mode.md`.

## Adopt Shipwake in a repository

1. Run `shipwake init` and read what it found. It changes nothing.
2. For each test command in CI, add a step after it that records the output, even when the tests fail:
   `shipwake add -f <format> --file <file> -n "<name>" --context runner=<ci-name>`.
   On GitHub, use the action `blu9a/shipwake@v0` with `format`, `file`, `name`, `context`.
   Shipwake never pushes on its own. To keep results across jobs, the results branch must be pushed
   (`--push`, or `push: true` with `contents: write`): explain this and let the person decide; don't add it
   for them.
3. Choose run names people understand ("Web acceptance tests", not "e2e-job-3"). Keep each name stable:
   runs with the same name form a series.
4. Run `shipwake doctor`, then `shipwake serve`, and check that the timeline looks right.
5. Offer `shipwake init --write` for a `.shipwake.yaml`; fill in only what the person wants.

Don't change test code to adopt Shipwake. If a runner can't write a supported format, write a converter
(below) instead.

## Convert a tool's output

When `shipwake formats` has no reader for a tool, write a converter: a YAML description of the tool's
report in `.shipwake/formats/<format>.yaml`, interpreted by Shipwake (see `docs/converters.md`). Don't
write a script when a converter can say it; people trust what they can read in review.

1. Run the tool and keep its report as `.shipwake/formats/samples/<tool>.<ext>`. Use a real run with
   passed, failed and skipped tests if you can. Remove anything private from it (absolute paths,
   hostnames, tokens).
2. Write the converter: `each` path to the tests, `name`, `status` (or `steps`), `duration` with its unit,
   `error`. Always add `totals` from the tool's own summary when the report has one: it is what stops a
   wrong conversion from being recorded later.
3. `shipwake formats test --update`, then read the expected run and compare it with the tool's own
   output: every test there, names readable, statuses right, durations in milliseconds. Fix until right.
4. `shipwake add -f <format> --file <report> -n "<name>" --dry-run` to show the person what CI will
   record. Commit the converter, sample and expected run together.

Only when a converter can't express it, write a script that outputs Shipwake's own format and record
it with `-f json`. Get the schema with `shipwake schema`; the `run` property is the format. Minimal shapes:

```json
{"name": "Agent evals", "kind": "eval", "context": {"model": "…", "dataset": "…", "runner": "…"},
 "metrics": [{"id": "quality", "suite": "Agent quality", "name": "Suggestion quality",
              "unit": "score", "direction": "higher", "values": [0.91, 0.89]}]}
```

```json
{"name": "Device tests", "tests": [{"feature": "Checkout", "name": "Pays with a saved card",
  "target": "iPhone", "status": "passed", "duration": 1840}]}
```

Rules:
- `id` is stable across runs; changing it starts a new series. Use dots for structure (`api.plans.p95`)
  so thresholds can use patterns.
- `values` are samples of the same measurement (repetitions), not different measurements.
- `direction`: `higher` when bigger is better (quality, throughput), else `lower`.
  When a tool records it wrongly or not at all, set it per metric under `metrics` in `.shipwake.yaml`.
- `kind` is the project's own word (`eval`, `load test`, `benchmark`…); it only picks the icon. Keep it
  consistent within a project.
- `duration` is in milliseconds. `status` is `passed`, `failed` or `skipped`.
- Put what the run depended on in `context`: `runner`, and for evals `model`, `prompt`, `dataset`.
  Changes in context are shown on the timeline.
- Keep a script converter in the repository too, with a test using a real output sample.

## Propose scenarios

Scenarios are how the person controls what gets built: describe the product's outward behavior, then build
only what they approve.

- Write Gherkin scenarios in `.feature` files, each with `@proposed` on the line above `Scenario:`. Describe what
  a user does and sees, in their words. Don't pin details nobody chose (a timeout, a retry count, a rate limit)
  unless the person asked for them.
- For a project adopting Shipwake, propose scenarios for what the product already does, so later changes are
  checked against them.
- Commit the proposal as an ordinary commit. The work is the person's: no agent attribution, no co-author, no
  trailers naming you.
- Test runners must skip proposals: playwright-bdd `tags: 'not @proposed'`, Cucumber `--tags 'not @proposed'`.
- The person changes and approves proposals in Shipwake (`shipwake serve`, at the top of the timeline). Never
  remove `@proposed` yourself, and don't implement a scenario until it is approved. To change an approved
  scenario, propose the change; don't edit it silently.

## Propose a metric

Where outputs vary (agent features, ranking, performance), a requirement is a number. Write it as a `.metric`
file, `@proposed` on its first line; the file name is the metric's ID, as the evaluation reports it
(`metrics/totals.exact.metric` for `totals.exact`):

```text
@proposed
Metric: Exact totals
  The share of bills whose shares add up to the bill plus tip, rounded half up to the cent.

  Measured on evals/bills.jsonl
  Each case scores 1 when the shares add up exactly, else 0
  Higher is better
  Blocks a release below 100%
```

Settings are fixed sentences, one per line: `Measured on <path>`, `Each case scores …`, `Higher is better` or
`Lower is better`, `Blocks a release below|above <value>`, `Warns below|above <value>`, and `Blocks a release when
it gets 10% worse than the baseline` (or `Warns when …`). A line that fits none is an error. Commit the dataset with
the proposal, and explain how it was chosen. The thresholds are the person's decision, as below. After approval,
write the evaluation: it records a run in Shipwake's format with the metric and one `cases` entry per dataset item
(`id`, `input`, `output`, `score` from 0 to 1, `reason`).

## Propose thresholds

Thresholds (`metrics` in `.shipwake.yaml`, with `direction`, `warn`, `block`) are product decisions. Propose them, explain each from the recorded
history (`shipwake check --json`, run files on the results branch), and let the person approve them. Label
your numbers as proposed until then.

- Machine-dependent numbers (latency, throughput, benchmarks): changes, such as
  `{warn: "+15% and +20ms", block: "+40% or +200ms"}`; the `and` ignores big percentages of tiny numbers.
- Quality scores, error rates and costs: levels, optionally with a change: `"< 0.82 or -5%"`.
- Conditions: `0.88` / `< 0.88` (level), `+20ms` (amount), `+15%` (percentage), `±` for either way,
  joined by `and` / `or`. `shipwake doctor` checks the syntax.
- Look at the spread of past values first; a limit inside normal noise produces false alarms.

Never loosen a threshold, delete a run or rename a series to make a check pass.

## Explain what a change did

```sh
shipwake check --commit <sha> --against previous --json
```

Each check has the run ID (`runs/<id>.json` on the results branch), and for a measurement its `metric` ID and
the `baseline` run it was compared with. For a failing test, read the run file: `tests[].error`,
`steps`, and `evidence`, which holds what a scenario's result step expected and what was actually seen.
Say which commits between the baseline and this run could explain it (`git log <base>..<sha>`), and whether
the run's context changed (another model or runner), which is often the real cause.

## Translate project text

With `languages: [en, zh]`, project text (features, scenarios, steps, run and suite names) is translated by
files listed under `translations`, mapping the original text to the translation. Draft the file from the
`.feature` files and run names; keep the product's own terms; leave out what should stay as it is. A person
reviews the file before it is committed.

## Draft release notes

Release notes go in the annotated tag (see `docs/releases.md`). To draft them:

1. Collect `Release-note:` and `Release-note-<lang>:` trailers since the last tag, and the scenarios added or
   changed (`git diff <last-tag>..HEAD -- '*.feature'`).
2. Write for people using the product: what they can do now, what works differently, what was fixed.
   Leave out refactors, tooling and internal names.
3. Write each configured language as a section after a `--- <code>` line, first language first.
4. Hand the draft to the person. Releasing is their decision; don't create or push the tag yourself
   unless asked.
