## Performance comparison ⚡

Rev-dep can perform multiple checks on 500k+ LoC monorepo with several sub-packages in around 150ms.

It outperforms Madge, dpdm, dependency-cruiser, skott, knip, depcheck and other similar tools.

Here is a performance comparison of specific tasks between rev-dep and alternatives:

| Task | Execution Time [ms] | Alternative | Alternative Time [ms] | Slower Than Rev-dep |
|------|--------------------:|-------------|----------------------:|--------------------:|
| Find circular dependencies | 151.4 ± 1.9 | knip | 3 039.9 ± 36.3 | 20x |
| Find unused exports | 186.4 ± 2.9 | knip | 3 176.1 ± 24.2 | 17x |
| Find unused files | 168.2 ± 1.8 | knip | 3 005.9 ± 34.0 | 18x |
| Find unused node modules | 170.0 ± 3.0 | knip | 3 068.9 ± 17.1 | 18x |
| Find missing node modules | 159.5 ± 3.0 | knip | 3 076.1 ± 29.8 | 19x |
| List all files imported by an entry point | 81.2 ± 1.9 | madge | 6 591.4 ± 129.2 | 81x |
| Discover entry points | 148.9 ± 3.8 | madge | 13 632.0 ± 137.1 | 92x |
| Resolve dependency path between files | 220.8 ± 3.6 | please suggest |
| Count lines of code | 251.4 ± 31.2 | please suggest |
| Analyze node_modules directory sizes | 560.7 ± 54.5 | please suggest |

> Platform: WSL Linux Debian Intel(R) Core(TM) i9-14900KF CPU
>
> Measurements: `hyperfine -w 4 -r 8` (4 warm-up + 8 measured runs)
> 
> Project: 580k lines of code, 6024 source code files next.js app

### Circular check performance comparison

Table below presents performance comparison between different tools performing circular imports detection.

`rev-dep` circular check is **~20 times** faster than the fastest alternative.

| Tool | Version | Command to Run Circular Check | Time [ms] |
|------|---------|-------------------------------|----------:|
| 🥇 [rev-dep](https://github.com/jayu/rev-dep) | 3.0.0 | `rev-dep circular` | **153.6** ± 2.3 |
| 🥈 [knip](https://github.com/webpro-nl/knip) * | 6.29.0 | `knip --cycles` | 3 039.9 ± 36.3 |
| 🥉 [circular-dependency-scanner](https://github.com/emosheeep/circular-dependency-scanner) | 3.0.1 | `ds . -i <ignore globs>` | 3 354.5 ± 32.6 |
| [dpdm-fast](https://github.com/SunSince90/dpdm-fast) | 1.0.14 | `dpdm --no-tree --no-warning --no-progress --tsconfig tsconfig.json` + list of directories with source code | 6 069.7 ± 315.7 |
| [dpdm](https://github.com/acrazing/dpdm) | 4.2.0 | `dpdm --no-tree --no-warning --no-progress --tsconfig tsconfig.json --exclude 'node_modules\|generated/prisma'` + list of directories with source code | 6 667.4 ± 44.4 |
| [dependency-cruiser](https://github.com/sverweij/dependency-cruiser) | 18.1.0 | `depcruise --config <config> --output-type err` + list of directories with source code | 8 257.7 ± 118.9 |
| [madge](https://github.com/pahen/madge) | 8.0.0 | `madge --circular --extensions ts,tsx,js --ts-config tsconfig.json` + list of directories with source code | 13 568.8 ± 63.7 |
| [skott](https://github.com/antoine-coulon/skott) | 0.35.11 | node script using skott `findCircularDependencies` function | 61 612.7 ± 132.6 |


\* knip always ignores type-only import edges and offers no flag to include them. Every cycle in
this codebase contains at least one, so knip reports 0 cycles — its 3 040 ms is a real full
analysis, just of a smaller graph. `rev-dep circular -t`, which applies the same rule, agrees
exactly (0 cycles) in 143.3 ms ± 10.3.

> Platform: WSL Linux Debian Intel(R) Core(TM) i9-14900KF CPU
>
> Measurements: `hyperfine -w 4 -r 8` (4 warm-up + 8 measured runs)
> 
> Project: 580k lines of code, 6024 source code files next.js app
