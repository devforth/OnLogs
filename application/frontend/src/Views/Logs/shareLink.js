export const getSharedLogHash = (timestamp = "") => {
  if (!timestamp) {
    return "";
  }

  return `#${encodeURI(String(timestamp).replace(/^#/, ""))}`;
};

export const buildSharedLogUrl = (currentHref = "", timestamp = "") => {
  const [baseUrl] = String(currentHref).split("#");
  return `${baseUrl}${getSharedLogHash(timestamp)}`;
};

export const readSharedLogHash = (hash = "") => {
  const raw = String(hash).replace(/^#/, "");
  try {
    return decodeURIComponent(raw);
  } catch {
    return raw;
  }
};
