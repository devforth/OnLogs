export const handleKeydown = (e, keyValue, cb) => {
  if (e.key === keyValue) {
    cb();
  }
};
