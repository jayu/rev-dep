---
title: "rev-dep duplicated-code"
description: "Find code duplicated across (and within) the files of the project"
---

Find code duplicated across (and within) the files of the project

### Synopsis

Scans every source file for copy-pasteable chunks - brace blocks and JSX elements,
at every level of nesting - and reports the ones that appear more than once.

Comparison ignores formatting and comments entirely, so re-indented copies still match.

Four filters decide what counts as worth reporting. --min-tokens and --min-lines
measure size; --min-depth and --min-statements measure complexity, which is what
separates a duplicated three-key config object from duplicated logic - no size
floor can, because the object's keys and string values may be long. --min-duplicates
sets how many copies it takes to qualify.

By default the code must match as written. Each --blind-* flag drops one category
of token out of the comparison, and they combine freely:

  --blind-identifiers   names are wildcards, so a copy whose variables, functions
                        or components were renamed is still reported
  --blind-strings       string and template contents are wildcards
  --blind-numbers       numeric literals are wildcards


```
rev-dep duplicated-code [flags]
```

### Examples

```
rev-dep duplicated-code --cwd ./src --blind-identifiers
```

### Options

```
      --blind-identifiers               Ignore the spelling of names, so a copy whose variables, functions or components were renamed still counts as duplication.
      --blind-numbers                   Ignore the value of numeric literals, so a copy with different constants still counts
      --blind-strings                   Ignore the text of string and template literals, so a copy with different messages or keys still counts
  -c, --cwd string                      Working directory for the command (default "$PWD")
  -f, --format string                   Output format: "human" or "json". JSON reports every finding with its canonical hash and the byte and line range of each occurrence, for comparing against another run or another tool (default "human")
  -h, --help                            help for duplicated-code
      --ignore-files strings            Glob patterns of files to leave out of the analysis.
      --json-snippets                   Include the source of each finding in JSON output. Off by default because snippets dominate the file size and a comparison keyed on ranges does not need them
      --min-depth int                   Smallest duplication to report, in nesting levels counting the block itself. 1 admits everything; 2 requires at least one nested level, which is what filters out flat objects and single JSX elements however long their keys or strings are
      --min-duplicates int              How many copies a chunk needs before it is reported. Raise to 3 to ignore code that has only been copied once (default 2)
      --min-lines int                   Smallest duplication to report, in lines of the first occurrence (default 3)
      --min-statements int              Smallest duplication to report, in statements directly inside the block. Applies only to statement blocks (function and control-flow bodies); object literals and JSX elements are expressions and are not filtered by it - use --min-depth for those
      --min-tokens int                  Smallest duplication to report, in tokens. Tokens rather than characters because the count does not change when a --blind-* flag is applied, so one number means the same amount of code whatever is being ignored (default 50)
      --process-ignored-files strings   Glob patterns to analyse even when gitignore excludes them.
      --skip-objects                    Do not report duplications that are only object literals. 
      --snapshot string                 Path to a JSON snapshot of acknowledged duplications. With it, the command reports what changed since the snapshot instead of everything that exists, and exits non-zero on any difference
      --update-snapshot                 Rewrite the --snapshot file from this run, acknowledging everything it found. Always explicit: nothing updates a snapshot on its own
```
