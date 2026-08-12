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
