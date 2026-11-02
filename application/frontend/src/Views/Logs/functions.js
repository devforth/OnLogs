import FetchApi from "../../utils/fetch";
import { stripAnsi } from "../../utils/ansi";
import { findSearchTextInLogs } from "../../utils/highlight";
export { findSearchTextInLogs };

const api = new FetchApi();

// One classification rule, mirrored by containerdb.GetLogStatusKey in Go.
const LEVELS = [
  ["ERROR", "error"],
  ["ERR", "error"],
  ["WARNING", "warn"],
  ["WARN", "warn"],
  ["DEBUG", "debug"],
  ["INFO", "info"],
  ["ONLOGS", "meta"],
];

export const classifyLogLine = (logLine = "") => {
  for (const token of stripAnsi(String(logLine)).split(/\s+/)) {
    const upper = token.toUpperCase();
    for (const [needle, level] of LEVELS) {
      if (upper.includes(needle)) {
        return level;
      }
    }
  }
  return "other";
};

// The badge is hidden for unclassified lines, so "other" is reported as "".
export const getLogLineStatus = (logLine = "") => {
  const level = classifyLogLine(logLine);
  return level === "other" ? "" : level;
};

// Mirrors containerdb.normalizeForSearch / fitsNormalizedSearch in Go.
const normalizeForSearch = (value, caseSens) => {
  const text = stripAnsi(value).replace(/\s+/g, " ").trim();
  return caseSens ? text : text.toLowerCase();
};

export const fitsSearch = (logLine = "", search = "", caseSens = false) => {
  const needle = normalizeForSearch(search, caseSens);
  return !needle || normalizeForSearch(logLine, caseSens).includes(needle);
};

// Milliseconds are what distinguish two adjacent rows, so they must survive; and
// the zone offset has to come from the timestamp itself, not from module-load
// time, or anything across a DST boundary is an hour out.
const parseLogTime = (t) => {
  const raw = t?.at?.(0);
  if (!raw) {
    return null;
  }
  const date = new Date(raw);
  return Number.isNaN(date.getTime()) ? null : date;
};

const zoneOf = (utc) => (utc ? "UTC" : undefined);

// toLocaleString builds a formatter on every call (~30 us); rows are formatted
// on every live line, so each formatter is built once.
const formatters = new Map();
const formatterFor = (locale, utc, options) => {
  const key = `${locale}|${Boolean(utc)}`;
  if (!formatters.has(key)) {
    formatters.set(key, new Intl.DateTimeFormat(locale, { ...options, timeZone: zoneOf(utc) }));
  }
  return formatters.get(key);
};

const TIME = { hour: "2-digit", minute: "2-digit", second: "2-digit", fractionalSecondDigits: 3 };
const DAY = { month: "short", day: "2-digit", year: "numeric" };

export const transformLogString = (t, utc) => {
  const date = parseLogTime(t);
  if (!date) {
    return "";
  }
  return formatterFor("sv-SE", utc, TIME).format(date).replace(",", ".");
};

export const transformLogStringForTimeBudget = (t, utc) => {
  const date = parseLogTime(t);
  if (!date) {
    return "";
  }
  return formatterFor("en-US", utc, DAY).format(date).replace(",", "");
};

export const getLogs = async function ({
  status = "",
  containerName = "",
  search = "",
  limit = 0,
  caseSens = false,
  hostName = "",
  startWith = "",
  signal,
}) {
  const newLogs = (
    (await api.getLogs({
      containerName,
      search,
      limit,
      status,
      caseSens,
      startWith,
      hostName,
      signal,
    })
  ));

  return newLogs;
};

export const getPrevLogs = async function ({
  containerName = "",
  search = "",
  limit = 0,
  status,

  caseSens = false,
  hostName = "",
  startWith = "",
}) {
  const newLogs = await api.getPrevLogs({
    containerName,
    search,
    limit,
    caseSens,
    startWith,
    hostName,
    status,
  });

  return newLogs;
};

export const scrollToBottom = () => {
  const el = document.querySelector("#endOfLogs");
  if (!el) {
    return;
  } else {
    el.scrollIntoView();
  }
};

export const scrollToNewLogsEnd = (selector, alignToTop) => {
  const el = document.querySelector(selector);

  if (!el) {
    return;
  } else {
    el.scrollIntoView(
      alignToTop
        ? { block: "end", inline: "nearest" }
        : { block: "start", inline: "nearest" }
    );
  }
};

export const scrollToSpecificLog = (selector, position) => {
  const el = document.querySelector(selector);

  if (!el) {
    return;
  } else {
    el.scrollIntoView(
      position
        ? position
        : {
            behavior: "auto",
            block: "center",
            inline: "center",
          }
    );
  }
};

export function debounce(callback, delay) {
  let timeoutId;

  return function (...args) {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => {
      callback.apply(this, args);
    }, delay);
  };
}
