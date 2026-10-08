---
on:
  push:
    branches: [try/gh-aw-t0]

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
      description: "Push the prepared patch (/tmp/gh-aw/aw-export.patch) to a branch and publish a one-click create-PR link. Call exactly once, after writing the patch file."
      runs-on: ubuntu-latest
      if: needs.agent.result == 'success' && needs.detection.result == 'success' && needs.detection.outputs.detection_success == 'true'
      permissions:
        contents: write
        pull-requests: read
      env:
        BRANCH_RE: '^aw-t0/[A-Za-z0-9._-]+$'
        BASE_RE: '^(master|try/gh-aw-t0|release/v[0-9]+\.[0-9]+)$'
        ALLOWED_PATHS_RE: '^hack/aw-t0/'
      inputs:
        branch:
          description: "Branch name to push. Must match aw-t0/<name>"
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
            [ -n "$ITEM" ] || { echo "::error::no push_branch item"; exit 1; }
            BRANCH=$(jq -r '.branch' <<<"$ITEM"); BASE=$(jq -r '.base' <<<"$ITEM"); TITLE=$(jq -r '.title' <<<"$ITEM" | head -c 200)
            [[ "$BRANCH" =~ $BRANCH_RE ]] || { echo "::error::branch not allowed: $BRANCH"; exit 1; }
            [[ "$BASE" =~ $BASE_RE ]] || { echo "::error::base not allowed: $BASE"; exit 1; }
            PATCH=$(find "$(dirname "$GH_AW_AGENT_OUTPUT")" -type f -name 'aw-export.patch' | head -1)
            [ -n "$PATCH" ] || { echo "::error::aw-export.patch not found in agent artifact"; exit 1; }
            { echo "branch=$BRANCH"; echo "base=$BASE"; echo "patch=$PATCH"; echo 'title<<EOF_T'; echo "$TITLE"; echo 'EOF_T'; } >> "$GITHUB_OUTPUT"
        - uses: actions/checkout@v5
          with:
            ref: ${{ steps.req.outputs.base }}
            path: repo
        - name: Apply, guard, commit, push (idempotent)
          working-directory: repo
          env:
            BRANCH: ${{ steps.req.outputs.branch }}
            BASE: ${{ steps.req.outputs.base }}
            PATCH: ${{ steps.req.outputs.patch }}
            TITLE: ${{ steps.req.outputs.title }}
            GH_TOKEN: ${{ github.token }}
          run: |
            set -euo pipefail
            git apply --check "$PATCH"
            git apply --index "$PATCH"
            CHANGED=$(git diff --cached --name-only --no-renames)
            [ -n "$CHANGED" ] || { echo "::warning::empty patch, nothing to push"; exit 0; }
            echo "Changed files:"; echo "$CHANGED"
            if grep -E '^\.github/workflows/' <<<"$CHANGED"; then
              echo "::error::patch touches .github/workflows; GITHUB_TOKEN cannot push it (A2)"; exit 1
            fi
            BAD=$(grep -v -E "$ALLOWED_PATHS_RE" <<<"$CHANGED" || true)
            [ -z "$BAD" ] || { echo "::error::paths outside allowlist ($ALLOWED_PATHS_RE):"; echo "$BAD"; exit 1; }
            git -c user.name="github-actions[bot]" -c user.email="41898282+github-actions[bot]@users.noreply.github.com" commit -q -m "$TITLE"

            enc() { jq -rn --arg s "$1" '$s|@uri'; }
            BODY="Automated change from run ${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"
            LINK="${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/compare/${BASE}...${BRANCH}?expand=1&title=$(enc "$TITLE")&body=$(enc "$BODY")"
            OWNER="${GITHUB_REPOSITORY%%/*}"
            OPEN_PR=$(gh api "repos/${GITHUB_REPOSITORY}/pulls?state=open&head=${OWNER}:${BRANCH}" --jq '.[0].html_url // empty')
            OLD=$(git ls-remote --heads origin "refs/heads/$BRANCH" | cut -f1)

            if [ -n "$OLD" ]; then
              git fetch -q --depth=1 origin "refs/heads/$BRANCH"
              if [ "$(git rev-parse 'HEAD^{tree}')" = "$(git rev-parse 'FETCH_HEAD^{tree}')" ]; then
                ACTION="unchanged: $BRANCH already has identical content ($OLD); not pushing"
              elif [ -n "$OPEN_PR" ]; then
                echo "::warning::$BRANCH has an open PR ($OPEN_PR) and different content; not overwriting a branch under review"
                ACTION="skipped: open PR $OPEN_PR"
              else
                git push origin "HEAD:refs/heads/$BRANCH" --force-with-lease="refs/heads/$BRANCH:$OLD"
                ACTION="updated: $BRANCH $OLD -> $(git rev-parse HEAD)"
              fi
            else
              git push origin "HEAD:refs/heads/$BRANCH" --force-with-lease="refs/heads/$BRANCH:"
              ACTION="created: $BRANCH at $(git rev-parse HEAD)"
            fi
            echo "::notice title=push-branch::$ACTION"
            if [ -n "$OPEN_PR" ]; then
              echo "::notice title=Existing PR::$OPEN_PR"
              { echo "### Existing PR"; echo; echo "$OPEN_PR"; } >> "$GITHUB_STEP_SUMMARY"
            else
              echo "::notice title=Create PR::$LINK"
              { echo "### One-click PR"; echo; echo "$ACTION"; echo; echo "[Create pull request]($LINK)"; } >> "$GITHUB_STEP_SUMMARY"
            fi
---

# T0: export a patch from the agent without opening a PR

This is a plumbing test. Do exactly these steps and nothing else.

1. Create the file `hack/aw-t0/probe.txt` containing exactly one line: `aw-t0 probe v2`.
2. In the repository root, run:
   ```bash
   git add -A
   git diff --cached --binary > /tmp/gh-aw/aw-export.patch
   cat /tmp/gh-aw/aw-export.patch
   ```
3. Call the `push_branch` tool once with:
   - `branch`: `aw-t0/probe`
   - `base`: `master`
   - `title`: `chore: aw-t0 probe`

Do not open issues or pull requests. Do not modify any other file.
