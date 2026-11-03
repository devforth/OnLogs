<script>
  import { tick } from "svelte";
  import { tryToParseLogString } from "../../utils/functions";
  import { toAnsiHtml, stripAnsi } from "../../utils/ansi";
  import { searchRegex } from "../../utils/highlight";
  import { chosenStatus } from "../../Stores/stores";
  import { store } from "../../Stores/stores.js";
  let {
    status = "",
    time = "",
    message = "",
    width = "",
    isHiglighted = false,
    // Null hides the button rather than rendering one that does nothing.
    sharedLinkCallBack = null,
    showContextCallBack = null,
    statusFilterable = true,
    // The active search: a long line whose matches are all past the preview
    // opens whole. onToggle re-applies the highlight the new text lost.
    search = "",
    caseSens = false,
    onToggle = null,
  } = $props();

  // A page of 128 KiB lines froze the view; the rest is one click away.
  const PREVIEW_CHARS = 5000;
  let text = $derived(message ?? "");
  let isLong = $derived(text.length > PREVIEW_CHARS);
  // The cut must not leave half an escape sequence behind.
  let preview = $derived(
    isLong ? text.slice(0, PREVIEW_CHARS).replace(/[\u001B\u009B][^A-Za-z]*$/, "") : text
  );
  let matchHidden = $derived.by(() => {
    const regex = isLong ? searchRegex(search, caseSens) : null;
    if (!regex) {
      return false;
    }
    const inPreview = regex.test(stripAnsi(preview));
    regex.lastIndex = 0;
    return !inPreview && regex.test(stripAnsi(text));
  });
  // Null until the reader toggles; until then a hidden match decides.
  let userExpanded = $state(null);
  let expanded = $derived(userExpanded ?? matchHidden);
  let collapsed = $derived(isLong && !expanded);
  let shown = $derived(collapsed ? preview : text);

  async function toggle() {
    userExpanded = !expanded;
    await tick();
    onToggle?.();
  }
  let parsedStr = $derived(collapsed ? null : tryToParseLogString(text));
  let messageHtml = $derived(toAnsiHtml(shown));
</script>

<div
  class="logsString {isHiglighted ? 'new' : ''} {message?.trim().length === 0
    ? 'emptyLogsString'
    : ''}"
  style="width: {width}px"
>
  <div
    onclick={async () => {
      if (!statusFilterable) {
        return;
      }
      if ($chosenStatus !== status) {
        chosenStatus.set(status);
      } else {
        chosenStatus.set("");
      }
    }}
    class="status {status ? status : 'hidden'} {status === $chosenStatus
      ? 'chosenStatus'
      : ''}"><p><span> ◉ </span>{status.toUpperCase()}</p></div
  >

  <div class="time row_group"
    ><p>{message?.trim()?.length > 0 ? time : ""}</p>
    <div>
      {#if message?.trim()?.length > 0 && showContextCallBack}
        <div
          id={`thumb-context-${time}`}
          class="contextButtonThumb"
          title="Show context"
          onclick={() => {
            showContextCallBack();
          }}
        >
          <i class="log log-Eye" id={`context-${time}`}></i>
        </div>{/if}
      {#if message?.trim()?.length > 0 && sharedLinkCallBack}
        <div
          id={`thumb-shared-${time}`}
          class="shareLinkButtonThumb"
          title="Copy link to this line"
          onclick={() => {
            sharedLinkCallBack();
          }}
        >
          <i class="log log-ShareLink" id={`shared-${time}`}></i>
        </div>{/if}
    </div>
  </div>
  <div class="message">
    {#if !parsedStr}<p>
        {@html messageHtml}
      </p>{:else if $store.transformJson}<p>{@html toAnsiHtml(parsedStr.startText)}</p>
      <pre>{JSON.stringify(parsedStr.json, null, 2)}</pre>
      <p>{@html toAnsiHtml(parsedStr.endText)}</p>
    {:else}<p>
        {@html messageHtml}
      </p>{/if}
    {#if isLong}<button class="showAll" onclick={toggle}
        >{expanded
          ? "Show less"
          : `Show all (${Math.ceil((text.length - PREVIEW_CHARS) / 1024)} KB more)`}</button
      >{/if}
  </div>
</div>
