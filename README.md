# Shipwake

**Check what your product does, not every line your agents wrote.**

[![CI](https://github.com/blu9a/shipwake/actions/workflows/ci.yml/badge.svg)](https://github.com/blu9a/shipwake/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/blu9a/shipwake?include_prereleases&sort=semver)](https://github.com/blu9a/shipwake/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

**[See it live: Shipwake's own site](https://blu9a.github.io/shipwake/)**, built with Shipwake from this
repository's history and CI runs.

With agents, code is written faster than anyone can review it. Reading every diff no longer scales, least of all
for a one-person company. Yet quality was never really about the code: it is about what the product does for the
people who use it. Shipwake moves quality control there: to the features you agreed on, what they do, and how
well they do it.

> Status: early. It works end to end; formats and commands may still change.

## What you get

- **Features, not diffs.** Scenarios describe how the product behaves, in plain sentences. An agent proposes them,
  you change and approve them in Shipwake, and every change then shows which scenarios it added, changed or broke,
  with its diff alongside when you want it.
- **Watch it instead of reading it.** One click plays a feature's scenarios as one video, with subtitles for each
  step. In a minute you see whether the agent built more than you asked for, or something that doesn't behave the
  way you expected.
- **Tests that serve the product.** Agents write tests by the hundred, and many pin details nobody chose, like a
  retry count or a rate limit, which then constrain the next agent. With quality anchored in approved features,
  tests like that can be cleaned up while every feature still works.
- **Measured where outputs vary.** Agent features and other non-deterministic behavior need numbers, not pass or
  fail. Metrics are written as sentences ("Blocks a release below 100%") and approved like scenarios; evaluations
  show each case before and now, so quality doesn't drift from release to release.
- **Releases that say what changed for the product.** A release lists the requirements it added or changed and the
  changes it brings, checked before you approve it, in `CHANGELOG.md` and an annotated tag with who approved it.
- **Rebuildable.** Approved features with their recordings describe the product well enough to rebuild it.

It is a methodology with a dashboard: your agent sets it up and does the work; you decide in Shipwake.

## How it works

1. **Propose.** An agent drafts scenarios (`.feature` files) or a metric (a `.metric` file), marked as proposed.
2. **Approve.** You read, change and approve them on the timeline, where they wait at the top. Each step is a
   commit of yours.
3. **Build and check.** The agent implements; your CI records what ran: tests with their recordings, benchmarks,
   evaluations. Each change is judged against what you approved, the previous run and the last release.
4. **Release.** The release page checks the newest commit, drafts the changelog, and records the release when you
   approve it.

Verdicts are deterministic: pass, fail and thresholds never depend on a model. Everything lives in Git: no server,
no account, no hosting platform required.

## Install

```sh
go install github.com/blu9a/shipwake/cmd/shipwake@latest
```

Or download a binary for Linux, macOS or Windows from [Releases](https://github.com/blu9a/shipwake/releases).
In CI, the GitHub Action installs it for you.

## Start

**Ask your agent.** Give it the [Shipwake skill](skill/shipwake/SKILL.md) and ask it to set up Shipwake in your
repository. Integration is the agent's job: it finds your test runners, records their results (writing a small
converter when a format isn't supported yet), and proposes scenarios for what your product already does, for you
to approve.

Or by hand:

```sh
shipwake init        # what it finds in your repository, and the commands for CI (changes nothing)
go test -json ./... > go-test.json
shipwake add -f go-test --file go-test.json -n "Go tests"
shipwake serve       # http://127.0.0.1:7070
```

JUnit XML covers most languages; `go test`, Playwright, Cucumber, Jest, pytest, Go benchmarks and k6 are read
directly. Any other tool's report takes a [converter](docs/converters.md): a few reviewed lines of YAML, which your
agent can write and test against a sample. Evaluation scores and anything else fit Shipwake's own JSON format.

### What it changes in your repository

| What | When |
|---|---|
| A branch `shipwake-results`, sharing no history with your code | when you record the first run (or keep runs in a directory) |
| `.shipwake.yaml` (optional) | when you want names, languages or thresholds |
| A step after each test command in CI | when you add it |
| `.feature` and `.metric` files | when you adopt scenarios and metrics |
| `CHANGELOG.md`, a `chore(release)` commit and a tag | when you release from Shipwake |

Your code, build and tests are untouched. Shipwake never pushes on its own: in CI, keep runs by sending the results
branch to origin, with `--push` or the GitHub Action's `push: true`:

```yaml
- uses: blu9a/shipwake@v0
  if: always()
  with: { format: junit, file: report.xml, name: Unit tests, context: runner=github-ubuntu, push: true }
```

To remove it, delete that branch, the CI steps and the config. Nothing else depends on it.

### Your first day

The first build shows your history straight away: the trunk as merges and commits, releases with their notes (from
`CHANGELOG.md` or tag messages), branches, and each commit's files, diffs and scenario changes. Your agent can
propose scenarios for what the product already does; once you approve them, every change is checked against them.

Results start with the first run you record. Most projects keep none: CI reads pass or fail and its logs expire. So
on day one each suite has a single run, with no trend yet. Absolute thresholds (`block: 0.82`, `< 2ms`) judge from
that first run; thresholds relative to a baseline (`+15%`) judge once there is a run to compare with.

Trends fill in as CI records each change. To have them now, record your recent releases once: see
[Getting started](docs/getting-started.md#trends-from-day-one).

## Keeping it running

- **Nothing to operate.** `shipwake build -o site` writes a static folder; publish it anywhere, for example on
  GitHub Pages from CI. `shipwake check` gates merges and releases (exit 1 when something blocks).
- **Upgrading.** Update the binary, or the version of the GitHub Action you use. `shipwake doctor` checks a setup.
  Until 1.0, formats may still change; each release's notes list breaking changes first.

## Docs

- [Getting started](docs/getting-started.md)
- [Recording runs](docs/recording.md): formats, Shipwake's JSON format, context and comparisons
- [Converters](docs/converters.md): any other tool, described in a few reviewed lines of YAML
- [Configuration](docs/config.md): names, languages, thresholds, translations, the changelog
- [CI](docs/ci.md): GitHub Actions, GitLab, others; publishing the site
- [Releases](docs/releases.md): the changelog, notes in tags, approval
- [Playwright reporter](packages/playwright/README.md): steps, screenshots, recordings and what was observed
- [Agent skill](skill/shipwake/SKILL.md): setting Shipwake up, writing converters, drafting release notes

## Contributing

How to build, test and propose changes, and the loop this repository follows itself: [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache-2.0.

---

<details><summary>中文</summary>

**检查你的产品做了什么，而不是逐行审阅智能体写下的每一行代码。**

有了智能体，代码的产出速度已经远远超过任何人能审阅的速度。逐个阅读 diff 已经跟不上，对一人公司来说更是如此。而质量本来就不在代码本身，而在产品为使用它的人做了什么。Shipwake（航迹）把质量控制放回到这里：你们约定好的功能、它们的行为，以及它们做得有多好。

> 状态：早期。已经端到端可用；格式和命令仍可能变化。

## 你会得到什么

- **看功能，而不是看 diff。** 场景用平实的句子描述产品的行为。智能体提出场景，你在 Shipwake 里修改并批准；之后每次变更都会显示它新增、修改或破坏了哪些场景，需要时 diff 也在旁边。
- **看演示，而不是读代码。** 一键把一个功能的所有场景连成一段视频播放，每一步都有字幕。一分钟就能看出智能体是不是做多了，或者行为是否和你预期的不一样。
- **为产品服务的测试。** 智能体会成百上千地写测试，其中很多锁定了没人决定过的细节，比如重试次数或限流参数，这些又会束缚之后的智能体。当质量以已批准的功能为锚点，这类测试就可以在所有功能仍然正常的前提下被清理。
- **在输出不确定的地方用度量。** 智能体功能和其他不确定的行为需要数字，而不只是通过或失败。指标用句子来写（“低于 100% 时阻止发布”），像场景一样被批准；评测展示每个用例之前和现在的结果，让质量不会随着版本悄悄下滑。
- **说清楚产品变了什么的发布。** 一个发布会列出它新增或修改的需求和带来的变更，在你批准前经过检查，记录在 `CHANGELOG.md` 和带批准人的附注标签里。
- **可以重建。** 已批准的功能和它们的录屏足以描述产品，可以据此重建。

它是一套方法论加一个仪表盘：智能体负责接入和干活，你在 Shipwake 里做决定。

## 工作方式

1. **提出。** 智能体起草场景（`.feature` 文件）或指标（`.metric` 文件），标记为提议。
2. **批准。** 你在时间线顶部阅读、修改并批准它们，每一步都是你的提交。
3. **构建和检查。** 智能体实现；CI 记录运行结果：带录屏的测试、基准测试、评测。每次变更都与你批准的内容、上一次运行和上一个发布对比判定。
4. **发布。** 发布页检查最新的提交，起草变更日志，你批准后记录发布。

判定是确定的：通过、失败和阈值从不依赖模型。一切都在 Git 里：不需要服务器、账号或托管平台。

## 开始使用

**安装：** `go install github.com/blu9a/shipwake/cmd/shipwake@latest`，或从 [Releases](https://github.com/blu9a/shipwake/releases) 下载 Linux、macOS、Windows 的二进制文件；在 CI 中由 GitHub Action 安装。在线示例：[Shipwake 自己的网站](https://blu9a.github.io/shipwake/)。

**交给你的智能体。** 把 [Shipwake 技能](skill/shipwake/SKILL.md)交给它，让它在你的仓库里接入 Shipwake。接入是智能体的工作：找到你的测试工具，记录它们的结果（格式还不支持时写一个小转换器），并为产品现有的行为提出场景，供你批准。也可以按英文部分的命令手动开始。

**它会改动仓库里的什么：** 记录第一次运行时，一个与代码不共享历史的 `shipwake-results` 分支；可选的 `.shipwake.yaml`；CI 里每个测试命令后的一步；采用场景和指标时的 `.feature`、`.metric` 文件；从 Shipwake 发布时的 `CHANGELOG.md`、一个 `chore(release)` 提交和标签。你的代码、构建和测试都不会被改动。Shipwake 从不主动推送；要在 CI 中保留运行结果，用 `--push` 或 GitHub Action 的 `push: true`。移除它只需删除那个分支、CI 步骤和配置。

**第一天：** 第一次构建就能看到全部历史（提交、合并、发布、分支、每个提交的改动）。智能体可以为产品现有的行为提出场景，批准之后每次变更都会以它们为准。结果从第一次记录开始积累；想马上看到趋势，可以在最近的几个版本上各运行一次并记录（见英文部分的 “Your first day”）。

## 后续维护

没有需要运维的服务：`shipwake build -o site` 生成静态网站，发布到任何地方；`shipwake check` 用来把关合并和发布。升级时更新二进制或 GitHub Action 的版本；`shipwake doctor` 检查配置。1.0 之前格式仍可能变化，每个版本的说明会把破坏性变更列在最前面。

参与贡献：见 [CONTRIBUTING.md](CONTRIBUTING.md)。许可证：Apache-2.0。

</details>
