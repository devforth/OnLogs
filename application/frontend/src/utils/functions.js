
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
