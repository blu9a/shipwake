# shipwake-playwright

Playwright reporter and recording helpers for [Shipwake](https://github.com/blu9a/shipwake).

Shipwake reads Playwright's own JSON reporter already. This package adds what that report doesn't have:
subtitles in step with each recording, Gherkin Background steps where they belong, and the helpers below.

```sh
npm i -D shipwake-playwright
```

```ts
// playwright.config.ts
reporter: [['list'], ['shipwake-playwright/reporter', { outputFile: 'shipwake/run.json', name: 'Web tests' }]],
use: { video: 'on' },
```

```sh
npx playwright test
shipwake add -f json --file shipwake/run.json --push
```

- **Every step**, including Gherkin Background steps that run inside hooks (playwright-bdd).
- **Attachments as files** next to the report, so Shipwake can keep them with the run.
- **`decisive(locator, testInfo, step, expected)`** in a result step records what was checked and the text actually seen, shown as "Observed" under the scenario's result.
- **`look(target, testInfo, subject, variant?)`** screenshots a page or an element for [visual review](../../docs/visual-review.md),
  named `look: <subject> [<variant>]`: `await look(row, testInfo, 'Bill row', ['dark', 'phone'])`. Shipwake compares it
  with the look approved for that subject and variant. `lookName(subject, variant)` gives the name alone.
- **`showPointer(page)` and `slowClicks(page)`** make recordings easy to follow: a white pointer that reaches each target before it clicks.
