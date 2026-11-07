# Development instructions

This repository uses Go through Docker Compose. Follow these rules for every code change, including AI-generated changes.

## Before pushing changes

Run **all three commands, in order**, before pushing:

```sh
make tidy
make fmt
make check
```

- Run the commands using the repository's Makefile and Docker Compose; do not rely on a host-installed Go compiler.
- Inspect `git diff` after `make tidy` and `make fmt`. Include any resulting `go.mod`, `go.sum`, and Go source updates in the commit.
- If a command fails, fix the issue and rerun the full sequence. Never push code known to fail the checks.
- If the commands cannot be executed, state this limitation explicitly; **do not claim to have validated them**.

## Go code readability

- Write idiomatic, readable Go with conventional line breaks, indentation, and scoped blocks.
- **Do not compress** multiple statements, declarations, loops, branches, or error checks into a single line merely to make code shorter.
- Use `gofmt` through `make fmt`, but also review code manually: `gofmt` cannot fix every unreadable one-line statement.
- Keep functions understandable and favor clear names and simple control flow.
- Follow existing package conventions and add/update tests when changing behavior.

## Pull requests

- Include relevant tests and documentation updates.
- Verify `make tidy`, `make fmt`, and `make check` **before** pushing and opening/updating a PR.
- Do not merge until the required CI checks have passed.
