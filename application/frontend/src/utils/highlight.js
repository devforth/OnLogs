const highlightStateByNode = new WeakMap();

export const findSearchTextInLogs = (sel, searchText, caseSens) => {
  const normalizedSearchText =
    typeof searchText === "string" ? searchText.trim() : "";

  if (!sel || !normalizedSearchText) return;

  const nodes = document.querySelectorAll(sel);
  if (!nodes.length) {
    return;
  }

  const escapeRegExp = (str = "") => str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const buildSearchPattern = (str = "") => {
    // Make whitespace robust against ANSI-driven node boundaries (\n, \t, multi-space).
    return escapeRegExp(str).replace(/\s+/g, "\\s+");
  };
  const regex = caseSens
    ? new RegExp(buildSearchPattern(normalizedSearchText), "g")
    : new RegExp(buildSearchPattern(normalizedSearchText), "gi");
  const queryKey = `${caseSens ? "1" : "0"}:${normalizedSearchText}`;

  const unwrapHighlights = (root, hadHighlights) => {
    if (!hadHighlights) {
      return;
    }
    root.querySelectorAll(".searchedText").forEach((el) => {
      const parent = el.parentNode;
      if (!parent) {
        return;
      }
      parent.replaceChild(document.createTextNode(el.textContent || ""), el);
      parent.normalize();
    });
  };

  const collectTextNodes = (root) => {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    const textNodes = [];
    let fullText = "";
    let cursor = 0;
    let current;

    while ((current = walker.nextNode())) {
      const text = current.nodeValue || "";
      if (!text.length) {
        continue;
      }
      const end = cursor + text.length;
      textNodes.push({
        node: current,
        start: cursor,
        end,
      });
      fullText += text;
      cursor = end;
    }
    return { textNodes, fullText };
  };

  const findNodeAtOffset = (entries, offset) => {
    for (let i = 0; i < entries.length; i++) {
      const entry = entries[i];
      if (offset >= entry.start && offset <= entry.end) {
        return {
          node: entry.node,
          offset: offset - entry.start,
        };
      }
    }
    if (entries.length && offset === entries[entries.length - 1].end) {
      const last = entries[entries.length - 1];
      return { node: last.node, offset: (last.node.nodeValue || "").length };
    }
    return null;
  };

  const highlightByRanges = (root) => {
    const { textNodes, fullText } = collectTextNodes(root);
    if (!textNodes.length || !fullText.length) {
      return false;
    }

    const ranges = [];
    let match;
    regex.lastIndex = 0;
    while ((match = regex.exec(fullText)) !== null) {
      if (!match[0].length) {
        break;
      }
      ranges.push({
        start: match.index,
        end: match.index + match[0].length,
      });
    }

    if (!ranges.length) {
      return false;
    }

    let foundMatches = false;
    for (let i = ranges.length - 1; i >= 0; i--) {
      const rangeInfo = ranges[i];
      const startPos = findNodeAtOffset(textNodes, rangeInfo.start);
      const endPos = findNodeAtOffset(textNodes, rangeInfo.end);
      if (!startPos || !endPos) {
        continue;
      }

      const range = document.createRange();
      range.setStart(startPos.node, startPos.offset);
      range.setEnd(endPos.node, endPos.offset);

      const span = document.createElement("span");
      span.className = "searchedText";
      const fragment = range.extractContents();
      span.appendChild(fragment);
      range.insertNode(span);
      if (span.textContent) {
        foundMatches = true;
      }
    }

    return foundMatches;
  };

  nodes.forEach((node) => {
    const prevState = highlightStateByNode.get(node);
    const hadHighlights = prevState?.hasHighlights || false;
    const currentText = node.textContent || "";

    if (
      prevState?.queryKey === queryKey &&
      prevState?.textSnapshot === currentText &&
      hadHighlights
    ) {
      return;
    }

    unwrapHighlights(node, hadHighlights);
    const hasHighlights = highlightByRanges(node);
    highlightStateByNode.set(node, {
      queryKey,
      hasHighlights,
      textSnapshot: node.textContent || "",
    });
  });
};
