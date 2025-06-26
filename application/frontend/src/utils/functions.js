import json2html from "json-to-html";

export const handleKeydown = (e, keyValue, cb) => {
  if (e.key === keyValue) {
    cb();
  }
};

export const emulateData = (amount) => {
  const randomArray = (length, max) =>
    [...new Array(length)].map(() => Math.round(Math.random() * max));

  let data = {
    dates: new Array(amount).fill("1"),
    debug: randomArray(10, 1000),
    error: randomArray(10, 1000),
    info: randomArray(10, 1000),
    warn: randomArray(10, 1000),
    other: randomArray(10, 1000),
  };
  return data;
};

export const tryToParseLogString = (str) => {
  const beginningOfJson = str.search(/[{[]/);
  const endingOfJson = str.search(/[\]}](?![\s\S]*[\]}])/);

  let html = "";
  let startText = "";
  let endText = "";

  if (beginningOfJson !== -1 && endingOfJson !== -1 && endingOfJson > beginningOfJson) {
    const jsonPart = str.slice(beginningOfJson, endingOfJson + 1);
    startText = str.slice(0, beginningOfJson);
    endText = str.slice(endingOfJson + 1);

    try {
      const parsed = JSON.parse(jsonPart);
      html = json2html(parsed, 2);
    } catch (e) { }
  }

  if (html) {
    return { startText, html, endText };
  } else return null;
};

export const copyText = function (ref, cb) {
  const text = ref;
  let textToCopy = text.innerText;
  if (navigator.clipboard) {
    navigator.clipboard.writeText(textToCopy).then(() => {
      cb();
    });
  } else {
    console.log("Browser Not compatible");
  }
};

export const copyCustomText = function (text, cb) {
  let textToCopy = text;
  if (navigator.clipboard) {
    navigator.clipboard.writeText(textToCopy).then(() => {
      cb();
    });
  } else {
    console.log("Browser Not compatible");
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

  return [showIfExisted(hours, "h"), showIfExisted(minutes, "m")];
}
