import { merge } from "lodash";
import { util } from "shared-only-util";
import { thing } from "phantom-dep";

export const sharedValue = merge({}, { util, thing });
