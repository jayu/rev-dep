import { mainFunction } from "exported-package";
import { featureA } from "exported-package/features/feature-a";
import { featureB } from "exported-package/features/feature-b";

// This import uses conditional mapping
import { helper } from "exported-package/utils/helper";

// This import uses wildcard mapping
import { something } from "exported-package/wildcard/something.js";

// This import uses root wildcard mapping
import { setup } from "exported-package/root/config/setup.config.js";

// This import uses directory swap mapping
import { xyz } from "exported-package/features/feature-from-dist.js";

// This import uses directory swap mapping
import { swapped } from "exported-package/some/xyz/file.js";

// This import uses invalid multiple wildcard pattern (currently bugged, but should be Mark as NotResolved)
import { invalid } from "exported-package/invalid/a/to/b/file.js";

// These imports test deeply nested conditional exports
import { deepNode } from "exported-package/deep";
import { deepDevDefault } from "exported-package/deep";
import { deepProdBrowser } from "exported-package/deep";

// This tests a blocked path
import { blocked } from "exported-package/deep/blocked";

// More blocked paths
import { privateInternal } from "exported-package/features/private-internal-utils";
import { blockedSomething } from "exported-package/blocked/something";
import { blockedOther } from "exported-package/blocked/other";

export const consumerFunction = () => {
    return "This is the consumer package " + mainFunction() + featureA() + featureB() + helper() + something() + setup() + xyz() + swapped() + invalid() + deepNode() + deepDevDefault() + deepProdBrowser() + blocked() + privateInternal() + blockedSomething() + blockedOther();
};
