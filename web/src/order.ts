// The order of a timeline: commits in history order, newest first, and above each commit the release
// that tags it, the workspace commits made while it was checked out (local mode), and the runs that
// tested it, newest run first. Runs follow their commit, not the time
// they ran: a run recorded later (nightly, re-run, backfilled) still belongs with what it tested.

export type Kind = 'release' | 'kept' | 'run' | 'commit';

export interface Placed {
  commit: string; // the commit it belongs to
  kind: Kind;
  at: number; // when a run ran, to order runs of the same commit
}

const rank: Record<Kind, number> = { release: 0, kept: 1, run: 2, commit: 3 };

/** The items in history order. commits is the history, newest first; items on other commits are left out. */
export function inHistory<T extends Placed>(commits: string[], items: T[]): T[] {
  const pos = new Map(commits.map((c, i) => [c, i]));
  return items
    .filter((x) => pos.has(x.commit))
    .sort((a, b) => (pos.get(a.commit) ?? 0) - (pos.get(b.commit) ?? 0) || rank[a.kind] - rank[b.kind] || b.at - a.at);
}
