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
    // .svelte.js modules may use runes, so they go through compileModule.
    build.onLoad({ filter: /\.svelte\.js$/ }, (args) => {
      const source = readFileSync(args.path, "utf8");
      const { js } = svelte.compileModule(source, {
        filename: args.path,
        generate: "dom",
        dev: false,
      });
      return { contents: js.code, warnings: [] };
    });
    build.onLoad({ filter: /\.svelte$/ }, (args) => {
      const source = readFileSync(args.path, "utf8");
      const { js } = svelte.compile(source, {
        filename: args.path,
        generate: "dom",
        // Svelte 5 replaced the boolean with "external" / "injected".
        css: "external",
        dev: false,
        // Dependencies still ship Svelte 4 source; only our own code is pinned.
        runes: args.path.includes("node_modules") ? undefined : true,
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
    // Mirrors the "@" -> src alias in vite.config.js.
    build.onResolve({ filter: /^@\// }, (args) => ({
      path: join(ROOT, "src", args.path.slice(2)),
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

// One window for the whole process. Svelte 5 checks `instanceof Node`, so nodes
// from a second jsdom window would fail against the first window's constructors.
let sharedDom = null;

function stubAnimations(window) {
  if (window.Element.prototype.animate) {
    return;
  }
  // jsdom has no Web Animations API; Svelte 5 transitions call element.animate().
  window.Element.prototype.animate = function () {
    const animation = {
      currentTime: 0,
      playState: "finished",
      startTime: 0,
      effect: { getComputedTiming: () => ({ duration: 0 }) },
      finished: Promise.resolve(),
      onfinish: null,
      oncancel: null,
      cancel() {},
      play() {},
      pause() {},
      finish() {},
      reverse() {},
      addEventListener() {},
      removeEventListener() {},
    };
    queueMicrotask(() => animation.onfinish && animation.onfinish());
    return animation;
  };
}

export function installDom({ fetchImpl } = {}) {
  if (sharedDom) {
    const { window } = sharedDom;
    window.document.body.innerHTML = "";
    FakeWebSocket.instances.length = 0;
    globalThis.__onlogsObservers = [];
    if (fetchImpl) {
      globalThis.fetch = fetchImpl;
      window.fetch = fetchImpl;
    }
    return { ...sharedDom, sockets: FakeWebSocket.instances };
  }

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

  // Svelte 5's runtime reaches for a much wider set of DOM globals than Svelte 3
  // did (Text, Comment, DocumentFragment, ...), so publish the whole jsdom
  // surface rather than guessing which ones matter.
  const skip = new Set(["window", "document", "navigator", "location", "top", "self", "parent", "frames", "globalThis"]);
  for (const key of Object.getOwnPropertyNames(window)) {
    if (skip.has(key) || key in globalThis) {
      continue;
    }
    const value = window[key];
    if (typeof value === "function" || (value && typeof value === "object")) {
      define(key, value);
    }
  }
  // Node 22 ships its own Event/CustomEvent/EventTarget, so the loop above skips
  // them — but jsdom rejects foreign Event instances. jsdom's must win.
  for (const key of [
    "Event", "CustomEvent", "EventTarget", "Node", "Text", "Comment", "Element",
    "HTMLElement", "DocumentFragment", "MutationObserver", "NodeFilter", "Range",
    "DOMParser", "NodeList", "HTMLCollection", "CSSStyleDeclaration",
  ]) {
    if (window[key]) {
      define(key, window[key]);
    }
  }

  globalThis.getComputedStyle = window.getComputedStyle.bind(window);
  globalThis.requestAnimationFrame =
    window.requestAnimationFrame || ((cb) => setTimeout(() => cb(Date.now()), 16));
  globalThis.cancelAnimationFrame = window.cancelAnimationFrame || clearTimeout;

  // jsdom implements no scrolling and no animations.
  window.Element.prototype.scrollTo = function () {};
  window.Element.prototype.scrollIntoView = function () {};
  window.scrollTo = () => {};
  stubAnimations(window);

  // jsdom implements neither of these.
  const StubIntersectionObserver = class {
    constructor(cb) {
      this.cb = cb;
    }
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  define("IntersectionObserver", StubIntersectionObserver);
  window.IntersectionObserver = StubIntersectionObserver;

  FakeWebSocket.instances.length = 0;
  define("WebSocket", FakeWebSocket);
  window.WebSocket = FakeWebSocket;
  globalThis.__onlogsObservers = [];

  if (fetchImpl) {
    globalThis.fetch = fetchImpl;
    window.fetch = fetchImpl;
  }

  sharedDom = { dom, window };
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

// Each test file is its own process, so one place owns the exit codes.
export function main(body) {
  body().then(
    () => process.exit(0),
    (err) => {
      console.error(err);
      process.exit(1);
    }
  );
}
