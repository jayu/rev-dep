---
title: "rev-dep node-modules installed"
description: "List all installed npm packages in the project"
---

List all installed npm packages in the project

### Synopsis

Recursively scans node_modules directories to list all installed packages.
Helpful for auditing dependencies across monorepos.

```
rev-dep node-modules installed [flags]
```

### Examples

```
rev-dep node-modules installed --include-modules=@myorg/*
```

### Options

```
  -c, --cwd string                Working directory for the command (default "$PWD")
  -e, --exclude-modules strings   list of modules to exclude from the output
  -h, --help                      help for installed
  -i, --include-modules strings   list of modules to include in the output
```
