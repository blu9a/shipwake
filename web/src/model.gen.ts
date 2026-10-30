// Generated from spec/shipwake.schema.json by scripts/types.mjs. Do not edit.

export interface Shipwake {
  site: Site;
  index: Project;
  commit: CommitDetail;
  diffs: {
    [k: string]: FileDiff;
  };
  run: Run;
  config: Config;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Site".
 */
export interface Site {
  version: number;
  generatedAt: string;
  projects: ProjectRef[];
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "ProjectRef".
 */
export interface ProjectRef {
  id: string;
  name: string;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Project".
 */
export interface Project {
  id: string;
  name: string;
  trunk: string;
  languages: string[];
  targets?: {
    [k: string]: Names;
  };
  text?: {
    [k: string]: Names;
  };
  rows: Row[];
  tags: Tag[];
  branches: Branch[];
  runs: RunSummary[];
  commits: {
    [k: string]: Commit;
  };
  rules?: {
    [k: string]: Rule;
  };
  kinds?: {
    [k: string]: Kind;
  };
  truncated?: boolean;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Names".
 */
export interface Names {
  [k: string]: string;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Row".
 */
export interface Row {
  sha: string;
  merge: boolean;
  branch?: string;
  title: string;
  kind?: string;
  commits?: string[];
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Tag".
 */
export interface Tag {
  name: string;
  sha: string;
  date: string;
  subject: string;
  body?: string;
  notes?: {
    [k: string]: string;
  };
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Branch".
 */
export interface Branch {
  name: string;
  key: string;
  updated: string;
  ahead: string[];
  behind: number;
  base: string;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "RunSummary".
 */
export interface RunSummary {
  id: string;
  kind: string;
  name: string;
  commit: string;
  branch?: string;
  start: string;
  duration: number;
  passed: number;
  failed: number;
  skipped: number;
  targets?: string[];
  context?: {
    [k: string]: string;
  };
  metrics?: number;
  previous?: Tally;
  release?: Tally;
  changed?: string[];
  was?: {
    [k: string]: string;
  };
  comparable?: boolean;
  hasRelease?: boolean;
  order: number;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Tally".
 */
export interface Tally {
  ok?: number;
  warn?: number;
  block?: number;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Commit".
 */
export interface Commit {
  sha: string;
  short: string;
  parents: string[];
  author: string;
  date: string;
  subject: string;
  body?: string;
  type?: string;
  scenarios?: number;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Rule".
 */
export interface Rule {
  direction?: string;
  warn?: string;
  block?: string;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Kind".
 */
export interface Kind {
  icon?: string;
  names?: Names;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "CommitDetail".
 */
export interface CommitDetail {
  sha: string;
  files: FileChange[];
  scenarios: ScenarioChange[];
  tags: string[];
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "FileChange".
 */
export interface FileChange {
  path: string;
  old?: string;
  status: string;
  added: number;
  deleted: number;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "ScenarioChange".
 */
export interface ScenarioChange {
  feature: string;
  title: string;
  change: string;
  steps: string[];
  was?: string[];
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "FileDiff".
 */
export interface FileDiff {
  lines?: string[];
  hl?: string[];
  more?: number;
  binary?: boolean;
  skipped?: number;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Run".
 */
export interface Run {
  schemaVersion?: number;
  id: string;
  kind: string;
  name: string;
  commit: string;
  branch?: string;
  start: string;
  duration: number;
  format: string;
  context?: {
    [k: string]: string;
  };
  tests?: Test[];
  metrics?: Metric[];
  cases?: Case[];
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Test".
 */
export interface Test {
  feature?: string;
  name: string;
  target?: string;
  status: string;
  start?: string;
  duration: number;
  attempts: number;
  error?: string;
  retried?: boolean;
  steps?: Step[];
  attachments?: Attachment[];
  evidence?: Evidence[];
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Step".
 */
export interface Step {
  name: string;
  status: string;
  start?: string;
  duration: number;
  attachments?: Attachment[];
  steps?: Step[];
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Attachment".
 */
export interface Attachment {
  name: string;
  type: string;
  path: string;
  size: number;
  start?: string;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Evidence".
 */
export interface Evidence {
  step: string;
  expected: string[];
  observed: string;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Metric".
 */
export interface Metric {
  id: string;
  suite?: string;
  name: string;
  unit: string;
  direction: string;
  values: number[];
  previous?: Comparison;
  release?: Comparison;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Comparison".
 */
export interface Comparison {
  run: string;
  label: string;
  value?: number;
  delta?: number;
  status: string;
  partial?: boolean;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Case".
 */
export interface Case {
  id: string;
  metric?: string;
  title?: string;
  input: string;
  output?: string;
  score: number;
  reason?: string;
}
/**
 * This interface was referenced by `Shipwake`'s JSON-Schema
 * via the `definition` "Config".
 */
export interface Config {
  name?: string;
  trunk?: string;
  languages?: string[];
  targets?: {
    [k: string]: Names;
  };
  translations?: {
    [k: string]: string;
  };
  metrics?: {
    [k: string]: {
      direction?: string;
      warn?: unknown;
      block?: unknown;
    };
  };
  kinds?: {
    [k: string]: {
      [k: string]: string;
    };
  };
  changelog?: unknown;
  results?: string;
}
