# Findings baseline

> Acknowledge existing findings, keep CI clean, and detect new or obsolete issues without depending on source positions or detector-array order.

A project can have existing issues that you want to fix gradually. A findings baseline records those issues so CI can pass while they remain acknowledged. It fails when a new issue appears or an acknowledged issue disappears.

The baseline compares **which problems exist**, rather than their total count. Replacing a missing dependency with a different missing dependency fails even when there is still only one issue.

## Set up a baseline

Add `baselineSnapshotPath` at the top level of your [config](./config-file-structure.mdx). Its path is relative to the config directory:

```json
{
  "$schema": "https://github.com/jayu/rev-dep/blob/master/config-schema/2.2.schema.json?raw=true",
  "configVersion": "2.2",
  "baselineSnapshotPath": ".rev-dep.baseline.json",
  "workspaces": [
    {
      "path": ".",
      "missingNodeModulesDetection": true,
      "circularImportsDetection": true
    }
  ]
}
```

Record the current findings:

```bash
rev-dep config run --update-snapshot
```

Review and commit the config and generated baseline. CI continues to use the ordinary command:

```bash
rev-dep config run
```

Unchanged acknowledged findings pass. A configured baseline that is missing or invalid fails; ordinary runs never create it automatically. Without `baselineSnapshotPath`, checks retain their usual behavior.

## New and obsolete findings

Suppose `src/main.ts` imports a missing package, `old-package`, when you record the baseline:

- Adding blank lines above that import passes: its position is not part of its identity.
- Importing a new missing package produces a new finding and fails.
- Declaring `old-package` in `package.json`, removing its import, or excluding it from detection makes the recorded finding obsolete and fails.
- Replacing `old-package` with `new-package` produces one obsolete finding and one new finding and fails.

After an intentional change, record the new state and commit the updated file:

```bash
rev-dep config run --update-snapshot
```

Obsolete findings fail too so the committed baseline continues to describe the current issue set. Updating does not fix code; it acknowledges the results of the current checks.

## What identifies a finding

Findings are grouped by workspace path and detector kind. All file paths in the baseline are relative to the config directory. Detector instances defined as an array contribute to one deduplicated set, so reordering rules does not change the baseline.

Examples of identities include:

- Missing dependency: importing file and package name.
- Unused dependency: owning `package.json` and package name.
- Unused export: file, exported name, and whether it is a type export.
- Module boundary: importing file and resolved target file.
- Circular imports: the sorted members of a cyclic strongly connected component. A new internal cycle leaves the identity unchanged if component membership stays the same.

Line numbers, columns, import indexes, rule names, and selected import chains are not stored. Multiple imports of the same missing package in one file are one issue. Nonliteral unresolved imports use the raw reported expression; changing formatting inside that expression can change its identity.

For restricted imports, identity is the forbidden file or module request reachable under the configured policies. Another route or importer reaching an already acknowledged target does not add a finding. Restricted importers instead records each prohibited entry-point/target pair, and restricted direct importers records direct importing-file/target pairs.

The file does not copy detector settings or contain rule-array indexes. An option change only requires an update if it changes the finding set. Adding or removing a detector with no findings passes. Removing a detector or workspace with acknowledged findings reports those findings as obsolete.

The baseline is versioned separately from the config. Its format is described by the [findings baseline JSON schema](https://github.com/jayu/rev-dep/blob/master/output-schema/findings-baseline-1.0.schema.json). Use `--update-snapshot` to regenerate an incompatible baseline.

## How this differs from duplicated-code snapshots

[Duplicated-code detection](./checks/duplicated-code.mdx) already has a specialized snapshot mechanism. Duplication needs canonical code hashes, per-file copy counts, normalization/blinding, and threshold handling. Those requirements differ from identities such as “this file imports this missing package.”

| Findings baseline | Duplicated-code snapshot |
| --- | --- |
| One file at top-level `baselineSnapshotPath` | A separate `snapshotPath` for each duplicated-code detection |
| Path relative to the config directory | Path relative to the detection's workspace |
| Stores abstract issue identities across ordinary checks | Stores canonical hashes and per-file occurrence counts, plus detection settings/context |
| Does not freeze detector settings or track source positions | Uses its existing settings reconciliation and specialized change classification |
| Excludes duplicated-code findings and their pass/fail status | Decides duplicated-code pass/fail independently |

Both use **the same update command**:

```bash
rev-dep config run --update-snapshot
```

It updates the general findings baseline and every configured duplicated-code snapshot. Review all affected snapshot files before committing. They remain separate files with separate comparison logic.

## Fixing and selected workspaces

With a findings baseline configured, `--fix` always rechecks the final code before comparing findings, even without `--recheck`. Fixing an acknowledged issue therefore makes its recorded finding obsolete. To fix and record the final state together:

```bash
rev-dep config run --fix --update-snapshot
```

You can validate selected workspaces:

```bash
rev-dep config run --workspaces packages/app
```

This compares only those workspace scopes, using the full config's resolution context. Paths such as `.` and `./` identify the same scope: selecting either runs every configured entry in that scope. It does not treat unselected findings as obsolete. Updating a general baseline requires a full run: `--update-snapshot --workspaces` is rejected when `baselineSnapshotPath` is configured.

## Output and failures

Console and issues-list output show the baseline summary and remaining violations. New findings use current locations; obsolete findings show their stored identities. A matched baseline reports how many findings are acknowledged rather than claiming no issues exist.

Use `--list-all-issues` or `--format issues-list` to display all obsolete findings.

[JSON output](./output-formats.mdx) adds a `baseline` object containing the snapshot path, status, counts, and new/obsolete identities. The counts describe the comparison:

- `baseline.counts.new`: unique findings detected now that are absent from the recorded baseline.
- `baseline.counts.obsolete`: recorded findings that are absent from the current findings, including issues fixed in code or excluded by configuration changes.
- `baseline.counts.acknowledged`: findings present in both the baseline and the current run.

These are observations; an ordinary run leaves the baseline file unchanged. Ordinary check issue lists and `baseline.new` contain the new violations. Obsolete findings are in `baseline.obsolete`; their check may no longer be configured. For filtered validation, `baseline.workspaces` identifies the selected scope.
