# Repository Instructions

- Always assign newly created GitHub issues and pull requests to `mdhender`.

## Alpha Workflow

- When a request is complete and all relevant tests pass, commit directly to `main` and push without asking for approval.
- When appropriate, commit messages should reference the relevant issue or close it with a GitHub closing keyword.
- Bump the version in `version.go` for every commit and include the change in that commit. Increment the patch number unless the user asks for a minor or major bump, and keep the pre-release label (e.g. `alpha`) unless told otherwise.
- After pushing, tag the pushed commit with `v` plus the version without build metadata (e.g. `v0.7.1-alpha`) and push the tag: `git tag v0.7.1-alpha && git push origin v0.7.1-alpha`.

## Random Number Generation

- Do not use `math/rand`; use `math/rand/v2`.
- Code that uses randomness must accept a seed and use a locally constructed random source. Do not draw from global random sources.
