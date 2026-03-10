export const getSharedLogHash = (timestamp = "") => {
  if (!timestamp) {
    return "";
  }

  return `#${String(timestamp).replace(/^#/, "")}`;
};

export const buildSharedLogUrl = (currentHref = "", timestamp = "") => {
  const [baseUrl] = String(currentHref).split("#");
  return `${baseUrl}${getSharedLogHash(timestamp)}`;
};

export const applySharedLogUrl = (
  currentHref = "",
  timestamp = "",
  historyLike = window.history
) => {
  const nextUrl = buildSharedLogUrl(currentHref, timestamp);

  historyLike.replaceState(null, "", nextUrl);

  return {
    nextUrl,
    hash: getSharedLogHash(timestamp),
  };
};
