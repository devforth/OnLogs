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
