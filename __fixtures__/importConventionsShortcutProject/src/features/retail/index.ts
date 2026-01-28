import { retailUtil } from '@retail/utils'; // Violation: should-be-relative (intra-domain aliased)
import { sharedFunc } from '../shared'; // Violation: should-be-aliased (inter-domain relative)
import { sharedFunc2 } from '@my-shared'; // Valid: inter-domain aliased (even if it's not the inferred one)

export const retailMain = () => {
  retailUtil();
  sharedFunc();
  sharedFunc2();
};
