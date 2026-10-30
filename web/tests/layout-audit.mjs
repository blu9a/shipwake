// Geometric layout checks, run inside the page by Playwright.
export function auditLayout() {
  const issues = [];
  const width = document.documentElement.clientWidth;
  const tolerance = 1;
  const describe = (el) => `${el.tagName.toLowerCase()} ${el.textContent.trim().slice(0, 65)}`;
  const locallyScrollable = (el) => {
    for (let p = el.parentElement; p && p !== document.body; p = p.parentElement) {
      const css = getComputedStyle(p);
      if (p.hasAttribute('data-horizontal-scroll') && /auto|scroll/.test(css.overflowX) && p.scrollWidth > p.clientWidth + tolerance) return true;
    }
    return false;
  };
  const visible = (el) => el.checkVisibility() && !el.closest('svg');
  if (document.documentElement.scrollWidth > width + tolerance) {
    issues.push(`Page overflow: ${document.documentElement.scrollWidth} > ${width}`);
  }
  for (const el of document.querySelectorAll('main *, .topbar *')) {
    if (!visible(el) || locallyScrollable(el)) continue;
    // A named scrolling label deliberately clips its inner text. Check the label's own bounds
    // below; its full title remains available and separate behavior checks cover revealing the tail.
    const label = el.closest('[data-scroll-name][title]');
    if (label && label !== el && getComputedStyle(label).overflowX === 'hidden' && getComputedStyle(label).whiteSpace === 'nowrap') continue;
    const rect = el.getBoundingClientRect();
    if (rect.width && (rect.left < -tolerance || rect.right > width + tolerance)) {
      issues.push(`Outside viewport: ${describe(el)} (${rect.left.toFixed(1)}..${rect.right.toFixed(1)})`);
    }
    // A zero-width/shrunken label can overlap its neighbour while the page itself
    // has no scrollbar. Check actual text boxes as well as element boxes.
    const css = getComputedStyle(el);
    if (css.textOverflow === 'ellipsis' && el.title) continue; // named nav trigger, full label in its picker
    for (const node of el.childNodes) {
      if (node.nodeType !== Node.TEXT_NODE || !node.textContent.trim()) continue;
      const range = document.createRange();
      range.selectNodeContents(node);
      const outside = (text) => text.width && (text.left < rect.left - tolerance || text.right > rect.right + tolerance);
      if (![...range.getClientRects()].some(outside)) continue;
      // pre-wrap may hang a blank at a soft line break outside the line box.
      // Confirm that printable text, rather than that invisible blank, overflows.
      for (const word of node.textContent.matchAll(/\S+/g)) {
        range.setStart(node, word.index);
        range.setEnd(node, word.index + word[0].length);
        if ([...range.getClientRects()].some(outside)) {
          issues.push(`Text overflows its own box: ${describe(el)}`);
          break;
        }
      }
    }
  }
  return { width, documentWidth: document.documentElement.scrollWidth, issues: [...new Set(issues)].slice(0, 40) };
}
