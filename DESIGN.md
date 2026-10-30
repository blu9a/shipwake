# Responsive layout contract

Shipwake keeps its top-bar navigation and centered, single-column page. The page
owns its width and the document owns vertical scrolling. Page content is bounded
by a 760px content width (1280px on a commit, for its file tree) and 16px/24px
page gutters.

- The top bar floats over the page and is exactly as wide as the page content;
  it never spans a wider window. Its width and the column's come from the same
  `--page-w` and `--pad` variables.
- Cards use the `--card*` tokens: plain raised surfaces in light mode, a faint
  glass (translucent gradient, light top edge) in dark mode.
- A commit's files show as a tree beside the diffs when the page is wider than
  900px: resizable (drag, arrow keys, double-click resets) and hideable, both
  remembered. Narrower, a Files / Changes switch shows one at a time.
- Code in diffs is colored at build time (`internal/highlight`, Chroma): whole
  files are tokenized, so the page only draws spans. Diffs off screen are not
  drawn (`content-visibility: auto`).
- The top bar is frosted glass, calibrated against the reference images: a
  nearly opaque neutral tint (dark 80%, light 68%) over a 40px blur, no
  saturation boost, highlight or gradient, and no shadow in dark mode. Content
  underneath shows only as soft light or shade, never as shapes. Write
  `backdrop-filter` without a prefix; the build adds it.
- Dark cards are quiet surfaces: 3.5% white over the `#0a0a0a` page with a 7%
  hairline; no gradients, highlights or shadows.

- Grid tracks containing text use `minmax(0, 1fr)`; grid/flex children that can
  shrink need `min-width: 0`. Context labels are bounded independently of values.
  Long branch names, paths, model IDs, scenario text and notes must wrap, except the step/file names below.
- Run overviews keep the details at full width and the verdict boxes in a row
  under them, at every width, so long values (commit titles, previous values) don't wrap early.
  The boxes have a maximum width and keep to the left rather than stretching.
  Statistics wrap only when their complete labels and counts cannot fit together. Use the content's
  minimum width to decide when to wrap, not the boxes' maximum width; this also applies at 320px,
  with Chinese labels and with larger text. Keep each status label and count intact.
  Hide measurement verdict statistics when the selected baseline has no judged
  metrics, and let metadata reclaim the empty column. All-OK results remain visible.
- Both measurement lists use `ui.result*` styles. Below 620px of **list width**,
  the name and status occupy the first row, value and delta the second. The
  numeric value and spread remain complete; the optional sparkline yields space.
  Do not add a separate viewport breakpoint to one of the lists.
- Section controls and segmented options wrap. A resize must keep the selected
  baseline, expanded details and draft release notes intact.
- Navigation triggers may ellipsize long names with their full title available.
  The picker displays the full name. Menus keep their content-based desktop width
  on phones, capped by the viewport with 16px gutters. Keep trigger alignment when
  it fits, shifting at viewport edges; never stretch a menu between both gutters.
  Reposition open menus when their content or viewport changes. Menu height stays
  bounded by the viewport; settings remain reachable.
- Test and step rows with attachments show a paperclip, without attachment counts.
  File previews start collapsed, including decisive screenshots; opening the steps
  list must not automatically open an attachment preview. Use a centered SVG disclosure arrow.
  Keep recordings above the steps/files, without a duplicate screenshot strip. Steps and files
  share the list-row spacing (44px minimum, 4px vertical padding). Step/file names stay on one line,
  using an ellipsis when they exceed the available width. Only an overflowing name scrolls on hover
  or focus (including a touch focus), inside its own bounds; leaving it resets the ellipsis. Preserve
  the full text and title, honor reduced motion, and recalculate after resize or translated text changes.
  A focused name keeps its focus when a wider viewport makes the whole name fit.
  An absent step badge reserves no width, especially on phones.
- Horizontal scrolling is local to intrinsically wide content such as source
  diffs and screenshot strips. Never mask page overflow with `overflow-x: hidden`.

`make test-responsive` builds the production app and checks fixture-backed pages
through 320–1440px, both languages, light/dark appearance, breakpoint boundaries,
long content, 200% text, menus, expanded details, baseline changes and saved notes.
The checks measure text boxes as well as page width, so clipping inside a list
cannot pass just because it has `overflow: hidden`. CI runs these browser checks
and retains screenshots/traces on failure. These are geometric/behavior checks;
visual review with real project data remains part of accepting layout changes.

## Workspace changes

Approval succeeds once: immediately remove its actions, then replace the timeline's data resources
with the rebuilt files. A render counter alone does not invalidate compiler-memoized loads. Keep
unrelated open proposals and unsaved text mounted. A failed approval remains editable and retryable.
Generated JSON keeps stable URLs, so explicit reloads must bypass the browser's HTTP cache.

## Timeline

The timeline is strictly newest first, top to bottom, with one rule and no exceptions: every entry
sits at the time it happened.

- Entries are what happened on the trunk: commits and merges at their time, test and measurement
  runs at the time they ran, releases when they were tagged. Runs carry no "tested commit" label;
  the run's page and the commit's page show the link.
- The newest entry, whatever it is, is the top card.
- Only a merge's branch history folds: its commits and the runs on them, newest first too, in a
  quieter style (plain commit subjects, one-line runs). Open branches have their own view.
