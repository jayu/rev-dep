# Refreshing the "In production" section

`projects.ts` powers the **From 400-file packages to 197-workspace monorepos**
section on the landing page. Every number there comes from telemetry.

To refresh it, run the single query below in Application Insights → Logs, then
copy the numbers into `projects.ts`. Nothing else needs to change.

Eleven rows come back: **4 profile cards**, **4 ceiling stats**, and **3 team
candidates that are reported for review but are not publish-ready** - see the
last section for why.

---

## The query

```kusto
let lookback = 90d;
//
// One row per tracked project, with everything the landing page might show.
// Runs once and is reused by every branch of the union below.
//
let perProject =
    customEvents
    | where timestamp > ago(lookback)
    | where name == "config-run"
    | extend
        projectId = tostring(customDimensions.projectId),
        machineId = tostring(customDimensions.machineId),
        isCI      = tostring(customDimensions.isCI),
        country   = client_CountryOrRegion          // derived from IP by the collector
    | where isnotempty(projectId)
    | summarize
        files      = max(tolong(customMeasurements.fileCount)),
        workspaces = max(tolong(customMeasurements.workspaceCount)),
        boundaries = max(tolong(customMeasurements.moduleBoundaries)),
        // Distinct NON-CI machines: CI containers get a fresh machineId every
        // run, so including them would report thousands of "developers".
        devs       = dcountif(machineId, isCI != "true"),
        runs       = count(),
        localRuns  = countif(isCI != "true"),
        ciRuns     = countif(isCI == "true"),
        dCircular  = max(tolong(customMeasurements.circularImports)),
        dOrphan    = max(tolong(customMeasurements.orphanFiles)),
        dUnusedEx  = max(tolong(customMeasurements.unusedExports)),
        dUnusedNM  = max(tolong(customMeasurements.unusedNodeModules)),
        dMissingNM = max(tolong(customMeasurements.missingNodeModules)),
        dUnresolved= max(tolong(customMeasurements.unresolvedImports)),
        dDevDeps   = max(tolong(customMeasurements.devDepsUsageOnProd)),
        dRestrImp  = max(tolong(customMeasurements.restrictedImports)),
        dRestrErs  = max(tolong(customMeasurements.restrictedImporters)),
        dRestrDir  = max(tolong(customMeasurements.restrictedDirectImporters)),
        dConvent   = max(tolong(customMeasurements.importConventions)),
        // max() on a string, not take_any(): always available, and it gives a
        // deterministic pick when a project is seen from several countries.
        country    = max(country)
      by projectId
    | extend checksEnabled =
          iff(dCircular   > 0, 1, 0) + iff(dOrphan   > 0, 1, 0)
        + iff(dUnusedEx   > 0, 1, 0) + iff(dUnusedNM > 0, 1, 0)
        + iff(dMissingNM  > 0, 1, 0) + iff(dUnresolved > 0, 1, 0)
        + iff(dDevDeps    > 0, 1, 0) + iff(dRestrImp > 0, 1, 0)
        + iff(dRestrErs   > 0, 1, 0) + iff(dRestrDir > 0, 1, 0)
        + iff(boundaries  > 0, 1, 0) + iff(dConvent  > 0, 1, 0);
//
// 4 profile cards + 3 team candidates (review only) + 4 ceilings.
//
union
    (perProject
     | top 1 by workspaces desc
     | project sortOrder = 10, section = "profile",
               label     = "Large monorepo",
               value     = strcat(tostring(workspaces), " workspaces"),
               detail    = strcat(tostring(files), " source files · ",
                                  tostring(checksEnabled), " checks configured · ",
                                  iff(ciRuns > localRuns, "runs on every CI build",
                                                          "mostly run locally")),
               region    = country),
    (perProject
     | top 1 by files desc
     | project sortOrder = 15, section = "profile",
               label     = "Large single codebase",
               value     = strcat(tostring(files), " source files"),
               detail    = strcat(tostring(workspaces), " workspace(s) · ",
                                  tostring(checksEnabled), " checks configured"),
               region    = country),
    // THREE team candidates, not one. The largest is regularly an artefact of
    // ephemeral environments, so the runner-up is often the real answer.
    // Each row self-flags via runs-per-machine - see the section below.
    (perProject
     | where devs > 1
     | sort by devs desc
     | take 3
     | extend rank = row_number()
     | extend runsPerMachine = round(todouble(localRuns) / todouble(devs), 1)
     | project sortOrder = 20 + rank, section = "review-before-use",
               label     = strcat("Team candidate #", tostring(rank)),
               value     = strcat(tostring(devs), " machines · ",
                                  tostring(runsPerMachine), " runs each"),
               detail    = strcat(tostring(files), " source files · ",
                                  tostring(localRuns), " local runs · ",
                                  case(runsPerMachine <  5.0, "SUSPECT - ephemeral environments",
                                       runsPerMachine < 10.0, "BORDERLINE - weigh against file count",
                                                              "PLAUSIBLE - looks like real people")),
               region    = country),
    (perProject
     | top 1 by boundaries desc
     | project sortOrder = 30, section = "profile",
               label     = "Architecture-first",
               value     = strcat(tostring(boundaries), " module boundary rules"),
               detail    = strcat(tostring(files), " source files · ",
                                  tostring(workspaces), " workspace(s)"),
               region    = country),
    (perProject
     | summarize medianFiles = percentile(files, 50), medianWs = percentile(workspaces, 50)
     | project sortOrder = 40, section = "profile",
               label     = "Typical single package",
               value     = strcat(tostring(toint(medianFiles)), " source files"),
               detail    = strcat("median project · ",
                                  tostring(toint(medianWs)), " workspace(s)"),
               region    = ""),
    (perProject
     | summarize v = max(files)
     | project sortOrder = 50, section = "ceiling",
               label = "files in the largest codebase",
               value = tostring(v), detail = "", region = ""),
    (perProject
     | summarize v = max(workspaces)
     | project sortOrder = 60, section = "ceiling",
               label = "workspaces in the largest monorepo",
               value = tostring(v), detail = "", region = ""),
    // Boundary rules, not developers: a devs ceiling inherits the same
    // ephemeral-environment problem as the team rows above.
    (perProject
     | summarize v = max(boundaries)
     | project sortOrder = 70, section = "ceiling",
               label = "boundary rules on one project",
               value = tostring(v), detail = "", region = ""),
    // Unlike files and workspaces, this one scales with `lookback` - it is a
    // count over the window, not a property of the project. Change lookback and
    // this number changes, so the label must always name the period.
    (perProject
     | summarize v = max(runs)
     | project sortOrder = 80, section = "ceiling",
               label = "runs on one project in the lookback window",
               value = tostring(v), detail = "", region = "")
| order by sortOrder asc
| project section, label, value, detail, region
```

