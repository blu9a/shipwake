import { expect } from '@playwright/test';
import { lookName } from '../../packages/playwright/src/index.ts';
import { Given, me, prove, shipwake, Then, When } from './project.mjs';

// The project's components, drawn by a browser as its tests would screenshot them. A change to one is a
// change to its styles; everything else renders the same pixels every time.
const components = {
  'Split button': (v) =>
    `<button style="font:600 15px system-ui;padding:10px 22px;border:0;border-radius:${v.round ?? 8}px;color:#fff;background:${v.color ?? '#2563eb'}">Split the bill</button>`,
  'List row': (v) =>
    `<div style="display:flex;justify-content:space-between;gap:12px;width:${v.width === 'phone' ? 300 : 560}px;padding:12px 16px;font:15px system-ui;border-bottom:1px solid ${v.dark ? '#333' : '#e5e5ea'}">
      <span>Dinner at the corner place</span><b style="color:${v.amount ?? (v.dark ? '#f4f4f5' : '#111')}">$120.00</b></div>`,
  Total: () => `<div style="font:700 22px system-ui;padding:8px 12px">Each pays $30.00</div>`,
  'Receipt card': () =>
    `<div style="width:260px;padding:14px 16px;border-radius:12px;border:1px solid #e5e5ea;font:14px system-ui">Receipt<br><b>$120.00</b> for 4 people</div>`,
};

/** A screenshot of one component, as the project's tests take it. */
async function shoot(browser, subject, v = {}) {
  const ctx = await browser.newContext({
    viewport: { width: v.width === 'desktop' ? 720 : 360, height: 240 },
    deviceScaleFactor: 1,
    colorScheme: v.dark ? 'dark' : 'light',
  });
  const page = await ctx.newPage();
  const bg = v.dark ? '#0a0a0a' : '#ffffff';
  const fg = v.dark ? '#f4f4f5' : '#111111';
  await page.setContent(`<body style="margin:0;padding:12px;background:${bg};color:${fg}"><div id="s" style="display:inline-block">${components[subject](v)}</div></body>`);
  const png = await page.locator('#s').screenshot({ animations: 'disabled', caret: 'hide' });
  await ctx.close();
  return png;
}

let n = 0;
/** Records a run of the project's UI tests on a commit, with a screenshot per subject and variant. */
async function recordLooks(browser, repo, commit, looks) {
  const files = {};
  const attachments = [];
  for (const { subject, variant = [], ...v } of looks) {
    const file = `look-${++n}.png`;
    files[file] = await shoot(browser, subject, v);
    attachments.push({ name: lookName(subject, variant), path: file, type: 'image/png' });
  }
  repo.record(commit, 'UI tests', [{ feature: 'Components', name: 'Every component looks right', status: 'passed', duration: 900, attachments }], files);
}

/** Approves every look of a commit, as the person would after reviewing it. */
const approveAll = (repo, commit) => shipwake(repo.home, 'looks', 'approve', '--all', '--commit', commit, '--repo', repo.dir);

const variants = [
  { variant: ['light', 'phone'], width: 'phone' },
  { variant: ['light', 'desktop'], width: 'desktop' },
  { variant: ['dark', 'phone'], width: 'phone', dark: true },
  { variant: ['dark', 'desktop'], width: 'desktop', dark: true },
];

/** The commit's list of looks, on its page. */
const looksList = (page) => page.getByRole('region', { name: 'Looks' });
const shot = (page, name) => page.locator(`[data-look="${name}"]`).first();

Given('a project records a screenshot of each component or screen with its tests', async ({ repo, browser, world }) => {
  world.base = repo.commit('feat: split a bill between friends');
  await recordLooks(browser, repo, world.base, [{ subject: 'Split button' }, { subject: 'List row' }, { subject: 'Total' }]);
  approveAll(repo, world.base);
});

Given('a change alters how one of them looks', async ({ repo, browser, world }) => {
  world.change = repo.commit('feat: a friendlier split button');
  await recordLooks(browser, repo, world.change, [{ subject: 'Split button', color: '#16a34a', round: 20 }, { subject: 'List row' }, { subject: 'Total' }]);
});

