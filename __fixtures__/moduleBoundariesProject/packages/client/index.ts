import { api } from "@packages/api"; // Valid
import { util } from "@packages/utils"; // Valid
import { secret } from "../utils/secret"; // Invalid (direct file import across boundary, if restricted) but valid if utils is allowed. 
