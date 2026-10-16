# Changelog

All notable changes to this project are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-10-16

### Added
- Desktop app built with Wails v2: port scanner, live listening-port sidebar, search, and confirmed process termination.
- Service name shown in the desktop Result card and the kill confirmation dialog.
- Known process name resolution on Windows using `tasklist`.
- Additional built-in service labels for common development, database, streaming, and smart-home ports.
- Custom service labels for the desktop app from `.portpilot.json`.
- New application icon for the desktop app and Windows executable.
- Windows executable metadata (product name, version, company, copyright) set from `wails.json`.

### Changed
- Port check messages now include the port number.
- Windows listener detection no longer depends on the English `LISTENING` label, so it works on non-English Windows.
- Desktop app now uses the shared `internal/ports` checker, with `AVAILABLE`, `OCCUPIED`, and `ERROR` statuses.
- Desktop app: right-click context menu is disabled.
- Desktop app: Ctrl+A (Cmd+A on macOS) is blocked outside text fields, so search and port inputs still allow select-all.

### Fixed
- Windows: `ListActivePorts` no longer returns duplicate rows when a process listens on both IPv4 and IPv6. Results are deduplicated by port and PID.
- Windows: `FindProcessByPort` matches ports exactly. Previously, checking port `3000` could match a process on port `30000`, so `kill` could target the wrong process.
- Windows: `FindProcessByPort` uses the same locale-independent listener detection as `ListActivePorts`.
- Windows: process name lookup no longer treats "No tasks" messages as a process name.
- `kill` command: `--force` and `--config` flags are now parsed by the `kill` flag set, so they take effect.
- Desktop app: `kill` re-checks port ownership before terminating a process, and refuses to act if the owner has changed.

### Platform support
- CLI: Linux, macOS, Windows.
- Desktop app: tested on Windows 11 and Linux. Not yet tested on macOS.
