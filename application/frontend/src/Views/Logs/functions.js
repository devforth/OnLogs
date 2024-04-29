import FetchApi from "../../utils/fetch";

const api = new FetchApi();

export const timezoneOffsetSec = new Date().getTimezoneOffset() * 60;

export const getLogLineStatus = (logLine = "") => {
  const statuses_errors = ["ERROR", "ERR", "Error", "Err", "error"];
  const statuses_warnings = ["WARN", "WARNING", "warning"];
  const statuses_other = ["DEBUG", "INFO", "ONLOGS", "debug", "info", "onlogs"];
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
        return statuses_other[j].toLowerCase() === "onlogs"
          ? "meta"
          : statuses_other[j].toLowerCase();
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
      )
        .toLocaleString("sv-EN", {
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
          fractionalSecondDigits: 3,
        })
        .replace(",", ".")
    : new Date(
        new Date().setTime(
          new Date(t?.at(0)?.slice(0, 19)?.replace("T", " "))?.getTime() -
            timezoneOffsetSec * 1000
        )
      )
        .toLocaleString("sv-EN", {
          hour: "2-digit",
          minute: "2-digit",
          second: "2-digit",
          fractionalSecondDigits: 3,
        })
        .replace(",", ".");
};

export const transformLogStringForTimeBudget = (t, options) => {
  return options
    ? new Date(
        new Date().setTime(
          new Date(t?.at(0)?.slice(0, 22)?.replace("T", " "))?.getTime()
        )
      )
        .toLocaleString("en-US", {
          month: "short",
          day: "2-digit",
          year: "numeric",
        })
        .replace(",", "")
    : new Date(
        new Date().setTime(
          new Date(t?.at(0)?.slice(0, 19)?.replace("T", " "))?.getTime() -
            timezoneOffsetSec * 1000
        )
      )
        .toLocaleString("en-US", {
          month: "short",
          day: "2-digit",
          year: "numeric",
        })
        .replace(",", "");
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

export const checkLastLogTimeStamp = (lastTimeStamp, newTimestamp) => {
  const timestamp = new Date(newTimestamp).getTime() - 1;
  if (lastTimeStamp > timestamp) {
    return timestamp;
  }
};

export const forceToBottom = () => {
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

export const findSearchTextInLogs = (sel, searchText, caseSens) => {
  const nodes = document.querySelectorAll(sel);

  let regex = caseSens
    ? new RegExp(searchText, "g")
    : new RegExp(searchText, "gi");
  nodes.forEach((n) => {
    n.innerHTML = n.innerHTML.replace(regex, function (match, n) {
      return `<span class="searchedText">${match}</span>`;
    });
  });
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
