import { test, expect } from '@playwright/test';
import { runs } from './fixture.mjs';
import { auditLayout } from './layout-audit.mjs';

const image = (name, path) => ({ name, path, type: 'image/png', size: 19354 });
const decisive = image('decisive: split bill result', 'result.png');
const ordinary = image('Before splitting', 'before.png');
const nested = image('Nested evidence', 'nested.png');
const video = { name: 'Recording', path: 'recording.webm', type: 'video/webm', size: 1024 };
const step = (name, rest = {}) => ({ name, status: 'passed', duration: 637, ...rest });

async function openRun(page, lang, amend) {
  await page.addInitScript((lang) => {
    localStorage.setItem('lang', lang);
    localStorage.setItem('mode', lang === 'en' ? 'light' : 'dark');
  }, lang);
  const run = structuredClone(runs.tests);
  const base = { ...run.tests[1], error: undefined, retried: false };
  run.tests = [
    { ...base, name: 'Recorded scenario', attachments: [video], steps: [
      step('Given a bill of 100.00'),
      step('When we split it between 3 people', { attachments: [ordinary] }),
      step('Then the shares add up', { attachments: [decisive] }),
    ] },
    { ...base, name: 'Plain scenario', steps: [step('A step without files')] },
    { ...base, name: 'Nested scenario', steps: [step('Outer step', { steps: [
      step('Inner step', { attachments: [nested] }),
    ] })] },
    { ...base, name: 'File-only scenario', steps: [], attachments: [decisive] },
  ];
  amend?.(run);
  await page.route('**/data/fixture/runs/tests.json', (route) => route.fulfill({ json: run }));
  await page.route('**/data/fixture/files/tests/*.png', (route) => route.fulfill({
    contentType: 'image/png',
    body: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aWsQAAAAASUVORK5CYII=', 'base64'),
  }));
  await page.route('**/data/fixture/files/tests/*.webm', (route) => route.fulfill({ status: 204 }));
  await page.goto('/#/fixture/run/tests');
  await expect(page.getByRole('heading', { name: 'Browser acceptance', exact: true })).toBeVisible();
}

async function readable(page) {
  await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
  expect((await page.evaluate(auditLayout)).issues).toEqual([]);
}

