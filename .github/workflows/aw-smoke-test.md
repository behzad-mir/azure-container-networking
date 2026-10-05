---
on:
  workflow_dispatch:

permissions:
  contents: read
  copilot-requests: write

engine: copilot

network: defaults

tools:
  github:
    toolsets: [repos]

safe-outputs:
  create-issue:
    title-prefix: "[aw-smoke] "
    max: 1
---

# Agentic Workflows smoke test

This workflow only checks that GitHub Agentic Workflows can run in this repository. Do not change any files.

1. Read `go.mod` at the repository root.
2. Report the `go` directive, the `toolchain` directive, and the versions of `golang.org/x/crypto` and `google.golang.org/grpc`.
3. Open one issue titled `Go module snapshot` with those four values in a Markdown table, plus the commit SHA you read them from.
