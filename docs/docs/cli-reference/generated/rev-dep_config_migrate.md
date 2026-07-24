---
title: "rev-dep config migrate"
description: "Upgrade a v2 config to the v3 (2.0) schema"
---

Upgrade a v2 config to the v3 (2.0) schema

### Synopsis

Upgrade a (.)rev-dep.config.json(c) from the v2 schema to v3 (config version 2.0).

It applies the safe, unambiguous changes in place (renaming the top-level 'rules' array to
'workspaces', bumping 'configVersion' to 2.0, and removing the discontinued 'algorithm'
option from circular-imports detectors), preserving all comments and formatting. Review the
change with git before committing.

It then lists what it could NOT change for you: glob patterns whose match set may have
shifted under v3's stricter, gitignore-aligned rules, and behavior changes that no config
edit can address. Review those manually — see the v3 breaking-changes guide.

```
rev-dep config migrate [flags]
```

### Options

```
  -c, --cwd string   Working directory (default "$PWD")
  -h, --help         help for migrate
```
