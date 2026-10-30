import { test, expect } from '@playwright/test';
import { fmtValue } from '../src/format.ts';
import { runs } from './fixture.mjs';
import { auditLayout } from './layout-audit.mjs';

test('binary storage scales consistently through EiB', () => {
  for (const [value, unit, expected] of [
    [0, 'B', '0 B'], [1, 'B', '1 B'], [1023, 'B', '1023 B'],
    [1024, 'B', '1.0 KiB'], [1024 ** 2, 'B', '1.0 MiB'],
    [62943 * 1024, 'B', '61.5 MiB'], [62943, 'KiB', '61.5 MiB'],
    [1024, 'MiB', '1.0 GiB'], [1024, 'GiB', '1.0 TiB'],
    [1024, 'TiB', '1.0 PiB'], [1024, 'PiB', '1.0 EiB'],
    [0.5, 'MiB', '0.5 MiB'], [-62943, 'KiB', '-61.5 MiB'], [0.02, 'USD', '0.02 USD'],
  ]) expect(fmtValue(value, unit)).toBe(expected);
});

async function withComparisons(page, previousStatus, releaseStatus, lang = 'en') {
  await page.addInitScript((lang) => {
    localStorage.setItem('lang', lang);
    localStorage.setItem('baseline', 'previous');
  }, lang);
  const run = structuredClone(runs.current);
  run.metrics = run.metrics.map((m, i) => ({
    ...m,
    ...(i === 0 ? { name: 'Memory per operation', unit: 'B', values: [62943 * 1024] } : {}),
    previous: { ...m.previous, status: previousStatus },
    release: { ...m.release, status: releaseStatus },
  }));
  await page.route('**/data/fixture/runs/current.json', (route) => route.fulfill({ json: run }));
  await page.goto('/#/fixture/run/current');
  await expect(page.getByRole('heading', { name: 'API load', exact: true })).toBeVisible();
  return page.locator('section:has(> dl.facts)');
}

test('an unjudged benchmark keeps measurements and hides the three zero verdicts', async ({ page }) => {
  const overview = await withComparisons(page, 'none', 'none');
  for (const width of [320, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const label of ['Blocking', 'Warning', 'OK']) {
      await expect(overview.getByText(label, { exact: true })).toHaveCount(0);
    }
    await expect(page.getByRole('link', { name: /Memory per operation/ })).toContainText('61.5 MiB');
    const gap = await overview.evaluate((el) => {
      const style = getComputedStyle(el);
      return el.getBoundingClientRect().right - Number.parseFloat(style.paddingRight)
        - Number.parseFloat(style.borderRightWidth) - el.querySelector('dl').getBoundingClientRect().right;
    });
    expect(Math.abs(gap), 'Metadata must reclaim the unused stats column').toBeLessThan(1);
    expect((await page.evaluate(auditLayout)).issues).toEqual([]);
  }
});

test('the chosen baseline controls verdict visibility, including all-OK results', async ({ page }) => {
  const overview = await withComparisons(page, 'none', 'ok');
  await expect(overview.getByText('OK', { exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'v1.0.0', exact: true }).click();
  await expect(overview.getByText('OK', { exact: true })).toBeVisible();
  await expect(overview.getByText('4', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Previous run', exact: true }).click();
  await expect(overview.getByText('OK', { exact: true })).toHaveCount(0);
});


for (const lang of ['en', 'zh']) {
  test(`${lang}: verdict boxes stay in one row when their content fits, independent of their maximum width`, async ({ page }) => {
    const overview = await withComparisons(page, 'ok', 'block', lang);
    const tiles = overview.locator(':scope > div > div');
    await expect(tiles).toHaveCount(3);
    for (const rootSize of [16, 20]) {
      await page.evaluate((size) => { document.documentElement.style.fontSize = `${size}px`; }, rootSize);
      for (const width of lang === 'zh' ? [320, 360, 390, 412, 768, 1440] : [390, 412, 768, 1440]) {
        await page.setViewportSize({ width, height: 900 });
        const boxes = await tiles.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().toJSON()));
        expect(Math.max(...boxes.map((b) => b.y)) - Math.min(...boxes.map((b) => b.y)), 'All three verdicts fit together').toBeLessThan(1);
        expect(Math.max(...boxes.map((b) => b.width)), 'Wide screens retain the existing maximum box width').toBeLessThanOrEqual(rootSize * 11 + 1);
        expect((await page.evaluate(auditLayout)).issues).toEqual([]);
      }
    }
    // When the text really needs more room, wrap boxes instead of clipping their labels/counts.
    await page.setViewportSize({ width: 320, height: 900 });
    await tiles.evaluateAll((els) => {
      for (const el of els) for (const child of el.children) child.style.fontSize = '40px';
    });
    const rows = await tiles.evaluateAll((els) => els.map((el) => el.getBoundingClientRect().y));
    expect(new Set(rows).size).toBeGreaterThan(1);
    expect((await page.evaluate(auditLayout)).issues).toEqual([]);
  });
}