for (const lang of ['en', 'zh']) {
  test(`${lang}: file previews are opt-in and attachment rows have a count-free marker`, async ({ page }) => {
    await openRun(page, lang);
    const label = lang === 'en' ? 'Attachments' : '附件';
    const stepsLabel = lang === 'en' ? 'Steps and files' : '步骤和文件';
    const fullLabel = lang === 'en' ? 'Full screen' : '全屏';
    const closeLabel = lang === 'en' ? 'Close' : '关闭';
    const recorded = page.locator('details.row').filter({ has: page.locator('summary', { hasText: 'Recorded scenario' }) });
    const plain = page.locator('details.row').filter({ has: page.locator('summary', { hasText: 'Plain scenario' }) });
    const nestedRow = page.locator('details.row').filter({ has: page.locator('summary', { hasText: 'Nested scenario' }) });
    const fileOnly = page.locator('details.row').filter({ has: page.locator('summary', { hasText: 'File-only scenario' }) });
    for (const row of [recorded, nestedRow, fileOnly]) {
      const marker = row.locator(':scope > summary').getByRole('img', { name: label, exact: true });
      await expect(marker).toHaveCount(1);
      await expect(marker).toHaveText('');
    }
    await expect(plain.locator(':scope > summary').getByRole('img', { name: label })).toHaveCount(0);
    await recorded.locator(':scope > summary').click();
    await expect(recorded.locator('video')).toBeVisible();
    await expect(recorded.locator('[data-horizontal-scroll]')).toHaveCount(0);
    await expect(recorded.locator('img:visible')).toHaveCount(0);
    const toggle = recorded.locator('summary').filter({ hasText: stepsLabel });
    const files = recorded.locator('details:has(> summary a[download])');
    await toggle.click();
    await expect(files).toHaveCount(2);
    await expect(recorded.locator('details[open]:has(> summary a[download])')).toHaveCount(0);
    await expect(recorded.locator('summary').filter({ hasText: 'Given a bill' }).getByRole('img', { name: label })).toHaveCount(0);
    for (const name of ['When we split', 'Then the shares']) {
      const marker = recorded.locator('summary').filter({ hasText: name }).getByRole('img', { name: label, exact: true });
      await expect(marker).toHaveCount(1);
      await expect(marker).toHaveText('');
    }
    for (const width of [320, 390, 768, 1024, 1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      await readable(page);
      const centers = await toggle.evaluate((el) => {
        const a = el.querySelector('span').getBoundingClientRect();
        const b = el.querySelector('svg').getBoundingClientRect();
        return Math.abs(a.top + a.height / 2 - b.top - b.height / 2);
      });
      expect(centers, 'The disclosure arrow must align with its label').toBeLessThan(1);
      if (width >= 390) {
        const first = recorded.locator('[class*="srow"]').filter({ hasText: 'Given a bill of 100.00' });
        const sizes = await first.evaluate((el) => {
          const top = el.closest('details.row').querySelector(':scope > summary');
          const a = getComputedStyle(el);
          const b = getComputedStyle(top);
          return { height: el.getBoundingClientRect().height, rowHeight: Number.parseFloat(b.minHeight),
            padding: a.paddingBlock, mainPadding: b.paddingBlock, lineHeight: a.lineHeight, mainLineHeight: b.lineHeight };
        });
        expect(sizes.height, 'A short step must have the same row height as the other lists').toBe(sizes.rowHeight);
        expect(sizes.padding).toBe(sizes.mainPadding);
        expect(sizes.lineHeight).toBe(sizes.mainLineHeight);
      }
      await expect(recorded.locator('details[open]:has(> summary a[download])')).toHaveCount(0);
    }
    const proof = files.filter({ has: page.getByText(decisive.name, { exact: true }) });
    const proofToggle = proof.locator(':scope > summary');
    await proofToggle.focus();
    await page.keyboard.press('Enter');
    await expect(proof).toHaveAttribute('open', '');
    await expect(proof.locator('img')).toBeVisible();
    await expect(recorded.locator('details[open]:has(> summary a[download])')).toHaveCount(1);
    await readable(page);
    await proofToggle.click();
    await expect(proof).not.toHaveAttribute('open', '');
    await proof.getByRole('button', { name: fullLabel, exact: true }).click();
    await expect(page.getByRole('button', { name: closeLabel, exact: true })).toBeVisible();
    await expect(proof).not.toHaveAttribute('open', '');
    await page.keyboard.press('Escape');
    await expect(page.getByRole('button', { name: closeLabel, exact: true })).toHaveCount(0);
    await nestedRow.locator(':scope > summary').click();
    await nestedRow.locator('summary').filter({ hasText: stepsLabel }).click();
    await expect(nestedRow.locator('summary').filter({ hasText: 'Outer step' }).getByRole('img', { name: label })).toHaveCount(1);
    await expect(nestedRow.locator('summary').filter({ hasText: 'Inner step' }).getByRole('img', { name: label })).toHaveCount(1);
    await expect(nestedRow.locator('details[open]:has(> summary a[download])')).toHaveCount(0);
    await fileOnly.locator(':scope > summary').click();
    await expect(fileOnly.locator('details[open]')).toHaveCount(0);
    await expect(fileOnly.locator('img:visible')).toHaveCount(0);
    await readable(page);
  });
}


const longName = (lang) => lang === 'zh'
  ? '确认所有人的金额加起来等于账单总额，且不会丢失最后一分钱。'.repeat(5)
  : 'Then every share adds up to the original bill without losing the last cent. '.repeat(5).trim();

async function openLongNames(page, lang) {
  const stepName = longName(lang);
  const fileName = `${lang === 'zh' ? '决定性截图：' : 'decisive: '}${longName(lang)}.png`;
  await openRun(page, lang, (run) => {
    run.tests[0].steps[0].name = stepName;
    run.tests[0].steps[1].attachments = [image(fileName, 'before.png')];
    run.tests[0].steps.push(step('OK'));
  });
  const row = page.locator('#t0');
  await row.locator(':scope > summary').click();
  await row.locator('summary').filter({ hasText: lang === 'en' ? 'Steps and files' : '步骤和文件' }).click();
  return { row, name: page.getByTitle(stepName, { exact: true }), file: page.getByTitle(fileName, { exact: true }) };
}

const shifted = (el) => new DOMMatrix(getComputedStyle(el.firstElementChild).transform).m41;
const animations = (el) => el.firstElementChild.getAnimations().length;

for (const lang of ['en', 'zh']) {
  test(`${lang}: long step/file names ellipsize and scroll within fixed rows on hover or focus`, async ({ page }) => {
    const { row, name, file } = await openLongNames(page, lang);
    for (const width of [320, 390, 768, 1024, 1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      await readable(page);
      const heights = await row.locator('.listRow').evaluateAll((rows) => rows.map((el) => el.getBoundingClientRect().height));
      expect(heights.every((height) => height === 44), 'Long names must not change the row height').toBe(true);
      for (const label of [name, file]) {
        await expect(label).toHaveAttribute('tabindex', '0');
        await expect(label.locator(':scope > span')).toHaveCSS('text-overflow', 'ellipsis');
      }
    }
    await file.scrollIntoViewIfNeeded();
    await readable(page);
    const fixed = await file.evaluate((el) => {
      const row = el.closest('.listRow');
      return { row: row.getBoundingClientRect().toJSON(), actions: row.querySelector('a[download]').getBoundingClientRect().toJSON() };
    });
    await file.hover();
    await expect.poll(() => file.evaluate(shifted)).toBeLessThan(-2);
    await readable(page);
    // Move the browser's real animation to its end hold: the tail must be readable inside the label.
    const tail = await file.evaluate(async (el) => {
      const inner = el.firstElementChild;
      const motion = inner.getAnimations()[0];
      motion.pause();
      motion.currentTime = Number(motion.effect.getTiming().duration) - 1;
      await new Promise((r) => requestAnimationFrame(r));
      const text = inner.firstChild;
      const range = document.createRange();
      range.setStart(text, text.length - 4);
      range.setEnd(text, text.length);
      return { tail: range.getBoundingClientRect().toJSON(), label: el.getBoundingClientRect().toJSON(),
        row: el.closest('.listRow').getBoundingClientRect().toJSON(),
        actions: el.closest('.listRow').querySelector('a[download]').getBoundingClientRect().toJSON() };
    });
    expect(tail.tail.right).toBeLessThanOrEqual(tail.label.right + 1);
    expect(tail.tail.left).toBeGreaterThanOrEqual(tail.label.left - 1);
    expect(tail.row).toEqual(fixed.row);
    expect(tail.actions).toEqual(fixed.actions);
    await expect(row.locator('details[open]:has(> summary a[download])')).toHaveCount(0);
    await page.mouse.move(1, 1);
    await expect.poll(() => file.evaluate(animations)).toBe(0);
    await expect(file.locator(':scope > span')).toHaveCSS('text-overflow', 'ellipsis');
    await name.focus();
    await expect.poll(() => name.evaluate(shifted)).toBeLessThan(-2);
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await expect.poll(() => name.evaluate(animations)).toBe(0);
    await expect(name.locator(':scope > span')).toHaveCSS('text-overflow', 'ellipsis');
    await page.emulateMedia({ reducedMotion: 'no-preference' });
    await expect.poll(() => name.evaluate(shifted)).toBeLessThan(-2);
    await page.keyboard.press('Tab');
    await expect.poll(() => name.evaluate(animations)).toBe(0);
    const resizing = page.getByTitle('When we split it between 3 people', { exact: true });
    await resizing.focus();
    await expect.poll(() => resizing.evaluate(shifted)).toBeLessThan(-2);
    await page.setViewportSize({ width: 1440, height: 900 });
    await expect(resizing).toBeFocused();
    await expect.poll(() => resizing.evaluate(animations)).toBe(0);
    await page.setViewportSize({ width: 390, height: 900 });
    await expect(resizing).toBeFocused();
    await expect.poll(() => resizing.evaluate(shifted)).toBeLessThan(-2);
    await page.keyboard.press('Tab');
    const short = page.getByTitle('OK', { exact: true });
    await short.hover();
    await expect(short).not.toHaveAttribute('tabindex');
    expect(await short.evaluate(animations)).toBe(0);
    await readable(page);
  });
}

test.describe('Touch access to truncated names', () => {
  test.use({ hasTouch: true, isMobile: true, viewport: { width: 390, height: 844 } });
  test('tapping a long step focuses and scrolls its full name without opening file previews', async ({ page }) => {
    const { row, name } = await openLongNames(page, 'en');
    await name.tap();
    await expect(name).toBeFocused();
    await expect.poll(() => name.evaluate(shifted)).toBeLessThan(-2);
    await expect(row.locator('details[open]:has(> summary a[download])')).toHaveCount(0);
    await readable(page);
  });
});
