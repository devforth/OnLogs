// Bundles a Svelte component with the repo's own Svelte and mounts it under
// jsdom, so component behaviour can be asserted headlessly.
import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import esbuild from "esbuild";
import * as svelte from "svelte/compiler";
import { JSDOM } from "jsdom";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");

const sveltePlugin = {
  name: "svelte",
  setup(build) {
    build.onLoad({ filter: /\.svelte$/ }, (args) => {
      const source = readFileSync(args.path, "utf8");
      const { js } = svelte.compile(source, {
        filename: args.path,
        generate: "dom",
        css: false,
        dev: false,
        // Test-only: exposes props on the instance so assertions can read them.
        accessors: true,
      });
      return { contents: js.code, warnings: [] };
    });
  },
};

// esbuild 0.15 has no "empty" loader; assets are irrelevant to behaviour tests.
const emptyAssetPlugin = {
  name: "empty-assets",
  setup(build) {
    build.onResolve({ filter: /\.(css|scss|svg|png|woff2?|ttf|eot)$/ }, (args) => ({
      path: args.path,
      namespace: "empty-asset",
    }));
    build.onLoad({ filter: /.*/, namespace: "empty-asset" }, () => ({
      contents: "export default {};",
      loader: "js",
    }));
  },
};

const aliasPlugin = {
  name: "alias",
  setup(build) {
    build.onResolve({ filter: /^svelte-intersection-observer$/ }, () => ({
      path: join(ROOT, "test/stubs/IntersectionObserver.svelte"),
    }));
  },
};

export async function bundleComponent(entryRelativePath) {
  const outdir = mkdtempSync(join(tmpdir(), "onlogs-harness-"));
  const outfile = join(outdir, "bundle.mjs");

  await esbuild.build({
    entryPoints: [join(ROOT, entryRelativePath)],
    outfile,
    bundle: true,
    format: "esm",
    platform: "browser",
    mainFields: ["svelte", "browser", "module", "main"],
    conditions: ["svelte", "browser", "import"],
    logLevel: "silent",
    plugins: [aliasPlugin, sveltePlugin, emptyAssetPlugin],
  });

  return outfile;
}

class FakeWebSocket {
  constructor(url) {
    this.url = url;
    this.readyState = 1;
    FakeWebSocket.instances.push(this);
  }
  send() {}
  close() {
    this.readyState = 3;
  }
}
FakeWebSocket.instances = [];

export function installDom({ fetchImpl } = {}) {
  const dom = new JSDOM("<!doctype html><html><body></body></html>", {
    url: "http://onlogs.test/",
    pretendToBeVisual: true,
  });

  const { window } = dom;
  const define = (name, value) =>
    Object.defineProperty(globalThis, name, {
      value,
      writable: true,
      configurable: true,
    });

  globalThis.window = window;
  globalThis.document = window.document;
  define("navigator", window.navigator);
  define("location", window.location);
  globalThis.HTMLElement = window.HTMLElement;
  globalThis.Element = window.Element;
  globalThis.Node = window.Node;
  globalThis.Event = window.Event;
  globalThis.CustomEvent = window.CustomEvent;
  globalThis.getComputedStyle = window.getComputedStyle.bind(window);
  globalThis.requestAnimationFrame =
    window.requestAnimationFrame || ((cb) => setTimeout(() => cb(Date.now()), 16));
  globalThis.cancelAnimationFrame = window.cancelAnimationFrame || clearTimeout;

  // jsdom implements no scrolling at all.
  window.Element.prototype.scrollTo = function () {};
  window.Element.prototype.scrollIntoView = function () {};
  window.scrollTo = () => {};

  // jsdom implements neither of these.
  globalThis.IntersectionObserver = window.IntersectionObserver = class {
    constructor(cb) {
      this.cb = cb;
    }
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  FakeWebSocket.instances = [];
  globalThis.WebSocket = window.WebSocket = FakeWebSocket;

  if (fetchImpl) {
    globalThis.fetch = fetchImpl;
    window.fetch = fetchImpl;
  }

  return { dom, window, sockets: FakeWebSocket.instances };
}

export function jsonResponse(payload, status = 200) {
  return {
    status,
    ok: status >= 200 && status < 300,
    json: async () => payload,
  };
}

// Lets every queued microtask and 0ms timer drain, which is what a Svelte
// reactive flush plus its awaited fetches need.
export async function settle(rounds = 30) {
  for (let i = 0; i < rounds; i++) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

export async function importBundle(outfile) {
  return import(pathToFileURL(outfile).href);
}