---

## Mapping the result to `projects.ts`

The four `profile` rows fill `projectProfiles`, the four `ceiling` rows fill
`scaleCeilings`. The `review-before-use` rows go nowhere until you have judged
them.

| Query column | Goes to |
| --- | --- |
| `label` | `kind` |
| `value` — the number part | `headline` |
| `value` — the words part | `headlineLabel` |
| `detail` — split on `·` | `facts[]` |
| `region` | `region` (optional; omit if blank) |

Example — a `profile` row of
`Large monorepo | 174 workspaces | 8339 source files · 28 checks configured · runs on every CI build | Poland`
becomes:

```ts
{
  kind: 'Large monorepo',
  headline: '174',
  headlineLabel: 'workspaces in one repo',
  facts: ['8,339 source files', '28 checks configured', 'Runs on every CI build'],
  region: 'Poland',
}
```

Add thousands separators by hand — the query returns raw integers.

The section title hard-codes both ends of the range - "From 400-file packages
to 197-workspace monorepos" - in `RealProjects.tsx`. Update it if the median
file count or the workspace ceiling moves.

---

## Two things to keep honest

**Never publish the project count**, or any percentage derived from it. The
sample is small, and "X% of projects" invites "out of how many?" — the one
question with a weak answer. These per-project facts are strong precisely
because each is a real, specific codebase.

**`devs` excludes CI.** CI containers get a fresh `machineId` on every run, so
counting them would turn a handful of pipelines into thousands of imaginary
developers.

---

## Why the team number needs a second look

The row labelled **"Largest team (verify first)"** is intentionally not
publish-ready. `machineId` is a hash of hardware and OS facts, so **any
short-lived environment produces a new one**:

- Codespaces, Gitpod, devcontainers
- AI agent sandboxes
- CI that `isCI` detection missed — self-hosted runners often set none of the
  standard environment variables

The tell is **runs per machine**. Real developers run a check many times on the
same laptop over 90 days. Ephemeral environments run it once or twice and
disappear.

A previous pull returned *282 machines, 920 local runs* on a **1,623-file**
project — 3.3 runs per machine. Three hundred engineers do not share a
1,600-file codebase, and they would not each run the tool three times and stop.
That was containers, and the number never went on the site.

**Rule of thumb:** below ~5 runs per machine, treat the count as environments
rather than people, and leave the team card out. Above ~10, it is probably real
— sanity-check it against the project's file count before publishing.

### Outcome of the last refresh

All three candidates were rejected, so **there is no team card on the page**:

