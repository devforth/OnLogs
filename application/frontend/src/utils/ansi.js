import AnsiToHtml from "ansi-to-html";

// Covers ANSI CSI and related escape sequences.
const ANSI_PATTERN =
  /[\u001B\u009B][[\]()#;?]*(?:(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><])/g;

const ansiConverter = new AnsiToHtml({
  newline: false,
  escapeXML: true,
  stream: false,
});

export function stripAnsi(value = "") {
  return String(value).replace(ANSI_PATTERN, "");
}

// The converter renders SGR (colour) codes only and leaves the final letter of
// any other sequence in the text, so those are dropped first.
export function toAnsiHtml(value = "") {
  return ansiConverter.toHtml(
    String(value).replace(ANSI_PATTERN, (seq) => (seq.endsWith("m") ? seq : ""))
  );
}
