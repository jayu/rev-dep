// Test different exports specificity
import { mainFunction } from "exported-package";
import { featureA } from "exported-package/features/feature-a";
import { featureB } from "exported-package/features/feature-b";

// Test conditional exports (development vs production)
import { helper } from "exported-package/utils/helper";

// Test basic wildcard scenario
import { something } from "exported-package/wildcard/something.js";

// Test root wildcard scenario
import { config } from "exported-package/root/config/setup.config.js";

// Test directory swap with file name
import { featureFromDist } from "exported-package/features/feature-from-dist.js";

// Test proper directory name with file name swap
import { testFile } from "exported-package/some/xyz/file.js";

// Test multiple wildcards (should be excluded/unresolvable)
import { invalidFile } from "exported-package/invalid/a/to/b/file.js";

// Test deeply nested exports with different condition sets
import { deepNode } from "exported-package/deep";
import { deepDevDefault } from "exported-package/deep";
import { deepFallback } from "exported-package/deep";

// Test blocked path in deep nested structure
import { deepBlocked } from "exported-package/deep/blocked";

// This should fail due to blocked path
import { internal } from "exported-package/features/private-internal-utils";


// This should fail due to blocked wildcard
import { blocked } from "exported-package/blocked/something";
import { blocked2 } from "exported-package/blocked/other";


export { mainFunction, featureA, featureB, helper, something, config, featureFromDist, testFile, invalidFile, deepNode, deepDevDefault, deepFallback, deepBlocked };
