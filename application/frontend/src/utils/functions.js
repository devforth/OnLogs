import { toast, toastIsVisible } from "../Stores/stores.js";


export const handleKeydown = (e, keyValue, cb) => {
  if (e.key === keyValue) {
    cb();
  }
};

export const tryToParseLogString = (str) => {
  const beginningOfJson = str.search(/[{[]/);
  const endingOfJson = str.search(/[\]}](?![\s\S]*[\]}])/);

  if (beginningOfJson === -1 || endingOfJson === -1 || endingOfJson <= beginningOfJson) {
    return null;
  }

  try {
    return {
      startText: str.slice(0, beginningOfJson),
      json: JSON.parse(str.slice(beginningOfJson, endingOfJson + 1)),
      endText: str.slice(endingOfJson + 1),
    };
  } catch (e) {
    return null;
  }
};

export const copyText = function (ref, cb) {
  copyCustomText(ref.innerText, cb);
};

// navigator.clipboard only exists in secure contexts; OnLogs is often served
// over plain HTTP.
function copyWithTextarea(text) {
  const focused = document.activeElement;
  const area = document.createElement("textarea");
  area.value = text;
  area.setAttribute("readonly", "");
  area.style.position = "fixed";
  area.style.opacity = "0";
  document.body.appendChild(area);
  area.focus();
  area.select();
  let copied = false;
  try {
    copied = document.execCommand("copy");
  } catch {
    copied = false;
  }
  area.remove();
  focused?.focus?.();
  return copied;
}

export const copyCustomText = function (text, cb) {
  const fallback = () => {
    if (copyWithTextarea(text)) {
      cb();
      return;
    }
    toast.set({
      tittle: "Error",
      message: "Could not copy to the clipboard.",
      position: "",
      status: "Error",
    });
    toastIsVisible.set(true);
  };
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).then(cb, fallback);
  } else {
    fallback();
  }
};

export function getTimeDifference(t) {
  const now = Date.now();
  const timestamp = Date.parse(t);
  const difference = Math.abs(now - timestamp) / 1000; // difference in seconds

  const hours = Math.floor((difference % 86400) / 3600);
  const minutes = Math.floor((difference % 3600) / 60);

  function showIfExisted(v, time) {
    if (v || v === 0) {
      return [v, time];
    }
    return "";
  }

  // Days lead only when present: StreamInfo tells "Recently" and "No logs"
  // apart by the first two parts.
  const parts = [showIfExisted(hours, "h"), showIfExisted(minutes, "m")];
  const days = Math.floor(difference / 86400);
  return days > 0 ? [[days, "d"], ...parts] : parts;
}
