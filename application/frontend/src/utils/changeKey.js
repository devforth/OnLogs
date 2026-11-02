export const changeKey = location.href.includes("localhost:5173")
  ? ""
  : "/ONLOGS_PREFIX_ENV_VARIABLE_THAT_SHOULD_BE_REPLACED_ON_BACKEND_INITIALIZATION";

// The first path segment after the prefix: a host or service may be named "stats".
export const pageOf = (pathname) =>
  (pathname.startsWith(changeKey) ? pathname.slice(changeKey.length) : pathname).split("/")[1] || "";
