import fs from "fs-extra";

const srcDir = `./dist`;
const destDir = `../backend/dist`;

// To copy a folder or file, select overwrite accordingly
try {
  fs.copySync(srcDir, destDir, { overwrite: true });
  console.log("success!");
} catch (err) {
  console.error(err);
}
