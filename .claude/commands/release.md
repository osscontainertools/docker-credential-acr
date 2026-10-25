---
model: sonnet
effort: high
---

Prepare a docker-credential-acr release.

Usage: `/release <version>`

`<version>` is `X.Y.Z`, without the `v`. Everything that names the release adds the prefix: the branch is `release-v<version>` and the tag is `v<version>`. The prefix is not cosmetic, the release workflow triggers on `push: tags: ['v*']` and Go modules only resolve `vX.Y.Z`, so a bare numeric tag builds nothing and cannot be imported.

The version lives only in the git tag, no file in the repo records it, and goreleaser stamps it into the binary via `-X main.version`. Never move or delete a published tag, the module proxy caches it permanently.

Do not run the unit tests, the linter or the cross compile locally. `unit-tests.yaml` and `build.yaml` run all three on every push to `main`, and `build.yaml` validates `.goreleaser.yml` on the way. Check they are green instead.

0. **Create release branch** from `main`:
   ```
   git checkout main && git pull
   git checkout -b release-v<version>
   ```

1. **Get the change list** — everything merged since the previous tag:
   ```bash
   PREV=$(gh api repos/osscontainertools/docker-credential-acr/releases/latest --jq .tag_name)
   git log --oneline $PREV..main
   gh pr list --repo osscontainertools/docker-credential-acr --state merged --limit 100 \
     --json number,title,mergedAt,author --jq '.[] | "\(.mergedAt) \(.number) \(.author.login) \(.title)"' | sort
   ```
   Filter by `mergedAt`, never by PR number, PRs merge out of order.

2. **CHANGELOG.md scaffolding** — insert at top, two blank lines before the previous release:
   ```
   # v<version> Release YYYY-MM-DD

   ## What's Changed
   ### Security        CVE fixes, hardening
   ### Bugfixes        incorrect behavior now fixed
   ### Usability       new user-facing capability or a changed default
   ### Maintenance     dependency bumps
   ### Fork Related    CI, test infra, internal tooling, no user visible effect


   # v<prev> Release ...
   ```

3. **Write the entries** — describe what the user observes or gains, not internals. No em dashes or semicolons. Drop empty categories. Order within a category by `mergedAt` ascending.

4. **Call out breaking changes** at the top of `## What's Changed`. The README documents neither the `credHelpers` value nor any migration, so the changelog is the only place a user learns about:
   - the value to put in `config.json` under `credHelpers` or `credsStore`, currently `acr`
   - a change to the binary name, the module path, or a flag

5. **Security CVEs** — check whether the dependency bumps in this release close any:
   ```bash
   govulncheck ./...
   git diff $PREV..main -- go.mod
   ```
   For a GHSA without an obvious CVE, look up the canonical id:
   ```bash
   curl -s https://api.github.com/advisories/GHSA-XXXX-XXXX-XXXX | jq -r '.cve_id'
   ```
   Format: `* github.com/foo/bar v1.2.3: CVE-YYYY-NNNNN`, where the version is the vulnerable one being replaced.

6. **Commit and open the PR**:
   ```
   git add CHANGELOG.md
   git commit -m "release"
   git push -u origin release-v<version>
   gh pr create --repo osscontainertools/docker-credential-acr \
     --base main \
     --title "release v<version>" \
     --body ""
   ```
   Merge it once `unit-tests` and `build` are green on the PR.

7. **Run the live tests** on the merged commit — the only coverage CI cannot provide, since Azure credentials are deliberately kept out of it. Needs `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_TENANT_ID` and `ACR_TEST_REGISTRY` exported:
   ```
   git checkout main && git pull
   make integration-test
   ```

8. **Tag the merged commit** via the API, so the tag comes from the remote state rather than a local checkout:
   ```bash
   REPO=osscontainertools/docker-credential-acr
   SHA=$(gh api repos/$REPO/commits/main --jq .sha)
   TAG=$(gh api repos/$REPO/git/tags -f tag=v<version> -f message=v<version> \
     -f object=$SHA -f type=commit --jq .sha)
   gh api repos/$REPO/git/refs -f ref=refs/tags/v<version> -f sha=$TAG
   ```

9. **Watch the release run** and review the draft it produces. goreleaser is configured with `draft: true`, so nothing is public until you publish:
    ```bash
    gh run watch $(gh run list --workflow=release.yaml --limit 1 --json databaseId --jq '.[0].databaseId')
    gh release view v<version>
    ```

    > ⚠️ **Never create the release yourself, and do not replace the draft.** This repo is tag-first, the opposite of kaniko. Here the tag triggers goreleaser, which *creates* the release and is the only thing that attaches the platform archives, one per target in `.goreleaser.yml`, and `checksums.txt`. In kaniko you create the release in the UI and it tags `main`, which is safe because a kaniko release carries no assets at all, its artifacts are container images pushed by `images.yaml`. Bringing that habit here yields an empty release: the assets exist only on goreleaser's draft, the run saves no Actions artifacts, so deleting or bypassing that draft loses the binaries even though the run reports success.

    So after tagging, leave the Releases page alone until the run finishes, then publish **goreleaser's draft**.

    If it happens anyway, rebuild from the tag and upload without touching the notes, never re-run the workflow, goreleaser would regenerate the body from its own changelog and clobber them:
    ```bash
    gh release view v<version> --json body --jq .body > /tmp/body.md   # back up first
    git worktree add --detach /tmp/wt v<version>                       # build the tag, not your working tree
    cd /tmp/wt && goreleaser release --clean --skip=publish,announce,validate
    gh release upload v<version> dist/*.tar.gz dist/checksums.txt
    ```
    Then diff the body against the backup to confirm the notes survived. Flag to the user that these are locally built binaries rather than CI built, and that `checksums.txt` was regenerated. Cutting the next patch version is the clean way to get CI provenance back, never re-point a published tag.

10. **Publish** once the notes read correctly:
    ```
    gh release edit v<version> --notes-file <file> --draft=false
    ```

11. **Bump consumers** — for a kaniko fork that is the module in `go.mod`, the import in `pkg/creds/creds.go` and `tools/tools.go`, then `go mod vendor`. Keep installing the binary under any previous name in `deploy/Dockerfile` so existing `config.json` files still resolve.

## Security releases

1. **Fix downstreams first** — release the affected consumers before this repo's fix is public. A public fix with no consumable release for consumers is an exploit with no remedy.
2. **Advisory, then vulndb** — publish a GitHub security advisory on this repo, then file https://go.dev/s/vulndb-report-new and request a CVE in the same submission. Name every affected module path, including forks carrying the same defect.
3. **Contact maintainers of affected forks privately first** and give them a disclosure date rather than an open ended embargo.
4. **Keep exploit detail out of commit messages and tracked files** until the advisory is public. Making a repository public exposes its entire history at once.
