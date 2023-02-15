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

export const transformLogString = (t, options) => {
  return options
    ? new Date(
        new Date().setTime(
          new Date(t?.at(0)?.slice(0, 22)?.replace("T", " "))?.getTime()
        )
      ).toLocaleString("sv-EN", {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
        fractionalSecondDigits: 3,
      })
    : new Date(
        new Date().setTime(
          new Date(t?.at(0)?.slice(0, 19)?.replace("T", " "))?.getTime() -
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
  caseSens = false,
  hostName = "",
  startWith = "",
  signal,
}) {
  const newLogs = (
    await api.getLogs({
      containerName,
      search,
      limit,

      caseSens,
      startWith,
      hostName,
      signal,
    })
  ).reverse();

  return newLogs;
};

export const getPrevLogs = async function ({
  containerName = "",
  search = "",
  limit = 0,

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

export const checkLastLogTimeStamp = (lastTimeStamp, newTimestamp) => {
  const timestamp = new Date(newTimestamp).getTime() - 1;
  if (lastTimeStamp > timestamp) {
    return timestamp;
  }
};

export const forceToBottom = () => {
  console.log("force bottom");
  const logsContainerEl = document.querySelector("#logs");
  if (logsContainerEl) {
    logsContainerEl.scrollTop = logsContainerEl.scrollHeight;
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
