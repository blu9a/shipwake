# The requirements loop

## The loop

1. Scenarios are proposed for a feature (usually drafted by an agent).
2. A person changes them: rewords, removes, adds.
3. The person approves them.
4. The scenarios are implemented; when they pass, a demonstration video of them appears.
5. Later changes start the loop again, and each iteration gets its own demonstration.

Shipwake should show this as it happened, from Git and the recorded runs only.

## What we learned first

Scenarios often go through exactly this loop with nothing to show for it in Git: one commit ("first
version of the scenarios"), because the proposal and the person's changes happened in chat. **Each stage must leave a trace
in Git**, or no tool can show the iteration.

## Questions

1. **Traces.** What does each stage leave in Git, using only commits, trailers and tags, so it works
   on any host and in any language?
2. **Derivation.** Can Shipwake rebuild each feature's story (proposed → changed → approved →
   passing → demo) from those traces and the recorded runs alone?
3. **The person's step.** Where does the person change the proposal: in the editor, in a pull request
   review, in chat? What is the least effort that still leaves a trace?
4. **Demonstrations.** Can every passing iteration produce its video automatically, and can two
   iterations be compared?
5. **Footprint.** How much of this must the project adopt?

## Conventions to try

The work belongs to the person responsible for it, whoever drafted it: commits keep the person's
authorship and nothing names an agent. The stages come from the scenarios' state.

| Stage | Trace |
|---|---|
| Proposed | scenarios tagged `@proposed` in a commit |
| Changed | later commits changing them while proposed |
| Approved | the commit removing `@proposed`, with `Approved-by: <person>` |
| Implemented | the first recorded run where the scenario's test passes |
| Demonstrated | that run's recording, per scenario version |

Only committed scenarios count: a proposal is what the branch's last commit says, the same thing a
published site shows. Text only in a working tree is nobody's proposal yet, and the workspace won't
save over a file with uncommitted changes.

## The person's step

In Shipwake, not in chat: `shipwake serve` is the workspace. Scenarios and metrics not yet approved wait in a
dashed box at the top of the timeline, editable in place; saving and approving are the person's commits.
A published site shows the same box read-only.

## Sandbox

A small separate project with a web page, Gherkin scenarios run by Playwright (playwright-bdd) and
recorded with Shipwake's reporter. The loop runs on it for real.

## To decide: tests that serve the product

Agents write tests by the hundred. Many pin details nobody chose (a retry count, a rate limit, a timeout), and
those then constrain the next agent, which can't tell a deliberate value from an accidental one. With quality
anchored in approved scenarios and metrics, such tests could be cleaned up while every feature still works.

What Shipwake could do:

- **Show which tests serve the product.** A test serves a requirement when it is one of the approved
  scenarios, or an evaluation of an approved metric. Every other test supports the code: useful, but nobody
  agreed to what it pins. The run page and the timeline could show the two apart.
- **Let an agent propose a cleanup.** Removing or loosening supporting tests becomes a proposal like any
  other: the tests to remove, why each pins no agreed behavior, and the evidence that every approved scenario
  and metric still passes without them. The person approves it in Shipwake; nothing is deleted on its own.
- **Protect what was agreed.** Approved scenarios and metrics are never weakened, skipped or removed without the
  person's approval (`shipwake check` could fail such a commit). Supporting tests have no such protection.

Open: how a unit test is tied to a requirement when it isn't a scenario (a tag, a naming convention, coverage of
the scenario's code), and whether "supporting" tests should show at all on a published site.
