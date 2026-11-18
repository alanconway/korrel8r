# Releasing a New Version

Release version `X.Y.Z` from `main`, or a patch from a `vX.Y` branch.
The root module, `pkg/api`, and `pkg/mcp` modules share a version but have
separate release commits. Their tags are published in dependency order: API,
MCP, root.

## Prepare

1. On branch `main` or `vX.Y`, update the `Unreleased` section of `CHANGELOG.md`.
   Commit and push, you must have a clean tree to run the next step.

2. With test cluster access, unset any local `REGISTRY_BASE` setting so the
   release uses the default `quay.io/korrel8r`:

   ```sh
   unset REGISTRY_BASE; make pre-release VERSION=X.Y.Z
   ```

   - Sets the Makefile version and dates the changelog, then tidies, generates,
     tests, and builds a local image against the last published module versions.
   - Creates a local `Prepare release X.Y.Z` commit. **No commits, tags, or images
     are pushed** by this target. Review this commit before publishing.

   Do not use `NO_CLUSTER=1`: a release must pass all tests. If validation fails,
   review the partial local changes before retrying; the script requires a clean tree.

## Publish tags and images

After reviewing the prepared commit, with image registry access, run:

```sh
unset REGISTRY_BASE; make push-release
```

This requires a clean `Prepare release X.Y.Z` commit at HEAD, a matching
`CHANGELOG` entry, and cluster access for final validation. `hack/push-release.sh` then:

1. Tags and pushes `pkg/api/vX.Y.Z` with the prepared commit.
2. Updates MCP to require the published API version, tidies and tests MCP,
   then commits, tags, and pushes `pkg/mcp/vX.Y.Z`.
3. Updates the root requirements to the published API and MCP versions, tidies,
   regenerates, runs all tests, and builds the final image. Commits and pushes
   `Release X.Y.Z`, publishes versioned and `latest` images, then tags and
   pushes `vX.Y.Z` for the root module.

Go cannot resolve unpublished module versions during `go mod tidy`. Therefore
checks against the final tagged dependency versions **must** run in `push-release`,
after the module tags exist; `pre-release` validates everything possible without
publishing. If `push-release` fails partway through, do not move or delete the
published tags or restart from scratch. Diagnose and finish the remaining stage;
changes to already-tagged module source require a new version.

## Post-release

On `main` or `vX.Y`, run:

```sh
unset REGISTRY_BASE; make post-release
```

This bumps `VERSION?=` to `X.Y.(Z+1)` (no `-dev`),
runs `make all`, and commits and pushes the version bump. Leave Go module
requirements at the last released version until preparing the next release.

## Patch an Older Version

Create branch `vX.Y` from tag `vX.Y.0` if it does not already exist. Follow the
same preparation and publication steps on that branch.
