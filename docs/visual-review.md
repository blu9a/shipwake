# Reviewing how things look

Screenshots recorded with a run show what a change did to how the product looks. Each one is compared,
pixel by pixel, with the look a person approved for it. A change shows only the screenshots that look
different or are new; a person approves a new look the way they approve a scenario, and later changes
are compared with what they approved. A look nobody approved holds the commit in `shipwake check` and on
the release page.

## Recording screenshots

A screenshot takes part when its attachment is named after what it shows:

```text
look: <subject>
look: <subject> [<variant>]
```

- **subject** is the component or screen: `Split button`, `Bill row`, `Checkout page`.
- **variant** is how it was taken, its parts separated by commas: `dark`, `light, phone`,
  `dark, desktop, zh`. A subject's variants are shown together.

`look: Bill row [dark, phone]` and `look: Bill row [dark, desktop]` are two variants of one subject. The
same subject and variant in a later run is the same look. Attachments may be PNG, JPEG or GIF, on a test
or on any of its steps; every format that carries attachments works (`playwright`, `json`, and the
[converters](converters.md) that map them). Any other attachment stays an ordinary attachment.

### With Playwright

[`shipwake-playwright`](../packages/playwright/README.md) has a helper that takes the screenshot and
names it:

```ts
import { look } from 'shipwake-playwright';

test('bill rows', async ({ page }, testInfo) => {
  for (const scheme of ['light', 'dark'] as const) {
    for (const [width, size] of [[390, 'phone'], [1280, 'desktop']] as const) {
      await page.emulateMedia({ colorScheme: scheme });
      await page.setViewportSize({ width, height: 800 });
      await page.goto('/bills');
      await look(page.getByRole('listitem').first(), testInfo, 'Bill row', [scheme, size]);
    }
  }
});
```

`look(target, testInfo, subject, variant?)` screenshots a page or an element with animations stopped and
the caret hidden, so the same look gives the same pixels. `lookName(subject, variant)` gives just the name,
for screenshots you attach yourself. Record the run with the Shipwake reporter or Playwright's JSON
reporter, as usual.

### Anything else

Name the attachment by the convention in your tool's report, or in Shipwake's own format:

```json
{ "tests": [{ "name": "Bill row renders", "status": "passed",
  "attachments": [{ "name": "look: Bill row [dark, phone]", "type": "image/png", "path": "shots/bill-row-dark-phone.png" }] }] }
```

Take screenshots on one kind of machine (the same browser, fonts and device scale), like measurements
on one runner: another machine draws text differently, and that difference is real pixels.

## What a commit's screenshots are compared with

The commit's newest run of each series counts, as for its tests and measurements. Each of its screenshots
is compared with the newest approval for the same subject and variant made on this commit or on one
before it in history. So:

- after a look is approved, the commit it was approved on and every later commit are compared with it;
- a branch that left before the approval still compares with the look approved before it, until it is
  merged or rebased;
- a screenshot with no approval for its subject and variant is **new**: there is nothing to compare it
  with, and it waits for a first approval.

A screenshot is **the same** when it is the approved file, or when no pixel differs; it is **changed**
when at least one pixel differs.

## How pixels are compared

The comparison is deterministic: no model, and the same two images always give the same result.

- Each pixel's color is compared in the YIQ color space, the perceptual difference used by most visual
  testing tools, with transparent pixels laid over white. A pixel differs when the difference is more than
  **0.1** of the largest one (black against white). The threshold is fixed.
- **Anti-aliasing is tolerated**: an edge pixel shaded between its darkest and brightest neighbours, where
  one of those neighbours sits in a flat area in both images, doesn't count (V. Vyšniauskas, *Anti-aliased
  pixel and intensity slope detector*, 2009). Font smoothing that moves by a sub-pixel is tolerated this
  way; a changed color, size or position is not.
- Images of different sizes differ in every pixel outside the area both cover.

The changed pixels are drawn in red on a transparent image as large as the larger screenshot; the page
lays it over the screenshots to highlight what changed.

## Reviewing

A commit's page lists its **Looks** when its runs recorded screenshots: the subjects with a changed or new
variant first, each with all its variants together and the changed and new ones marked; the subjects that
look the same are folded away. The timeline marks a commit with looks to review.

A look's page compares the approved look and the new one:

- **Side by side**, the approved look first;
- **Slider**, one over the other, with a handle that reveals the new look from the right;
- **Onion skin**, the new look faded over the approved one.

**Highlight changes** lays the changed pixels over them. Other variants of the subject are above, to
switch between them.

## Approving

Under `shipwake serve`, **Approve this look** approves it as you, with your Git identity; **Approve all
variants** approves every changed or new variant of the subject. From the terminal:

```sh
shipwake looks                                  # HEAD's screenshots: same, changed or new
shipwake looks --commit 1a2b3c4 --json
shipwake looks approve "Bill row [dark, phone]"  # one look of HEAD
shipwake looks approve "Bill row"                # every changed or new variant of a subject
shipwake looks approve --all --commit 1a2b3c4    # every changed or new look of a commit
```

An approval is kept with the runs: a file `looks/<id>.json` on the results branch (or in the results
directory) with the subject, variant, commit, run, the screenshot's SHA-256, and who approved it and when.
On a results branch it is committed with an `Approved-by:` trailer:

```text
look: approve "Bill row [dark, phone]" on 1a2b3c4

Approved-by: Sam Rivera <sam@example.com>
```

Approving needs no branch checked out and changes nothing in the project's code. Nothing is pushed:
`shipwake looks approve --push` sends the results branch to origin when you ask for it. A published site
is static, so a look's page there gives the command instead.

## Checks and releases

`shipwake check` fails while any screenshot of the commit is changed or new, and names it:

```text
1a2b3c4, compared with the last release, else the previous run
  ✓ UI tests: 12 tests passed
  ✗ Looks: "Split button" looks different from its approved look (21279 pixels), and nobody approved the new one
  ✗ Looks: "Receipt card" is new and waits for a first approval
  ✓ Looks: 5 screenshots as approved
```

`--json` gives each as a check of kind `look` with the screenshot's name. The release page judges the
newest trunk commit the same way: one verdict.
