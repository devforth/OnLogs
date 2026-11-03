const highlightStateByNode = new WeakMap();

const escapeRegExp = (str = "") => str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// Whitespace in the term matches any whitespace run: ANSI node boundaries split
// it into \n, \t or several spaces. Null for an empty term.
export const searchRegex = (searchText, caseSens) => {
  const term = typeof searchText === "string" ? searchText.trim() : "";
  if (!term) {
    return null;
  }
  return new RegExp(escapeRegExp(term).replace(/\s+/g, "\\s+"), caseSens ? "g" : "gi");
};

export const findSearchTextInLogs = (sel, searchText, caseSens) => {
  const normalizedSearchText =
    typeof searchText === "string" ? searchText.trim() : "";

  if (!sel || !normalizedSearchText) return;

  const nodes = document.querySelectorAll(sel);
  if (!nodes.length) {
    return;
  }

  const regex = searchRegex(normalizedSearchText, caseSens);
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

  // Binary search for the first entry whose end reaches the offset; entries
  // are in document order, so this matches the former linear scan.
  const findNodeAtOffset = (entries, offset) => {
    let low = 0;
    let high = entries.length - 1;
    while (low < high) {
      const mid = (low + high) >> 1;
      if (entries[mid].end < offset) low = mid + 1;
      else high = mid;
    }
    const entry = entries[low];
    if (entry && offset >= entry.start && offset <= entry.end) {
      return {
        node: entry.node,
        offset: offset - entry.start,
      };
    }
    if (entries.length && offset === entries[entries.length - 1].end) {
      const last = entries[entries.length - 1];
      return { node: last.node, offset: (last.node.nodeValue || "").length };
    }
    return null;
  };

  // One Range for every match: each live Range is updated on every DOM change,
  // so a new one per match made highlighting quadratic.
  const range = document.createRange();

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
