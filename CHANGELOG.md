# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-08-27

### Added
- Interactive TUI grid for shaping a GitHub contribution calendar
- Backdated empty-commit generation with configurable counts per day (`fixed` or `min-max` range)
- Range selection (`v`) and bulk select/clear (`a` / `u`)
- Push flow: blank-repo and existing-repo modes, streaming git output
- Auto-detection and pre-fill of existing `origin` remote
- JSON state persistence at `<dir>/.commitforge/state.json`
- CLI flags: `--dir`, `--years`, `--message`, `--message-mode`, `--remote`, `--no-push`, `--yes`
- Regenerate command that rebuilds history from saved state
- Clear-all command that wipes generated commits and force-pushes

[Unreleased]: https://github.com/<user>/commitforge/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/<user>/commitforge/releases/tag/v0.1.0
