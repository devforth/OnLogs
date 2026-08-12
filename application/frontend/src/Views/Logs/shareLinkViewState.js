export const hasSearchResetRequest = (
  lastResetVersion = 0,
  nextResetVersion = 0
) => nextResetVersion !== lastResetVersion;

// Svelte invalidates an object when any of its members is assigned, so plain
// fields cannot carry state between reactive blocks without retriggering them.
// Closure variables behind function calls can.
export function createLogsViewFlags() {
  let deepLinkPending = false;
  let skipSearchReload = false;
  let skipStatusReload = false;

  return {
    beginDeepLink() {
      deepLinkPending = true;
      // The reset of searchText/chosenStatus that follows retriggers the search
      // and status blocks; that reload must not overwrite the deep-link window.
      skipSearchReload = true;
      skipStatusReload = true;
    },
    endDeepLink() {
      deepLinkPending = false;
    },
    isDeepLinkPending() {
      return deepLinkPending;
    },
    consumeSearchSkip() {
      const skip = skipSearchReload;
      skipSearchReload = false;
      return skip;
    },
    consumeStatusSkip() {
      const skip = skipStatusReload;
      skipStatusReload = false;
      return skip;
    },
  };
}
