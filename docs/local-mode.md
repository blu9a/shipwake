# Local mode

Try Shipwake on a repository without changing it. In local mode Shipwake only reads the repository; what it
adds lives in a workspace of the project's own on your machine. Deleting the workspace removes every trace.

```sh
shipwake serve        # then "Open a repository" in the project menu, and give its path
```

A repository that already has a committed `.shipwake.yaml` opens the usual way. Any other opens in local mode.

## Where things are

| | Where |
|---|---|
| History, branches, tags, the changelog | read from the repository; nothing is written there |
| `.shipwake.yaml`, `features/`, `metrics/`, converters | the workspace, `~/.shipwake/<name>/`, a Git repository of its own |
| Proposals, edits and approvals | commits in the workspace, as you, with `Approved-by` |
| Runs and their recordings | the workspace's `shipwake-results` branch (`shipwake add` finds it from the repository) |
| Every project opened | `~/.shipwake/projects.yaml`; `shipwake serve` with no repository opens them all |
| The agent skill | `~/.shipwake/skill/SKILL.md` |

`SHIPWAKE_HOME` moves `~/.shipwake` elsewhere.

**Branches.** Requirements belong to a branch. The workspace keeps a branch for each of the repository's, under
the same name, and follows the branch the repository has checked out; a new one starts from the workspace's
trunk.

**Tests.** Test code stays in the repository, as usual. Acceptance tests read the `.feature` files from the
workspace, so they run where the workspace is: on your machine, not in CI.

**Releases.** Recording a release writes a tag and the changelog, so in local mode the release page gives the
command to run in the repository rather than running it.

**Agents.** The agent icon at the top of the page gives a prompt to paste into your agent: the skill's file, the
repository, its workspace and the checked-out branch, so it writes Shipwake's files to the workspace.

## Adopting

When you're ready to keep the requirements in the repository:

```sh
shipwake adopt ~/code/app
```

It copies the workspace's files into the repository and commits them as you, in one commit whose message lists
what was approved in local mode and by whom. The workspace is kept: it has the requirements' history and the runs
recorded so far, which Shipwake keeps showing for the project.
