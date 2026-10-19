## Description

<!-- Provide a brief overview of the changes introduced by this pull request. -->

## Related Issue

<!-- Link related issue(s) here. E.g., Fixes #123, Resolves #456 -->
Fixes #

## Type of Change

<!-- Please check the options that apply. -->

- [ ] 🐛 Bug fix (non-breaking change which fixes an issue)
- [ ] ✨ New feature (non-breaking change which adds functionality)
- [ ] ⚡ Performance improvement (zero-allocation hot-path tuning, caching, etc.)
- [ ] 💥 Breaking change (fix or feature that would cause existing functionality to change)
- [ ] 📝 Documentation update (improving guides, recipes, or README)
- [ ] 🧪 Testing (adding or improving unit, benchmark, fuzz, or chaos tests)
- [ ] 🔧 Tooling & CI (changes to build scripts, Docker, or workflows)

## Performance & Latency Impact

<!--
If this PR modifies hot-path components (internal/pipeline, internal/policy, internal/mcp, router):
Include before/after benchmark results from `make bench`.
-->

- [ ] No impact on hot path / latency-neutral
- [ ] Benchmark results attached below:

```text
<!-- Paste benchmark comparison here if applicable -->
```

## Checklist

- [ ] My code follows the [Contribution Guidelines](CONTRIBUTING.md) and Go code style.
- [ ] I have run `go fmt ./...` and `make vet`.
- [ ] All unit and integration tests pass with the race detector (`make test`).
- [ ] I have added tests that prove my fix is effective or that my feature works.
- [ ] I have updated relevant documentation or code comments.
- [ ] My commit messages adhere to [Conventional Commits](https://www.conventionalcommits.org/).