When('I open the change', async ({ page, served, world }) => {
  await served.open(page, `/commit/${world.change}`);
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
});

Then('I see that one of them looks different', async ({ page, $testInfo, $step }) => {
  const list = looksList(page);
  await expect(list).toBeVisible();
  const button = shot(page, 'Split button');
  await expect(button).toBeVisible();
  await expect(button).toHaveAttribute('data-look-status', 'changed');
  await expect(list.locator('[data-look-status="changed"]')).toHaveCount(1);
  await expect(list.getByText('Changed', { exact: true }).first()).toBeVisible();
  await prove({ page, $testInfo, $step }, list.locator('[data-look-subject="Split button"]'), ['Changed', 'Split button']);
});

Then('the screenshots that look the same are not in the way', async ({ page }) => {
  const list = looksList(page);
  const folded = list.locator('details', { hasText: '2 look the same' });
  await expect(folded).toBeVisible();
  await expect(folded).not.toHaveAttribute('open', '');
  await expect(shot(page, 'List row')).toBeHidden();
  await expect(shot(page, 'Total')).toBeHidden();
});

Given('a change alters how a button looks', async ({ repo, browser, world }) => {
  world.base = repo.commit('feat: split a bill between friends');
  await recordLooks(browser, repo, world.base, [{ subject: 'Split button' }]);
  approveAll(repo, world.base);
  world.change = repo.commit('feat: a friendlier split button');
  await recordLooks(browser, repo, world.change, [{ subject: 'Split button', color: '#16a34a', round: 20 }]);
});

