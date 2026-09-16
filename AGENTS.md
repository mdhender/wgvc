# Repository Instructions

- Always assign newly created GitHub issues and pull requests to `mdhender`.

## Alpha Workflow

- When a request is complete and all relevant tests pass, commit directly to `main` and push without asking for approval.
- When appropriate, commit messages should reference the relevant issue or close it with a GitHub closing keyword.

## Random Number Generation

- Do not use `math/rand`; use `math/rand/v2`.
- Code that uses randomness must accept a seed and use a locally constructed random source. Do not draw from global random sources.
