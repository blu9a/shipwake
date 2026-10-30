# @shipwake/playwright

Playwright reporter and recording helpers for [Shipwake](../../README.md).

```ts
// playwright.config.ts
reporter: [['list'], ['@shipwake/playwright/reporter', { outputFile: 'shipwake/run.json', name: 'Web tests' }]],
use: { video: 'on' },
```

```sh
npx playwright test
shipwake add -f json --file shipwake/run.json --push
```

- **Every step**, including Gherkin Background steps that run inside hooks (playwright-bdd).
- **Attachments as files** next to the report, so Shipwake can keep them with the run.
- **`decisive(locator, testInfo, step, expected)`** in a result step records what was checked and the text actually seen, shown as "Observed" under the scenario's result.
- **`showPointer(page)` and `slowClicks(page)`** make recordings easy to follow: a white pointer that reaches each target before it clicks.
