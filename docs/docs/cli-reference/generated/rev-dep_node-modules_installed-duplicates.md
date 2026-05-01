---
title: rev-dep node-modules installed-duplicates
---

Find and optimize duplicate package installations

### Synopsis

Identifies packages that are installed multiple times in node_modules.
Can optimize storage by creating symlinks between duplicate packages.

```
rev-dep node-modules installed-duplicates [flags]
```

### Examples

```
rev-dep node-modules installed-duplicates --optimize --size-stats
```

### Options

```
  -c, --cwd string   Working directory for the command (default "$PWD")
  -h, --help         help for installed-duplicates
      --isolate      Create symlinks only within the same top-level node_module directories. By default optimize creates symlinks between top-level node_module directories (eg. when workspaces are used). Needs --optimize flag to take effect
      --optimize     Automatically create symlinks to deduplicate packages
      --size-stats   Print node modules dirs size before and after optimization. Might take longer than optimization itself
      --verbose      Show detailed information about each optimization
```
