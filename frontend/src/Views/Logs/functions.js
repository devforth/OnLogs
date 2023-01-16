import FetchApi from "../../utils/fetch";

const api = new FetchApi();

export const timezoneOffsetSec = new Date().getTimezoneOffset() * 60;

export const getLogLineStatus = (logLine = "") => {
  const statuses_errors = ["ERROR", "ERR", "Error", "Err"];
  const statuses_warnings = ["WARN", "WARNING"];
  const statuses_other = ["DEBUG", "INFO", "ONLOGS"];
  const logLineItems = logLine.split(" ");
  var i, j;

  for (i = 0; i < logLineItems.length; i++) {
    for (j = 0; j < logLineItems.length; j++) {
      if (logLineItems[i].includes(statuses_errors[j])) {
        return "error";
      }
    }
    for (j = 0; j < logLineItems.length; j++) {
      if (logLineItems[i].includes(statuses_warnings[j])) {
        return "warn";
      }
    }
    for (j = 0; j < logLineItems.length; j++) {
      if (logLineItems[i].includes(statuses_other[j])) {
        return statuses_other[j].toLowerCase();
      }
    }
  }
  return "";
};

export const transformLogString = (str, options) => {
  return options
    ? new Date(
        new Date().setTime(
          new Date(str.at(0).slice(0, 19).replace("T", " ")).getTime()
        )
      ).toLocaleString("sv-EN", {
        year: "numeric",
        month: "short",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : new Date(
        new Date().setTime(
          new Date(str.at(0).slice(0, 19).replace("T", " ")).getTime() -
            timezoneOffsetSec * 1000
        )
      ).toLocaleString("sv-EN", {
        year: "numeric",
        month: "short",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      });
};

export const getLogs = async function ({
  containerName = "",
  search = "",
  limit = 0,
  offset,
  caseSens = false,
  hostName = "",
  startWith = "",
}) {
  const newLogs = (
    await api.getLogs({
      containerName,
      search,
      limit,
      offset,
      caseSens,
      startWith,
      hostName,
    })
  ).reverse();

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
    el.scrollIntoView(!alignToTop);
  }
};
