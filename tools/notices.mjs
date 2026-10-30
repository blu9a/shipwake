// Third-party notices for the release binaries: the license of every Go module linked into
// shipwake, and of every package bundled into its web app, in one file shipped with each archive.
// A package without a license file stops the release rather than ship without its notice.
//   node tools/notices.mjs [out]   (default .cache/notices/THIRD_PARTY_NOTICES.md)
import { execFileSync } from 'node:child_process';
import { mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const out = resolve(process.argv[2] ?? join(root, '.cache/notices/THIRD_PARTY_NOTICES.md'));
const run = (cmd, args, cwd = root) => execFileSync(cmd, args, { cwd, encoding: 'utf8', maxBuffer: 64 << 20 });

const licenseFiles = (dir) =>
  readdirSync(dir)
    .filter((f) => /^(licen[cs]e|copying|notice)/i.test(f))
    .sort()
    .map((f) => join(dir, f));

const packages = [];

// Go itself: its runtime and standard library are in every binary.
packages.push({ name: 'Go', version: run('go', ['env', 'GOVERSION']).trim(), dir: run('go', ['env', 'GOROOT']).trim() });

// Go: the modules the binary links, with their versions.
const linked = run('go', ['list', '-deps', '-f', '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}', './cmd/shipwake']);
for (const line of [...new Set(linked.split('\n').filter(Boolean))].sort()) {
  const [name, version, dir] = line.split(' ');
  packages.push({ name, version, dir });
}

// The web app: the production dependencies the page bundles.
const web = JSON.parse(run('pnpm', ['licenses', 'list', '--prod', '--json'], join(root, 'web')));
for (const list of Object.values(web)) {
  for (const p of list) {
    packages.push({ name: p.name, version: p.versions.join(', '), dir: p.paths[0] });
  }
}

const parts = [
  '# Third-party notices',
  '',
  'Shipwake includes the following software. Each is distributed under its own license, reproduced here.',
];
for (const p of packages) {
  const files = licenseFiles(p.dir);
  if (files.length === 0) throw new Error(`no license file for ${p.name} ${p.version} in ${p.dir}`);
  parts.push('', `## ${p.name} ${p.version}`);
  for (const f of files) parts.push('', '```text', readFileSync(f, 'utf8').trimEnd(), '```');
}
mkdirSync(dirname(out), { recursive: true });
writeFileSync(out, `${parts.join('\n')}\n`);
console.log(`${packages.length} packages → ${out}`);
