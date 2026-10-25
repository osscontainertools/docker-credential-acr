---
model: sonnet
effort: high
---

Prepare a docker-credential-acr release.

Usage: `/release <version>`

`<version>` is `X.Y.Z` without the `v`. Branch `release-v<version>`, tag `v<version>`. The release workflow only triggers on `v*` tags and Go modules only resolve `vX.Y.Z`.

The version lives only in the tag, goreleaser stamps it via `-X main.version`. Never move or delete a published tag, the module proxy caches it permanently.

Don't run tests, lint or the cross compile locally. Check `unit-tests` and `build` are green on `main` instead.

0. **Create release branch**:
   ```
   git checkout main && git pull
   git checkout -b release-v<version>
   ```

1. **Get the change list**:
   ```bash
   PREV=$(gh api repos/osscontainertools/docker-credential-acr/releases/latest --jq .tag_name)
   SINCE=$(gh pr list --repo osscontainertools/docker-credential-acr --state merged \
     --search "release $PREV" --json mergedAt --jq '.[0].mergedAt')
   gh pr list --repo osscontainertools/docker-credential-acr --state merged --limit 200 \
     --json number,title,mergedAt,author \
     --jq ".[] | select(.mergedAt > \"$SINCE\") | \"\(.mergedAt) \(.number) \(.author.login) \(.title)\"" | sort
   ```
   Filter by `mergedAt`, never by PR number.

2. **CHANGELOG.md scaffolding** — insert at top, two blank lines before the previous release:
   ```
   # v<version> Release YYYY-MM-DD

   ## Update Notice   only when a flag is activated or removed, or a breaking change lands
   ## What's Changed
   ### Security      CVE fixes, hardening
   ### Bugfixes      incorrect behavior now fixed
   ### Usability     new user-facing capability, changed default, flag graduation
   ### Maintenance   build(deps): bumps
   ### Fork Related  CI, test infra, internal tooling


   # v<prev> Release ...
   ```

3. **What's Changed** — one line per PR, mechanical, no explanation.
   - Format: `* <title>: <PR URL> [<PR URL> ...]`.
   - Order by `mergedAt` ascending, merged bumps by their earliest PR. Drop empty categories.
   - Titles state what the user observes or gains. No em dashes or semicolons.
   - Expand grouped bumps into one line per package via `gh pr view <n>`.
   - Merge repeated bumps of one package into a single range: `build(deps): bump <pkg> from <first> to <last>`.
   - Every version change in `git diff $PREV..main -- go.mod .github/workflows/` must appear in Maintenance. If not, stop and report the gap.

4. **Update Notice** — the explanation goes here, never in What's Changed. Only for changes this repo made. Upstream changes (Go platform minimums, stdlib behavior) stay a Maintenance line. Follow the shape of kaniko's v1.28.0 notice:
   ~~~
   In this Release we activated this feature-flag:
   * [FF_DOCKER_ACR_X](https://github.com/osscontainertools/docker-credential-acr#flag-ff_docker_acr_x)

   <what it does, and whether migration is needed>

   You can roll-back this change by overriding it in the environment ie.
   ```yaml
   job:
     variables:
       FF_DOCKER_ACR_X: "0"
   ```
   Please also notify us by [filing a new issue](https://github.com/osscontainertools/docker-credential-acr/issues/new/choose).

   We further removed this feature-flag:
   * `FF_DOCKER_ACR_Y`

   <its behavior is now unconditional, the flag has no effect and can be removed>
   ~~~
   Also cover any change to the `credHelpers` value (`acr`), the binary name or the module path. The README documents none of them.

5. **Security** — scan the previous tag to find what this release fixes. Build govulncheck with the toolchain in `go.mod`, an older one cannot load the packages:
   ```bash
   BIN=$(mktemp -d)
   GOTOOLCHAIN=$(awk '/^toolchain/{print $2}' go.mod) GOBIN=$BIN go install golang.org/x/vuln/cmd/govulncheck@latest
   git worktree add --detach ../prev $PREV
   (cd ../prev && $BIN/govulncheck -format json ./... | jq -r 'select(.finding) | .finding | "\(.osv) \(.trace[0].module)@\(.trace[0].version) fixed \(.fixed_version)"' | sort -u)
   git worktree remove --force ../prev
   curl -s https://vuln.go.dev/ID/<GO-ID>.json | jq '.aliases'
   ```
   Keep findings whose fixed version is now in `go.mod`. Format: `* <module> <vulnerable version>: CVE-YYYY-NNNNN [CVE-...]`, stdlib as `go stdlib`. Add the name users know it by in parentheses, e.g. `CVE-2026-111943 (eMudhra)`. For a GHSA, look up the CVE with `curl -s https://api.github.com/advisories/<GHSA> | jq -r .cve_id`.

6. **Consistency check** — compare style against the previous two releases and kaniko's `CHANGELOG.md`.

7. **Commit and open the PR**:
   ```
   git add CHANGELOG.md
   git commit -m "release"
   git push -u origin release-v<version>
   gh pr create --repo osscontainertools/docker-credential-acr --base main --title "release v<version>" --body ""
   ```
   Merge once `unit-tests` and `build` are green.

8. **Live tests** on the merged commit. CI has no Azure credentials. Needs `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_TENANT_ID`, `ACR_TEST_REGISTRY`:
   ```
   git checkout main && git pull
   make integration-test
   ```

9. **Tag via the API**:
   ```bash
   REPO=osscontainertools/docker-credential-acr
   SHA=$(gh api repos/$REPO/commits/main --jq .sha)
   TAG=$(gh api repos/$REPO/git/tags -f tag=v<version> -f message=v<version> \
     -f object=$SHA -f type=commit --jq .sha)
   gh api repos/$REPO/git/refs -f ref=refs/tags/v<version> -f sha=$TAG
   ```

10. **Watch the release run**, it creates a draft:
    ```bash
    gh run watch $(gh run list --workflow=release.yaml --limit 1 --json databaseId --jq '.[0].databaseId')
    gh release view v<version>
    ```
    The draft must carry one archive per target in `.goreleaser.yml` plus `checksums.txt`.

    > ⚠️ Never create or replace the release yourself, unlike kaniko this repo is tag-first. Only goreleaser's draft carries the binaries, the run keeps no artifacts.

    If the draft was lost anyway, rebuild from the tag and upload. Never re-run the workflow, it overwrites the notes:
    ```bash
    gh release view v<version> --json body --jq .body > body.md
    git worktree add --detach ../wt v<version>
    cd ../wt && goreleaser release --clean --skip=publish,announce,validate
    gh release upload v<version> dist/*.tar.gz dist/checksums.txt
    ```
    Diff the body against `body.md`. Tell the user the binaries are locally built and `checksums.txt` was regenerated. The next patch release restores CI provenance.

11. **Publish** — paste the CHANGELOG section as notes:
    ```
    gh release edit v<version> --notes-file <file> --draft=false
    ```

12. **Bump consumers** — in the kaniko fork: the module in `go.mod`, the import in `pkg/creds/creds.go` and `tools/tools.go`, then `go mod vendor`. Keep installing the binary under any previous name in `deploy/Dockerfile`.

## Security releases

1. **Fix downstreams first** — release affected consumers before this repo's fix is public.
2. **Advisory, then vulndb** — publish a GitHub security advisory, then file https://go.dev/s/vulndb-report-new and request a CVE in the same submission. Name every affected module path, forks included.
3. **Contact affected forks privately** with a disclosure date, not an open ended embargo.
4. **Keep exploit detail out of commits and tracked files** until the advisory is public.
