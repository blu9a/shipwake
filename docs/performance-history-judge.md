# History and result judgment: paired benchmark

Measured source: baseline `b40843d96b713c4bf07c94def399b91a85640b6b`, candidate
`751d0fb9b6c02c085fe7f680b6ba5fb560a43fe6`. Date: 2026-10-29 UTC.

History reads independent merge ranges with at most eight Git subprocesses, while
keeping each range's Git ordering and assembling the rows in their original order.
Judgment resolves each metric rule once per invocation, indexes context predecessors
by name/time, and searches earlier runs within their series. Baselines still use the
existing history/ancestry rules, including reruns and separate branches/runners.

| Measurement | Before: median (min–max) | After: median (min–max) | Median change |
|---|---:|---:|---:|
| history ms/op | 884.155 (859.598–901.400) | 135.483 (122.600–140.174) | -84.7% |
| history B/op | 5,442,002 (5,295,554–5,534,765) | 5,522,256 (5,455,693–5,599,378) | +1.5% |
| history allocs/op | 35,388 (35,353–35,391) | 36,308 (36,287–36,324) | +2.6% |
| judge ms/op | 354.990 (343.107–365.893) | 10.507 (10.003–11.334) | -97.0% |
| judge B/op | 64,483,785 (64,482,649–64,484,233) | 1,514,804 (1,513,756–1,514,872) | -97.7% |
| judge allocs/op | 910,533 (910,532–910,537) | 40,620 (40,619–40,621) | -95.5% |

Time is elapsed monotonic time reported by Go `testing.B`, per operation. B/op is
allocated Go heap bytes, not peak RSS; allocs/op counts heap allocations. The history
speedup adds approximately 1.5% allocated bytes and 2.6% allocations in this fixture.
Eight concurrent Git processes bound the parallelism; their peak RSS was not measured.

Protocol:

- Linux 6.8.0-138-generic, x86_64, glibc 2.35; Intel Xeon Gold 6342 @ 2.80GHz,
  96 logical CPUs; Go 1.27.3, Git 2.34.1; `GOMAXPROCS=8` in both variants.
- History: the existing deterministic `git fast-import` fixture has 300 merges,
  two branch commits per merge and a root (901 commits); Load returns 300 rows and
  900 commit headers. Setup/import is outside the `b.Loop` timer.
- Judgment: the existing benchmark has 300 trunk commits, 600 runs, two runners and
  12 measurements/run (7,200 vectors), with a release and wildcard/exact rules.
  The timed operation includes rebuilding run summaries and `Apply`, as before.
- Five paired samples, alternating before/after order. Each history sample averages
  exactly three operations, each judgment sample exactly five. The table gives the
  median and range of those five sample means, rather than a single run.
- Each benchmark process creates its own deterministic fixture. OS caches were not
  flushed; this is not a cold-disk claim. There are 5/5 complete pairs and no failed
  samples, retries or excluded observations.
- This developer host is shared, not exclusive. Load averages were about 7.7/8.9/8.4
  at the start and 6.9/8.6/8.3 at the end. The comparison is paired on this host;
  these numbers are not an end-to-end site-build speedup or a comparison with a
  different GitHub runner. Other graph shapes and CPU/storage budgets can differ.

Reproduce using the same Go/Git versions and fixture definitions:

```sh
# Run in the candidate checkout; use a separate baseline worktree.
benchmark_dir=$(mktemp -d)
git worktree add --detach "$benchmark_dir/baseline" b40843d96b713c4bf07c94def399b91a85640b6b
mkdir -p "$benchmark_dir/before" "$benchmark_dir/after"
(cd "$benchmark_dir/baseline" && go test -c -o "$benchmark_dir/before/history.test" ./internal/history)
(cd "$benchmark_dir/baseline" && go test -c -o "$benchmark_dir/before/judge.test" ./internal/judge)
go test -c -o "$benchmark_dir/after/history.test" ./internal/history
go test -c -o "$benchmark_dir/after/judge.test" ./internal/judge
python3 tools/bench-pair.py --before "$benchmark_dir/before" --after "$benchmark_dir/after" --out "$benchmark_dir/results" --repeats 5 --gomaxprocs 8
```

`before.txt` and `after.txt` are valid `go-bench` inputs. Record both with the same
name/runner and their actual commit SHA to show the change on the timeline; keep
this developer-host series separate from `runner=github-ubuntu`.

Evidence fingerprints (SHA-256):

- `before.txt`: `0522cc07d4ec305a1bd44474a691600282aab23b1e093f054ad7361db2440a3d`
- `after.txt`: `ab3911115e46034e40cbaca4ddaefd5d5cebeb9762695e473c9e190415152a4d`
- `before/history.test`: `e5ed2918f0a97eca4cbe9f06a6194f3f66284e7f12e5b83ff2a0ce376eed75ac`
- `before/judge.test`: `8a346beb316f880f33cc80d7ad44ad43bf0a29b82d55f03bfc1de56721606ac3`
- `after/history.test`: `9e829272f259da2ea7e7b15f80631add673865c36c6a16cdb2146fcf857fee0b`
- `after/judge.test`: `50fbc7c546164f1ca65111f0716c95900c2a506e7f29960a5625fafaea92e00d`
