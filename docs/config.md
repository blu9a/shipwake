# Configuration

`.shipwake.yaml` at the repository root. Every setting is optional; without the file, the project is
named after its directory, the trunk is `main` and everything is in English.

```yaml
name: Shipwake
trunk: main

languages: [en, zh]            # release notes and project text; the first is required
translations:
  zh: .shipwake/zh.yaml        # "original text": "translation"

targets:                       # display names for where tests ran
  desktop: { en: Web desktop, zh: 网页桌面版 }
  phone:   { en: Web phone,   zh: 网页手机版 }

metrics:
  quality:        { direction: higher, warn: 0.88, block: 0.82 }
  cost:           { warn: 0.022, block: 0.03 }
  "api.*.p95":    { warn: "+15% and +20ms", block: "+40% or +200ms" }
  "api.*.rps":    { direction: higher, warn: "-10%", block: "-25%" }

kinds:                         # your own words for kinds of runs: icon and names
  eval:      { icon: gauge, zh: 评测 }
  load test: { icon: bolt,  zh: 压测 }

results: git:shipwake-results  # or a directory

changelog: CHANGELOG.md        # the default; a map of language to file, or none (notes in tags only)
```

## Metrics

Settings by measurement ID. A key with `*` is a pattern where `*` stands for one dot-separated part, so
`api.*.p95` matches `api.plans.p95`; an exact ID wins over a pattern.

**Direction** says which way is better: `higher` (quality, throughput, hit rate) or `lower` (latency,
cost, errors). Tools usually record it (`direction` in each metric); the setting here wins, for tools
that get it wrong or don't say. Changes are colored by it, and thresholds are read by it.

**Thresholds** (`warn`, `block`) are conditions joined by `and` / `or`; `and` binds first.

| Condition | Meaning |
|---|---|
| `0.88` | a level, read in the direction: below it for `higher` is better, above it for `lower` |
| `< 0.82`, `> 4s` | a level, stated explicitly; a unit converts (`4s` on a metric in ms) |
| `+20ms`, `-0.03` | a change by this amount from the baseline. Without a unit, in the metric's own unit: `+0.5` on a metric in `%` is half a point |
| `+15%`, `-10%` | a change by this percentage of the baseline |
| `±15%`, `±5` | a change of this size in either direction |

Combining avoids false alarms and catches more:

- `+15% and +20ms` holds only when a change is large both relatively and absolutely: 2 → 3 ms
  (+50%) and 2000 → 2020 ms (+1%) both pass.
- `+40% or +200ms` catches a large jump on a fast operation and on a slow one.
- `< 0.82 or -5%` blocks when quality is low, or when it fell sharply while still above the level.

Units convert within time (`ns` to `h`) and bytes (`B`, `KiB`… and `KB`…). Use changes for numbers
that depend on the machine, such as latency, throughput and benchmarks, and levels for quality, error
rates and costs.

Changes need a baseline. Measurements are compared with the run at the last release; a run without
one is compared with the previous run of its series, and a run with neither is judged on its levels
only. A threshold that could only be judged in part says so (`shipwake check`, the run page).

Measurements without thresholds are shown with their change and trend but no verdict.

## Kinds

A run's kind is your own word, given with `--kind`: `test`, `eval`, `load test`, `benchmark`,
`security scan`… It only sets the icon; every kind is measured and judged the same way. Icons: `test`,
`gauge`, `bolt`, `timer`, `shield`, `chart`. Without one, a known kind gets its usual icon and others get
`test` or `chart` by what the run holds.

A run holding both tests and measurements, such as an eval with cases and a score, shows as two rows on the
timeline, each with its own counts. Recording them as two runs works just as well.

Thresholds are a decision about the product, so they belong in review like code: change them in a pull
request, not to get a pass.

## Translations

Project text such as feature, scenario and step names, run names and suite names comes from your code and
results. A translation file maps the original text to another language:

```yaml
"I decide on every suggestion": 我决定每一条建议
"Accepting a suggestion applies it": 接受建议后立即生效
```

Missing entries fall back to the original. An agent can draft the file from the `.feature` files; a person
should read it before it is committed.

## Scenarios

`.feature` files are recognized wherever they are. For every commit Shipwake shows which scenarios were
added, changed or removed, compared with the commit's first parent, alongside the changed files.
