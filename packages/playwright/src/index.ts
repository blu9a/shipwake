// Recording helpers for Playwright tests whose results go to Shipwake.
import type { Locator, Page, TestInfo } from '@playwright/test';

/**
 * Marks the element a result step checked: outlines it, attaches a screenshot, and records what was
 * expected and the text actually observed in it, so the reason the step passed comes from the run.
 *
 *   await expect(goal).toContainText('due Nov 15');
 *   await decisive(goal, testInfo, 'the goal is due on Nov 15', ['due Nov 15']);
 */
export async function decisive(target: Locator, info: TestInfo, step: string, expected: string[] = []) {
  await target.scrollIntoViewIfNeeded();
  const observed = (await target.innerText()).replace(/\s+/g, ' ').trim();
  await target.evaluate((el) => {
    const s = (el as HTMLElement).style;
    (el as HTMLElement).dataset.shipwake = 'decisive';
    s.outline = '3px solid #E5484D';
    s.outlineOffset = '3px';
  });
  await info.attach(`decisive: ${step}`, { body: await target.page().screenshot(), contentType: 'image/png' });
  await info.attach(`observed: ${step}`, { body: JSON.stringify({ step, expected, observed }), contentType: 'application/json' });
}

/**
 * The attachment name Shipwake reads as a screenshot for visual review: "look: <subject>", or
 * "look: <subject> [<variant>]" with the variant's parts joined by commas, such as
 * "look: Split button [dark, phone]". The same subject and variant in a later run is compared with it.
 */
export function lookName(subject: string, variant?: string | string[]): string {
  const name = subject.replace(/\s+/g, ' ').trim();
  const parts = (Array.isArray(variant) ? variant : variant ? variant.split(',') : []).map((v) => v.replace(/\s+/g, ' ').trim()).filter(Boolean);
  if (!name) throw new Error('a look needs a subject, such as "Split button"');
  if (parts.some((p) => /[[\],]/.test(p))) throw new Error(`a variant's parts can't contain brackets or commas: ${parts.join(' | ')}`);
  if (!parts.length && /\]$/.test(name)) throw new Error(`a subject without a variant can't end with "]": ${name}`);
  return parts.length ? `look: ${name} [${parts.join(', ')}]` : `look: ${name}`;
}

/**
 * Records how something looks, for Shipwake's visual review: a screenshot of the page or of one element,
 * attached as `lookName(subject, variant)`. Animations are stopped and the caret hidden, so the same
 * look gives the same pixels. Shipwake compares it with the look approved for that subject and variant.
 *
 *   await look(page.getByRole('button', { name: 'Split' }), testInfo, 'Split button', ['dark', 'phone']);
 */
export async function look(
  target: Page | Locator,
  info: TestInfo,
  subject: string,
  variant?: string | string[],
  options: { fullPage?: boolean; mask?: Locator[] } = {},
) {
  const name = lookName(subject, variant);
  const shot = { animations: 'disabled' as const, caret: 'hide' as const, mask: options.mask };
  const body = 'goto' in target ? await target.screenshot({ ...shot, fullPage: options.fullPage }) : await target.screenshot(shot);
  await info.attach(name, { body, contentType: 'image/png' });
}

/** Clears the outline left by the previous decisive step. */
export async function clearMarks(page: Page) {
  await page
    .evaluate(() =>
      document.querySelectorAll('[data-shipwake="decisive"]').forEach((el) => {
        const s = (el as HTMLElement).style;
        s.outline = '';
        s.outlineOffset = '';
        delete (el as HTMLElement).dataset.shipwake;
      }),
    )
    .catch(() => {});
}

/**
 * Shows a touch-style pointer in recordings: a white circle that follows the pointer to its target
 * and presses down on clicks. Combine with `slowClicks` so it arrives before the click.
 */
export async function showPointer(page: Page) {
  await page.addInitScript(() => {
    let dot: HTMLDivElement | null = null;
    let hide = 0;
    const ensure = () => {
      if (dot?.isConnected) return dot;
      dot = document.createElement('div');
      dot.style.cssText =
        'position:fixed;left:0;top:0;width:30px;height:30px;margin:-15px 0 0 -15px;border-radius:50%;z-index:2147483647;pointer-events:none;' +
        'background:rgba(255,255,255,.72);border:1.5px solid rgba(0,0,0,.28);box-shadow:0 1px 6px rgba(0,0,0,.28);transition:transform .12s ease-out,opacity .25s;opacity:0';
      (document.body || document.documentElement).appendChild(dot);
      return dot;
    };
    const at = (e: PointerEvent) => {
      const d = ensure();
      d.style.left = `${e.clientX}px`;
      d.style.top = `${e.clientY}px`;
      d.style.opacity = '1';
      clearTimeout(hide);
    };
    addEventListener('pointermove', at, true);
    addEventListener('pointerdown', (e) => {
      at(e);
      ensure().style.transform = 'scale(.78)';
    }, true);
    addEventListener('pointerup', () => {
      const d = ensure();
      d.style.transform = '';
      hide = window.setTimeout(() => {
        d.style.opacity = '0';
      }, 900);
    }, true);
  });
}

/** Makes every click, check and tap move the pointer to its target and wait `lead` ms first. */
export function slowClicks(page: Page, lead = 200) {
  const proto = Object.getPrototypeOf(page.locator('body')) as Record<string, (...a: unknown[]) => Promise<unknown>>;
  if ((proto as { __shipwake?: boolean }).__shipwake) return;
  (proto as { __shipwake?: boolean }).__shipwake = true;
  for (const name of ['click', 'check', 'tap', 'dblclick']) {
    const orig = proto[name] as (...a: unknown[]) => Promise<unknown>;
    proto[name] = async function (this: Locator, ...args: unknown[]) {
      await this.hover({ force: true, timeout: 2000 }).catch(() => {});
      await this.page().waitForTimeout(lead);
      return orig.apply(this, args);
    };
  }
}
