# Task: add tests for otf-mcp

## Goal
Add table-driven unit tests for the auth provider chaining and the
structuredContent output of the MCP tools. Aim for 70% coverage on
those packages.

## Done means
- `go build ./...` succeeds
- `go vet ./...` is clean
- `go test ./...` passes, run twice in a row
- `go test -cover ./...` shows the coverage target for those packages
- Each package's tests are committed separately with a clear message

## Allowed
- Create or edit `*_test.go` files
- Add small test helpers or fakes inside test files
- Edit non-test code ONLY to make it testable (for example extracting
  an interface), and list every such change in NOTES.md

## Not allowed
- Any network access to OrangeTheory services. Use fakes and recorded
  fixtures only.
- Reading or writing the OS keychain. Use an in-memory fake for
  go-keyring.
- Changing behavior, public APIs, or dependencies (go.mod)
- Touching README, CI config, or release files

## If stuck
Write what blocked you in NOTES.md and stop. Don't loop on the same
failure more than 3 times.

## Working rules
- Work on branch agent/tests. Commit after each package.
- Stop after 2 hours or 40 tool iterations, whichever comes first,
  and write a summary in NOTES.md.
