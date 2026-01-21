// This file should be blocked by package.json exports
export const internalFunction = () => {
  return "This should be inaccessible";
};
