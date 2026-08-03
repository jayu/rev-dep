---
title: "rev-dep node-modules prune-docs"
description: "Remove markdown/docs-like files from installed node_modules packages"
---

Remove markdown/docs-like files from installed node_modules packages

### Synopsis

Removes files from installed node_modules packages based on glob patterns.
Useful for pruning README/LICENSE/docs files to reduce dependency size.

```
rev-dep node-modules prune-docs [flags]
```

### Examples

```
rev-dep node-modules prune-docs --defaults
rev-dep node-modules prune-docs --patterns "*.md,README.md,docs/**"
rev-dep node-modules prune-docs --defaults --patterns "*.txt"
```

### Options

```
  -c, --cwd string         Working directory for the command (default "$PWD")
      --defaults           Use default prune patterns: LICENSE, README.md, docs/**
  -h, --help               help for prune-docs
      --pattern strings    Alias for --patterns
  -p, --patterns strings   Glob patterns (relative to each package root) of files to remove, e.g. "*.md,README.md,docs/**"
```
