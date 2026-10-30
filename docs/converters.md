# Converters

Shipwake reads many tools' output directly (`shipwake formats`). For any other tool, a **converter**
describes the tool's report: where the tests are and which fields hold their name, status and time.
A converter is a short YAML file in your repository. Shipwake interprets it; nothing in it runs, so it
can be reviewed like any other configuration.

```text
.shipwake/formats/
  mocha-json.yaml              the converter
  samples/mocha.json           a real report from the tool
  samples/mocha.expected.json  the run it converts to, reviewed by a person
```

Use it like a built-in format: `shipwake add -f mocha-json --file mocha.json -n "Unit tests"`.

## An example

```yaml
# Mocha's JSON reporter: npx mocha --reporter json > mocha.json
format: mocha-json
about: Mocha (--reporter json)
input: json
start: stats.start
tests: # Mocha lists passed, failed and pending tests separately
  - { each: "passes[]", feature: "file | base", name: fullTitle, status: "=passed", duration: duration }
  - { each: "failures[]", feature: "file | base", name: fullTitle, status: "=failed", duration: duration, error: err.message }
  - { each: "pending[]", feature: "file | base", name: fullTitle, status: "=skipped" }
totals:
  total: stats.tests
  passed: stats.passes
  failed: stats.failures
  skipped: stats.pending
sample: samples/mocha.json
expected: samples/mocha.expected.json
```

## Why you can rely on one

- **It agrees with the tool.** `totals` names the tool's own counts in the report. Every conversion
  compares them with what it read; if they disagree, nothing is recorded and the command says why.
  A tool that changes its output can't make Shipwake record something wrong.
- **It is pinned to real output.** `sample` is a report the tool actually wrote; `expected` is the run
  it converts to, which a person reviewed. `shipwake formats test` and `shipwake doctor` convert the
  sample again and compare.
- **You see before you record.** `shipwake add … --dry-run` prints what would be recorded and stops.
- **It runs nothing.** A converter only names fields. It can't run commands, read other files or
  reach the network.

## Writing one

1. Run the tool once and keep its report as the sample.
2. Write the converter. (An agent with the [Shipwake skill](../skill/shipwake/SKILL.md) can draft
   it from the sample.)
3. `shipwake formats test --update` writes the expected run. Read it: names, statuses, durations,
   errors. Fix the converter until it is right.
4. Commit all three files. From then on `shipwake formats test` (or `doctor`) checks them.

## Reference

| Key | |
|---|---|
| `format` | the name used with `-f` |
| `about` | one line: the tool and the option that writes the report |
| `input` | `json` (default), `jsonl` (one object per line) or `xml` |
| `kind` | the run's kind (default `test`, or `measurement` with only metrics) |
| `name`, `start` | the run's name and start time, when the report has them |
| `tests` | where tests are; a list when the report keeps them in several places |
| `tests.each` | the path to every test |
| `tests.where` | keep only some: `type = scenario`, `type != background` |
| `tests.feature`, `name`, `target` | how tests group, what they're called, where they ran |
| `tests.status` | the status; common words are understood, `status_map` maps others |
| `tests.duration`, `duration_unit` | `ms` (default), `s`, `us` or `ns`; `hh:mm:ss.fff` is read too |
| `tests.start`, `error` | when it started; why it failed (kept only for failed tests) |
| `tests.steps` | the test's steps, such as Gherkin steps: `each`, `name`, `status`, `duration` |
| `metrics` | measurements: `each`, `id`, `name`, `suite`, `unit`, `direction`, `value` |
| `totals` | the tool's own counts: `total`, `passed`, `failed`, `skipped` |
| `sample`, `expected` | the real report and its reviewed run, next to the converter |

**Paths** are dot-separated keys: `stats.passes`. `name[]` visits every element of an array (in a
field, it collects them: `steps[].result.error_message`), `name[2]` one element, `*` every value of an
object, with its key as `@key`. `^` reaches the enclosing element (`^.name` is a test's file), `$` the
top of the report. A step that meets an array takes its first element, which is how XML children are
read; in XML, attributes are `@name` and text is `#text`.

**Values** can be `=text` (that text), a template (`"{keyword}{name}"`), or a path with filters:
`nodeid | after(::)`. Filters: `before(s)`, `after(s)`, `last(s)`, `join(s)`, `base` (the last part of a
file path), `trim`. `a || b` takes `b` when `a` is empty. Totals may add fields:
`numPendingTests + numTodoTests`; a count the report leaves out is zero.

## Built-in converters

Shipwake's own converters use the same format, with real reports as samples, in
`internal/readers/formats`: `cucumber-json` (any Gherkin runner), `jest-json` (Jest and Vitest) and
`pytest-json` (pytest-json-report). A converter that proves itself in projects can become one: send
it with its sample.
