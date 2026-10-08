---
on:
  push:
    branches: [try/gh-aw-t0]
    paths: [".github/workflows/aw-t0-patch-export.lock.yml"]

permissions:
  contents: read
  copilot-requests: write

engine: copilot

network: defaults

tools:
  edit:
  bash: true

safe-outputs:
  jobs:
    push-branch:
      description: "Push the prepared patch (/tmp/gh-aw/aw-export.patch) to a new branch and publish a one-click create-PR link. Call exactly once, after writing the patch file."
      runs-on: ubuntu-latest
      if: needs.detection.result == 'success' && needs.detection.outputs.detection_success == 'true'
      permissions:
        contents: write
      inputs:
        branch:
          description: "Branch name to push. Must start with aw-t0/"
          required: true
          type: string
        base:
          description: "Base branch the patch was made against"
          required: true
          type: string
        title:
          description: "Short PR title"
          required: true
          type: string
      steps:
        - name: Locate request and patch
          id: req
          run: |
            set -euo pipefail
            ITEM=$(jq -c '[.items[] | select(.type == "push_branch")][0] // empty' "$GH_AW_AGENT_OUTPUT")
            if [ -z "$ITEM" ]; then echo "::error::no push_branch item"; exit 1; fi
            BRANCH=$(jq -r '.branch' <<<"$ITEM"); BASE=$(jq -r '.base' <<<"$ITEM"); TITLE=$(jq -r '.title' <<<"$ITEM")
            [[ "$BRANCH" =~ ^aw-t0/[A-Za-z0-9._/-]+$ ]] || { echo "::error::branch not allowed: $BRANCH"; exit 1; }
            [[ "$BASE" =~ ^(master|try/gh-aw-t0|release/v[0-9]+\.[0-9]+)$ ]] || { echo "::error::base not allowed: $BASE"; exit 1; }
            ROOT=$(dirname "$GH_AW_AGENT_OUTPUT")
            echo "Artifact root: $ROOT"; find "$ROOT" -maxdepth 6 -type f | head -50
            PATCH=$(find "$ROOT" -type f -name 'aw-export.patch' | head -1)
            [ -n "$PATCH" ] || { echo "::error::aw-export.patch not found in agent artifact"; exit 1; }
            echo "branch=$BRANCH" >> "$GITHUB_OUTPUT"; echo "base=$BASE" >> "$GITHUB_OUTPUT"
            echo "patch=$PATCH" >> "$GITHUB_OUTPUT"
            { echo 'title<<EOF_T'; echo "$TITLE"; echo 'EOF_T'; } >> "$GITHUB_OUTPUT"
        - uses: actions/checkout@v5
          with:
            ref: ${{ steps.req.outputs.base }}
            path: repo
        - name: Apply, guard, commit, push
          working-directory: repo
          env:
            BRANCH: ${{ steps.req.outputs.branch }}
            BASE: ${{ steps.req.outputs.base }}
            PATCH: ${{ steps.req.outputs.patch }}
            TITLE: ${{ steps.req.outputs.title }}
          run: |
            set -euo pipefail
            git apply --check "$PATCH"
            git apply --index "$PATCH"
            if git diff --cached --name-only | grep -E '^\.github/workflows/'; then
              echo "::error::patch touches .github/workflows; GITHUB_TOKEN cannot push it (A2)"; exit 1
            fi
            git -c user.name="github-actions[bot]" -c user.email="41898282+github-actions[bot]@users.noreply.github.com" commit -q -m "$TITLE"
            git push origin "HEAD:refs/heads/$BRANCH" --force-with-lease
            enc() { jq -rn --arg s "$1" '$s|@uri'; }
            BODY="Automated change from run ${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"
            LINK="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/compare/${BASE}...${BRANCH}?expand=1&title=$(enc "$TITLE")&body=$(enc "$BODY")"
            echo "::notice title=Create PR::$LINK"
            { echo "### One-click PR"; echo; echo "[Create pull request]($LINK)"; echo; git show --stat HEAD; } >> "$GITHUB_STEP_SUMMARY"
---

# T0: export a patch from the agent without opening a PR

This is a plumbing test. Do exactly these steps and nothing else.

1. Create the file `hack/aw-t0/probe.txt` containing one line: `aw-t0 probe from run ${{ github.run_id }}`.
2. In the repository root, run:
   ```bash
   git add -A
   git diff --cached --binary > /tmp/gh-aw/aw-export.patch
   cat /tmp/gh-aw/aw-export.patch
   ```
3. Call the `push_branch` tool once with:
   - `branch`: `aw-t0/probe-${{ github.run_id }}`
   - `base`: `try/gh-aw-t0`
   - `title`: `chore: aw-t0 probe`

Do not open issues or pull requests. Do not modify any other file.
