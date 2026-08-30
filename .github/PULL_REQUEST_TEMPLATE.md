# Pull Request

## Description

<!-- Provide a clear and concise summary of what this PR changes and why. -->

## Related issue

<!-- Link the issue this PR addresses. Use "Closes #123" or "Fixes #123" so GitHub links them automatically. -->

Closes #

## Type of change

<!-- Check all that apply. -->

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Refactor (no functional changes)
- [ ] Documentation update
- [ ] Build / CI / tooling change

## Checklist

- [ ] I have read the [CONTRIBUTING guidelines](https://github.com/user/commitforge#contributing) (if present) and the [Code of Conduct](https://github.com/user/commitforge/blob/main/CODE_OF_CONDUCT.md)
- [ ] There is an open issue describing the problem this PR solves, or I have created one
- [ ] I have added tests that cover my changes (`go test ./... -cover` passes)
- [ ] `go vet ./...` reports no issues
- [ ] `gofmt -l .` reports no files (and `goimports` is clean)
- [ ] `golangci-lint run` passes (or `make lint`)
- [ ] Documentation (README.md and/or doc comments) is updated where needed
- [ ] New and existing unit tests pass locally with my changes
- [ ] I have updated [CHANGELOG.md](https://github.com/user/commitforge/blob/main/CHANGELOG.md) under the `[Unreleased]` section
