export const hasSearchResetRequest = (
  lastResetVersion = 0,
  nextResetVersion = 0
) => nextResetVersion !== lastResetVersion;

export const shouldAutoScrollLogs = (
  autoscroll = false,
  isSharedLinkFocusMode = false
) => autoscroll && !isSharedLinkFocusMode;

export const shouldFlushBufferedLogs = (
  logsFromWSLength = 0,
  isSharedLinkFocusMode = false,
  releaseRequested = false
) => logsFromWSLength > 0 && (!isSharedLinkFocusMode || releaseRequested);
