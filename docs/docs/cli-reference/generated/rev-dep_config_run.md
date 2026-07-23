---
title: "rev-dep config run"
description: "Execute all checks defined in (.)rev-dep.config.json(c)"
---

Execute all checks defined in (.)rev-dep.config.json(c)

### Synopsis

Process (.)rev-dep.config.json(c) and execute all enabled checks (circular imports, orphan files, module boundaries, import conventions, node modules, unused exports, unresolved imports, restricted imports and restricted dev deps usage) per workspace.

```
rev-dep config run [flags]
```

### Options

```
  -c, --cwd string                  Working directory (default "$PWD")
      --fix                         Automatically fix fixable issues
      --format string               Output format (json, issues-list)
  -h, --help                        help for run
      --lint-config config lint     Also lint the config after running; prints only error/warning counts and fails (non-zero exit) on any lint error. Use config lint for details and --fix
      --lint-config-rules strings   Which lint rules to run with --lint-config (comma-separated). Default: all. Implies --lint-config
      --list-all-issues             List all issues instead of limiting output
      --recheck                     Run all checks again after '--fix' to validate the final state
  -v, --verbose                     Show warnings and verbose output
      --workspaces strings          Subset of workspaces to run (comma-separated list of workspace paths)
```
