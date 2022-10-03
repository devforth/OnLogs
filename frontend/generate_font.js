import webfontsGenerator from "@vusion/webfonts-generator";
import fs from "fs";

fs.readdir("src/assets/res/font", function (err, items) {
  if (err) {
    
    console.log("cant read res directory");
  }
  const files = items
    .filter((i) => i.toLowerCase().endsWith(".svg"))
    .map((i) => { return `src/assets/res/font/${i}` });
 

  webfontsGenerator(
    {
      files: files,
      dest: "src/assets/res",
      fontName: "onLogsFont",

      // https://github.com/nfroidure/svgicons2svgfont options
      normalize: true,

      cssTemplate: "src/assets/res/font/font-css.hbs",
      templateOptions: {
        classPrefix: "log-",
        baseSelector: ".log",
      },
      types: ["svg", "ttf", "woff", "woff2", "eot"],
    },
    function (error) {
      if (error) {
        console.log("Fail!", error);
      } else {
        console.log("Done!");
      }
    }
  );
});
