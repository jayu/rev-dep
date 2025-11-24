// All export from examples

export * from "module-name";
export * as name1 from "module-name";
export { name1,  nameN } from "module-name";
export { import1 as name1, import2 as name2,  nameN } from "module-name";
export { default } from "module-name";
export { default as name1 } from "module-name";
export { type MyType } from "./types";
export type { MyType } from "./types";
export { default, type MyType2 } from "./types";

// All import examples

import defaultExport from "module-name";
import * as name from "module-name";
import { export1 } from "module-name";
import { export1 as alias1 } from "module-name";
import { default as alias } from "module-name";
import { export1, export2 } from "module-name";
import { export1, export2 as alias2 } from "module-name";
import { "string name" as alias } from "module-name";
import defaultExport, { export1 } from "module-name";
import defaultExport, * as name from "module-name";
import "module-name";
import { type MyType2 } from "./types";
import type {  MyType2 } from "./types";
import fnA, { type MyType3 } from "./types";
import fnA, { type MyType3, MyVal } from "./types";