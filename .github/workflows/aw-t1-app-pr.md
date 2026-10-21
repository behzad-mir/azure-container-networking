---
on:
  push:
    branches: [try/gh-aw-t1]

permissions:
  contents: read
  copilot-requests: write

engine: copilot

network: defaults

checkout:
  ref: master

tools:
  edit:
  bash: true

safe-outputs:
  github-app:
    client-id: ${{ vars.ACN_APP_CLIENT_ID }}
    private-key: ${{ secrets.ACN_APP_PRIVATE_KEY }}
  create-pull-request:
    title-prefix: "[aw-t1] "
    labels: [automation]
    draft: true
    base-branch: master
    preserve-branch-name: true
    allowed-branches: ["aw-t1/*"]
    allowed-files: ["hack/aw-t1/**"]
    fallback-as-issue: false
    max: 1
---

# T1: open a pull request with a GitHub App token

This is an automated test. Make one tiny, deterministic change and propose it as a pull request.

1. Create the file `hack/aw-t1/probe.txt` containing exactly one line: `aw-t1 probe v1`.
2. Create a branch named `aw-t1/probe` and commit the file to it.
3. Call the `create_pull_request` tool once with:
   - `branch`: `aw-t1/probe`
   - `title`: `chore: aw-t1 app PR probe`
   - `body`: `Automated test of gh-aw create-pull-request using a GitHub App token. Safe to close.`

Do not modify any other file. Do not open issues.