| Candidate | Reading | Verdict |
| --- | --- | --- |
| #1 | 282 machines · 3.3 runs each · 1,623 files | Ephemeral environments |
| #2 | 47 machines · 8.3 runs each · 8,339 files | **Plausible, unconfirmed** |
| #3 | 16 machines · 3.4 runs each · **6 files** | Junk |

**#2 is the one worth revisiting.** 47 developers on an 8,339-file monorepo is
about 177 files each, and that project runs mostly in CI — which fits a team
whose local runs are occasional. It is off the page only because nobody has
confirmed it, not because the number looks wrong.

Adding it means a fifth profile card, and the grid is four wide — so either drop
a card or move that row to five columns.

**#3 also exposes a gap in the ratio test:** a six-file project is never a
"team", whatever its runs-per-machine works out to. Consider adding
`| where files > 200` to the team branch.

---

## Dedicated query: find a real team

The main query ranks by raw machine count, which is why its top result keeps
being containers. This one ranks by **returning machines** instead, and it is
the query to run when you want a team-size card.

### The idea

Runs-per-machine is a weak proxy. The strong signal is **whether a machine comes
back on a different day**:

- A developer's laptop appears on many separate days over 90 days.
- A container, Codespace or agent sandbox appears **once**, then its fingerprint
  is gone forever.

So instead of counting machines, count machines seen on **2 or more distinct
days**. Ephemeral environments score ~0 on that measure no matter how many of
them there are. A 282-container project collapses to almost nothing; a real
47-person team barely moves.

```kusto
let lookback   = 90d;
let minFiles   = 200;   // a 6-file project is never a "team"
let minDays    = 2;     // seen on 2+ separate days = a machine that came back
//
// Local runs only. CI is excluded up front - it is the main source of
// throwaway machine fingerprints.
//
let events =
    customEvents
    | where timestamp > ago(lookback)
    | where name == "config-run"
    | where tostring(customDimensions.isCI) != "true"
    | extend
        projectId  = tostring(customDimensions.projectId),
        machineId  = tostring(customDimensions.machineId),
        country    = client_CountryOrRegion,
        files      = tolong(customMeasurements.fileCount),
        workspaces = tolong(customMeasurements.workspaceCount)
    | where isnotempty(projectId) and isnotempty(machineId);
//
// How many separate days did each machine show up on?
//
let machineDays =
    events
    | summarize days = dcount(bin(timestamp, 1d)), runs = count()
      by projectId, machineId;
//
// Per project: total machines vs machines that actually came back.
//
let teams =
    machineDays
    | summarize
        machines   = dcount(machineId),
        developers = dcountif(machineId, days >= minDays),   // the real number
        regulars   = dcountif(machineId, days >= 5),         // weekly-ish users
        runs       = sum(runs)
      by projectId;
//
// Project size and location, for the sanity check and the card.
//
let meta =
    events
    | summarize
        files      = max(files),
        workspaces = max(workspaces),
        country    = max(country),
        activeDays = dcount(bin(timestamp, 1d))
      by projectId;
teams
| join kind=inner meta on projectId
| extend
    retentionPct   = round(100.0 * todouble(developers) / todouble(machines), 0),
    runsPerMachine = round(todouble(runs) / todouble(machines), 1)
| extend verdict = case(
    files      < minFiles, "SKIP - too small to have a team",
    developers < 3,        "SKIP - almost no machine came back",
    retentionPct < 30.0,   "SUSPECT - mostly one-shot environments",
    regulars   < 3,        "BORDERLINE - few regular users",
                           "LIKELY REAL")
| sort by developers desc
| project verdict, developers, regulars, totalMachines = machines,
          retentionPct, runsPerMachine, activeDays, files, workspaces, country
| take 10
```

### Reading the result

Take the **first row whose verdict is `LIKELY REAL`** — that is the card.

| Column | Meaning |
| --- | --- |
| `developers` | Machines seen on 2+ separate days. **Publish this, not `totalMachines`.** |
| `regulars` | Machines seen on 5+ days — the people who actually live in it |
| `totalMachines` | Raw count, including every throwaway environment |
| `retentionPct` | `developers / totalMachines`. Near 100 = laptops. Near 0 = containers. |

`retentionPct` is the tell. The 282-machine project should land in the single
digits; a genuine team should be well over half.

Publish it as, for example:

```ts
{
  kind: 'Whole engineering team',
  region: 'United States',
  headline: '47',
  headlineLabel: 'developers on one codebase',
  facts: ['8,339 source files', '174 workspaces', 'Run locally, every week'],
}
```

Adding a fifth profile card means the four-wide grid in `RealProjects.tsx`
needs to become five, or one existing card has to go.

### The runs ceiling is window-dependent

`runs on one project in 90 days` is the only ceiling that is a count over the
lookback window rather than a property of the codebase. Widen `lookback` and it
grows without anything changing. **Keep the period in the label.**
