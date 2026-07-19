## What & why

<!-- What does this change and why? Link any related issues. -->

## Type

- [ ] Feature
- [ ] Fix
- [ ] Refactor / chore
- [ ] Docs
- [ ] Security

## Checklist

- [ ] `gofmt -l .` is clean and `go vet ./...` passes
- [ ] `go build ./... && go test ./...` pass
- [ ] Frozen contracts (`plan/00-ARCHITECTURE.md`) unchanged, or updated there first
- [ ] Generated code in sync (`make site-plans`, `make openapi-sync`) if applicable
- [ ] Tests added/updated for behavior changes
- [ ] No secrets committed
- [ ] Docs / `docs/build-log/` updated if behavior changed

## Security impact

<!-- Any auth, input-handling, crypto, quota, or network-exposure implications? -->
