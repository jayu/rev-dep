---
title: rev-dep list-cwd-files
---

List all files in the current working directory

### Synopsis

Recursively lists all files in the specified directory,
with options to filter results.

```
rev-dep list-cwd-files [flags]
```

### Examples

```
rev-dep list-cwd-files --include='*.ts' --exclude='*.test.ts'
```

### Options

```
      --count             Only display the count of matching files
      --cwd string        Directory to list files from (default "$PWD")
      --exclude strings   Exclude files matching these glob patterns
  -h, --help              help for list-cwd-files
      --include strings   Only include files matching these glob patterns
```
