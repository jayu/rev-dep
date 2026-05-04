---
title: "rev-dep node-modules dirs-size"
description: "Calculates cumulative files size in node_modules directories"
---

Calculates cumulative files size in node_modules directories

### Synopsis

Calculates and displays the size of node_modules folders
in the current directory and subdirectories. Sizes will be smaller than actual file size taken on disk. Tool is calculating actual file size rather than file size on disk (related to disk blocks usage)

```
rev-dep node-modules dirs-size [flags]
```

### Examples

```
rev-dep node-modules dirs-size
```

### Options

```
  -c, --cwd string   Working directory for the command (default "$PWD")
  -h, --help         help for dirs-size
```
