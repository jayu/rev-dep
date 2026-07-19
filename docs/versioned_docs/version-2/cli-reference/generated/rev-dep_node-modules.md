---
title: "rev-dep node-modules"
description: "Analyze and manage Node.js dependencies"
---

Analyze and manage Node.js dependencies

### Synopsis

Tools for analyzing and managing Node.js module dependencies.
Helps identify unused, missing, or duplicate dependencies in your project.

### Examples

```
  rev-dep node-modules used -p src/index.ts
  rev-dep node-modules unused --exclude-modules=@types/*
  rev-dep node-modules missing --entry-points=src/main.ts
```

### Options

```
  -h, --help   help for node-modules
```
