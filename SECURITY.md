# Security

## Reporting a vulnerability

Please report it privately, not in a public issue: use
[Report a vulnerability](https://github.com/blu9a/shipwake/security/advisories/new) on this repository.
Describe what an attacker can do, the version or commit, and how to reproduce it.

You'll get an answer within a week. Once it's fixed and released, the advisory is published with credit
to you, unless you prefer otherwise.

## Supported versions

Fixes go into the newest release. Before 1.0, older versions are not patched.

## How Shipwake is exposed

- `shipwake build` writes a static site: no server, no accounts, nothing executed in the viewer's browser
  beyond the site itself. What it shows comes from the repository and its recorded runs, so publish a site
  only where that content may be seen.
- `shipwake serve` listens on `127.0.0.1` by default. It can change the local repository on request (save and
  approve proposals, create a release tag), accepting only same-origin JSON requests. Don't expose it on a
  network.
- Shipwake never pushes to a remote unless asked (`--push`, `push: true`).
