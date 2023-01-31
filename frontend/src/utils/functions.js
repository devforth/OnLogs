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
  const beginningOfJson = str.indexOf("{");
  const endingOfJson = str.lastIndexOf("}");

  let html = "";
  let startText = "";
  let endText = "";

  if (beginningOfJson !== -1 && endingOfJson !== -1) {
    if (endingOfJson > beginningOfJson) {
      const jsonPart = str.slice(beginningOfJson, endingOfJson);
      startText = str.slice(0, beginningOfJson);
      endText = str.slice(endingOfJson + 1, -1);
      try {
        let normilizedStr = JSON.parse(jsonPart + "}");
        html = json2html(normilizedStr, 2);
      } catch (e) {
        console.log(e);
      }
    }
  }

  if (html) {
    return { startText, html, endText };
  } else return null;
};
