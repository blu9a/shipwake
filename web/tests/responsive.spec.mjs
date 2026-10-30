import { test, expect } from '@playwright/test';
import { auditLayout } from './layout-audit.mjs';
import { routes, branch, head, path, project, files } from './fixture.mjs';

// Include both sides of the page, overview and measurement-list breakpoints.
const widths = [320, 390, 480, 652, 653, 700, 701, 732, 733, 748, 749, 768, 1024, 1440];
async function readable(page) {
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  const result = await page.evaluate(auditLayout);
  expect(result.issues, JSON.stringify(result, null, 2)).toEqual([]);
}
async function open(page, route, lang = 'en', mode = 'light') {
  await page.addInitScript(({ lang, mode }) => {
    if (!localStorage.getItem('lang')) localStorage.setItem('lang', lang);
    if (!localStorage.getItem('mode')) localStorage.setItem('mode', mode);
  }, { lang, mode });
  await page.goto(`/#${route}`);
  await expect(page.locator('main h1')).toBeVisible();
  await expect(page.locator('[aria-busy="true"]')).toHaveCount(0);
}

for (const lang of ['en', 'zh']) {
  for (const route of routes) {
    test(`${lang}: ${route} stays readable through resize`, async ({ page }) => {
      const errors = [];
      page.on('pageerror', (error) => errors.push(error.message));
      await open(page, route, lang, lang === 'en' ? 'light' : 'dark');
      const heading = await page.locator('main h1').textContent();
      for (const width of widths) {
        await page.setViewportSize({ width, height: 900 });
        await readable(page);
        await expect(page.locator('main h1')).toHaveText(heading);
      }
      expect(errors).toEqual([]);
    });
  }
}

test('menus fit and keep long branch names available at every width', async ({ page }) => {
  await open(page, `/fixture/branch/${branch}`);
  for (const width of [320, 390, 700, 701, 768, 1440, 320]) {
    await page.setViewportSize({ width, height: 600 });
    for (const name of ['Projects', 'Branch', 'Settings']) {
      const button = page.getByRole('button', { name, exact: true });
      await button.click();
      await expect(button).toHaveAttribute('aria-expanded', 'true');
      if (name === 'Branch') await expect(page.getByRole('link', { name: new RegExp(branch) })).toBeVisible();
      await readable(page);
      await page.keyboard.press('Escape');
      await expect(button).toHaveAttribute('aria-expanded', 'false');
    }
  }
});

for (const lang of ['en', 'zh']) {
  test(`${lang}: short menus retain their desktop width while open on phones`, async ({ page }) => {
    await page.route('**/data/site.json', (route) => route.fulfill({ json: {
      ...files['/data/site.json'],
      projects: [{ id: 'fixture', name: 'Shipwake' }, { id: 'empty', name: 'Other' }],
    } }));
    await page.route('**/data/fixture/index.json', (route) => route.fulfill({ json: {
      ...project, branches: [{ ...project.branches[0], name: 'fix/menu' }],
    } }));
    await open(page, '/fixture/run/current', lang);
    const names = lang === 'en' ? ['Projects', 'Branch', 'Settings'] : ['项目', '分支', '设置'];
    for (const name of names) {
      await page.setViewportSize({ width: 1440, height: 900 });
      const button = page.getByRole('button', { name, exact: true });
      await button.click();
      const menu = page.locator(`[id="${await button.getAttribute('aria-controls')}"]`);
      await expect(menu).toBeVisible();
      const desktop = await menu.boundingBox();
      for (const width of [320, 390, 480, 700, 701, 768, 1440, 390]) {
        await page.setViewportSize({ width, height: 600 });
        await readable(page);
        await expect(button).toHaveAttribute('aria-expanded', 'true');
        const box = await menu.boundingBox();
        expect(Math.abs(box.width - desktop.width), 'Short content must not stretch with the viewport').toBeLessThan(1);
        expect(box.x).toBeGreaterThanOrEqual(15);
        expect(box.x + box.width).toBeLessThanOrEqual(width - 15);
        if (name === names[2]) {
          const trigger = await button.boundingBox();
          expect(Math.abs(box.x + box.width - trigger.x - trigger.width), 'Settings stays aligned to its trigger').toBeLessThan(1);
        }
      }
      await page.keyboard.press('Escape');
      await expect(menu).toHaveCount(0);
    }
  });
}

test('baseline and full measurement values survive resizing and reload', async ({ page }) => {
  await open(page, '/fixture/run/current');
  const value = page.getByRole('link', { name: /RequestLatencyWithoutAnyWordBreaks/ });
  await expect(value).toContainText('12,400 ms');
  await page.getByRole('button', { name: 'v1.0.0', exact: true }).click();
  await expect(value).toContainText('Blocking');
  for (const width of [320, 1024, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(value).toContainText('12,400 ms');
    await expect(value).toContainText('± 55 ms');
    await readable(page);
  }
  await page.reload();
  await expect(value).toContainText('Blocking');
  await readable(page);
});

test('expanded scenarios and independently scrollable diffs stay contained', async ({ page }) => {
  await open(page, '/fixture/run/tests');
  const scenario = page.locator('details.row').first();
  await scenario.locator(':scope > summary').click();
  for (const width of [320, 768, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(scenario).toHaveAttribute('open', '');
    await readable(page);
  }
  await page.goto(`/#/fixture/commit/${head}`);
  const file = page.locator('details').filter({ has: page.locator('summary', { hasText: path }) });
  await file.locator('summary').click();
  await expect(file.locator('table')).toBeVisible();
  await readable(page);
});

test('release notes retain content through resizing, language and theme changes, and refresh', async ({ page }) => {
  await open(page, '/fixture/release');
  const notes = `Draft release notes ${'LongUnbrokenReleaseText'.repeat(10)}`;
  await page.locator('textarea').fill(notes);
  for (const width of [320, 1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await expect(page.locator('textarea')).toHaveValue(notes);
    await readable(page);
  }
  await page.getByRole('button', { name: 'Settings', exact: true }).click();
  await page.locator('.topbar').getByRole('button', { name: '中文', exact: true }).click();
  await page.locator('.topbar').getByRole('button', { name: '深色', exact: true }).click();
  await page.keyboard.press('Escape');
  await expect(page.locator('html')).toHaveAttribute('lang', 'zh-CN');
  await expect(page.locator('html')).toHaveAttribute('data-mode', 'dark');
  await readable(page);
  await page.reload();
  await expect(page.locator('textarea')).toHaveValue(notes);
  await expect(page.locator('html')).toHaveAttribute('lang', 'zh-CN');
  await expect(page.locator('html')).toHaveAttribute('data-mode', 'dark');
});

test('200% text remains readable on a narrow screen', async ({ page }) => {
  await open(page, '/fixture/run/current');
  await page.setViewportSize({ width: 320, height: 900 });
  await page.evaluate(() => {
    const sizes = [...document.querySelectorAll('html, body, body *')]
      .map((el) => [el, Number.parseFloat(getComputedStyle(el).fontSize)]);
    for (const [el, size] of sizes) el.style.fontSize = `${size * 2}px`;
  });
  await readable(page);
});