When('I open its screenshot', async ({ page, served, world }) => {
  await served.open(page, `/commit/${world.change}`);
  await looksList(page).getByRole('link', { name: 'Split button', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Split button', level: 1 })).toBeVisible();
});

/** Every image in the box has loaded and has pixels. */
async function loaded(box) {
  for (const img of await box.locator('img').all()) {
    await expect.poll(() => img.evaluate((i) => i.complete && i.naturalWidth > 0)).toBe(true);
  }
}

Then('I can compare the approved look and the new one, side by side or one over the other', async ({ page, $testInfo, $step }) => {
  const main = page.locator('main');
  // side by side: the approved look, then the new one
  await expect(main.getByRole('button', { name: 'Side by side' })).toHaveAttribute('aria-pressed', 'true');
  await expect(main.getByRole('img', { name: 'Approved: Split button' })).toBeVisible();
  await expect(main.getByRole('img', { name: 'New: Split button' })).toBeVisible();
  await loaded(main);
  // one over the other: a slider between them, then an onion skin
  await main.getByRole('button', { name: 'Slider' }).click();
  const slider = main.getByRole('slider', { name: 'Position' });
  await expect(slider).toBeVisible();
  await slider.fill('25');
  await main.getByRole('button', { name: 'Onion skin' }).click();
  const opacity = main.getByRole('slider', { name: 'Opacity' });
  await expect(opacity).toBeVisible();
  await opacity.fill('60');
  await expect(main.getByRole('img', { name: 'Approved: Split button' })).toBeVisible();
  await expect(main.getByRole('img', { name: 'New: Split button' })).toBeVisible();
  await main.getByRole('button', { name: 'Side by side' }).click();
  await prove({ page, $testInfo, $step }, main.locator('figure').first().locator('..'), ['Approved', 'New']);
});

Then('the parts that changed are highlighted', async ({ page, $testInfo, $step }) => {
  const main = page.locator('main');
  await expect(main.getByRole('checkbox', { name: 'Highlight changes' })).toBeChecked();
  await expect(main.getByText(/pixels differ from the approved look\./)).toBeVisible();
  const diff = main.locator('img[data-diff]').last();
  await expect(diff).toBeVisible();
  // the highlight marks the button's pixels, and nothing around it
  const marked = await diff.evaluate(async (img) => {
    await img.decode();
    const c = document.createElement('canvas');
    c.width = img.naturalWidth;
    c.height = img.naturalHeight;
    const g = c.getContext('2d');
    g.drawImage(img, 0, 0);
    const px = g.getImageData(0, 0, c.width, c.height).data;
    let on = 0;
    for (let i = 3; i < px.length; i += 4) if (px[i] > 0) on++;
    const corner = px[3]; // the top-left pixel is page background in both looks
    return { on, total: c.width * c.height, corner };
  });
  expect(marked.on).toBeGreaterThan(0);
  expect(marked.on).toBeLessThan(marked.total);
  expect(marked.corner).toBe(0);
  await prove({ page, $testInfo, $step }, main.locator('figure').last(), ['New']);
});

Given('a project records each screenshot in light and dark, and at phone and desktop widths', async ({ repo, browser, world }) => {
  world.base = repo.commit('feat: list the bills');
  await recordLooks(browser, repo, world.base, variants.map((v) => ({ subject: 'List row', ...v })));
  approveAll(repo, world.base);
});

When('a change alters how a list row looks', async ({ page, repo, browser, served, world }) => {
  world.change = repo.commit('fix: amounts stand out in dark mode');
  // the amount's color changes in dark mode only
  await recordLooks(browser, repo, world.change, variants.map((v) => ({ subject: 'List row', ...v, ...(v.dark ? { amount: '#fbbf24' } : {}) })));
  await served.open(page, `/commit/${world.change}`);
});

Then('its variants are shown together, with the ones that changed marked', async ({ page, $testInfo, $step }) => {
  const row = looksList(page).locator('[data-look-subject="List row"]');
  await expect(row.locator('[data-look]')).toHaveCount(4);
  await expect(row.getByText('2 of 4 variants changed')).toBeVisible();
  for (const v of variants) {
    const name = lookName('List row', v.variant).replace(/^look: /, '');
    await expect(row.locator(`[data-look="${name}"]`)).toHaveAttribute('data-look-status', v.dark ? 'changed' : 'same');
  }
  await loaded(row);
  // on the look's page, the variants are there together too, the changed ones marked
  await row.getByRole('link', { name: 'List row', exact: true }).click();
  const nav = page.getByRole('navigation', { name: 'Variants' });
  await expect(nav.locator('[data-look]')).toHaveCount(4);
  await expect(nav.locator('[data-look-status="changed"]')).toHaveCount(2);
  await expect(nav.locator('[data-look-status="same"]')).toHaveCount(2);
  await prove({ page, $testInfo, $step }, nav, ['dark, phone', 'dark, desktop']);
});

When('I approve the new look', async ({ page, served, world }) => {
  await served.open(page, `/commit/${world.change}`);
  await looksList(page).getByRole('link', { name: 'Split button', exact: true }).click();
  await page.getByRole('button', { name: 'Approve this look' }).click();
  await expect(page.getByText('Approved. Later changes are compared with this look.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Approve this look' })).toHaveCount(0);
});

Then('later changes are compared with the look I approved', async ({ page, repo, browser, served, world }) => {
  // a later change that leaves the button as approved: nothing to review
  world.later = repo.commit('docs: explain splitting');
  await recordLooks(browser, repo, world.later, [{ subject: 'Split button', color: '#16a34a', round: 20 }]);
  await served.restart();
  await served.open(page, `/commit/${world.later}`);
  const list = looksList(page);
  await expect(list.getByText('1 looks the same')).toBeVisible();
  await expect(list.locator('[data-look-status="changed"], [data-look-status="new"]')).toHaveCount(0);
  await list.getByText('1 looks the same').click();
  await shot(page, 'Split button').click();
  await expect(page.getByText('Looks as approved.')).toBeVisible();
  const short = world.change.slice(0, 7);
  await expect(page.getByText(new RegExp(`^Approved by ${me.name} on ${short}`))).toBeVisible();
  expect(shipwake(repo.home, 'check', '--commit', world.later, repo.dir)).toContain('Looks: 1 screenshot as approved');
});

Then('the approval records who approved it', async ({ page, repo, world, $testInfo, $step }) => {
  const commit = repo.git('log', '-1', '--format=%B', '--grep=^look: approve', 'shipwake-results');
  expect(commit).toContain(`look: approve "Split button" on ${world.change.slice(0, 7)}`);
  expect(commit).toContain(`Approved-by: ${me.name} <${me.email}>`);
  const said = page.getByText(new RegExp(`^Approved by ${me.name}`));
  await prove({ page, $testInfo, $step }, said, [`Approved by ${me.name}`]);
});

Given('a change alters how a button looks and nobody has approved it', async ({ repo, browser, world }) => {
  world.base = repo.commit('feat: split a bill between friends');
  await recordLooks(browser, repo, world.base, [{ subject: 'Split button' }, { subject: 'Total' }]);
  approveAll(repo, world.base);
  world.change = repo.commit('feat: a friendlier split button');
  await recordLooks(browser, repo, world.change, [{ subject: 'Split button', color: '#16a34a', round: 20 }, { subject: 'Total' }]);
});

When('I check whether the change can be released', async ({ page, repo, served, world }) => {
  try {
    world.check = { out: shipwake(repo.home, 'check', '--commit', world.change, repo.dir), code: 0 };
  } catch (e) {
    world.check = { out: e.stdout, code: e.status };
  }
  await served.open(page, '/release');
  await expect(page.getByRole('heading', { name: 'New release', exact: true })).toBeVisible();
});

Then('the check fails and names the screenshot that changed', async ({ page, world, $testInfo, $step }) => {
  expect(world.check.code).toBe(1);
  expect(world.check.out).toContain('✗ Looks: "Split button" looks different from its approved look');
  expect(world.check.out).toContain('✓ Looks: 1 screenshot as approved');
  // the release page judges the same way: one verdict
  const held = page.getByRole('link', { name: 'Looks: Split button looks different and nobody approved it' });
  await expect(held).toBeVisible();
  await page.getByRole('textbox').last().fill('### Features\n\n- A friendlier split button');
  await expect(page.getByRole('button', { name: 'Approve release', exact: true })).toBeDisabled();
  await prove({ page, $testInfo, $step }, held, ['Split button']);
});

Given('a change adds a screenshot of a new component', async ({ repo, browser, world }) => {
  world.base = repo.commit('feat: split a bill between friends');
  await recordLooks(browser, repo, world.base, [{ subject: 'Split button' }]);
  approveAll(repo, world.base);
  world.change = repo.commit('feat: a receipt for every bill');
  await recordLooks(browser, repo, world.change, [{ subject: 'Split button' }, { subject: 'Receipt card' }]);
});

Then('I see it as new, with nothing to compare it with', async ({ page, $testInfo, $step }) => {
  const card = shot(page, 'Receipt card');
  await expect(card).toHaveAttribute('data-look-status', 'new');
  await expect(looksList(page).locator('[data-look-subject="Receipt card"]').getByText('New', { exact: true }).first()).toBeVisible();
  await looksList(page).getByRole('link', { name: 'Receipt card', exact: true }).click();
  const said = page.getByText(/^New: nothing was approved for it yet, so there is nothing to compare it with\./);
  await expect(said).toBeVisible();
  await expect(page.getByRole('button', { name: 'Side by side' })).toHaveCount(0);
  await expect(page.getByRole('img', { name: 'Receipt card' })).toBeVisible();
  await prove({ page, $testInfo, $step }, said, ['nothing to compare it with']);
});

Then('it waits for my approval like a change', async ({ page, repo, world }) => {
  await expect(page.getByRole('button', { name: 'Approve this look' })).toBeVisible();
  let out = '';
  try {
    shipwake(repo.home, 'check', '--commit', world.change, repo.dir);
  } catch (e) {
    out = e.stdout;
  }
  expect(out).toContain('✗ Looks: "Receipt card" is new and waits for a first approval');
});
