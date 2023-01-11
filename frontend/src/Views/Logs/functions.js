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
    ? str.at(0).slice(0, 19).replace("T", " ")
    : new Date(
        new Date().setTime(
          new Date(str.at(0).slice(0, 19).replace("T", " ")).getTime() -
            timezoneOffsetSec * 1000
        )
      ).toLocaleString("sv-SE");
};

export const getLogs = async function ({
  containerName = "",
  search = "",
  limit = 0,
  offset,
  caseSens = false,
  hostName = "",
  startWith,
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
  console.log(el);
  if (!el) {
    return;
  } else {
    el.scrollIntoView({ behavior: "smooth" });
  }
};

export const scrollToNewLogsEnd = () => {
  const el = document.querySelector(".newLogsEnd");
  console.log(el);
  if (!el) {
    return;
  } else {
    el.scrollIntoView();
  }
};
