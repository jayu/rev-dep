---
title: "rev-dep config lint"
description: "Report (and optionally remove) config glob/path patterns that match nothing"
---

Report (and optionally remove) config glob/path patterns that match nothing

### Synopsis

Scan a (.)rev-dep.config.json(c) for "dead" glob and path patterns — ignore
patterns, entry point patterns, workspace paths, graph excludes, denied files/modules and
similar — that no longer match any discovered file or module. Over time configs
accumulate patterns for files that were renamed or deleted; this command surfaces them
so the config stays lean.

With --fix, dead patterns are removed in place, preserving all comments and formatting.
Some patterns are reported but never auto-removed because deleting them could change a
check's behavior or make the config invalid — workspace paths, required entry points / files
/ modules, and module-boundary selectors. These are marked "not auto-removed"; resolve
them by hand.

```
rev-dep config lint [flags]
```

### Options

```
  -c, --cwd string      Working directory (default "$PWD")
      --fix             Remove dead patterns from the config file (preserves comments and formatting)
  -h, --help            help for lint
      --rules strings   Lint rules to run (comma-separated): orphan-file-globs, orphan-module-globs, overlapping-globs, trailing-commas, compact. Default: all. orphan-file-globs/overlapping-globs use file discovery; orphan-module-globs parses the dependency tree; trailing-commas and compact only read the config file.
  -v, --verbose         Show warnings and verbose output
```
