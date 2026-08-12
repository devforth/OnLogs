import { globSync } from "node:fs";
import { spawnSync } from "node:child_process";

const files = globSync("src/**/*.test.mjs").sort();
if (files.length === 0) {
  console.error("no test files matched src/**/*.test.mjs");
  process.exit(1);
}

let failed = 0;
for (const file of files) {
  const result = spawnSync(process.execPath, [file], { stdio: "inherit" });
  if (result.status !== 0) {
    console.error(`FAIL ${file}`);
    failed += 1;
  }
}

console.log(`\n${files.length - failed}/${files.length} test files passed`);
process.exit(failed === 0 ? 0 : 1);
