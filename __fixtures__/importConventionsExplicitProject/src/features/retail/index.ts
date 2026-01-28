import { sharedFunc } from '@legacy-shared'; // Violation: wrong-alias (expected @shared)

export const retailMain = () => {
  sharedFunc();
};
