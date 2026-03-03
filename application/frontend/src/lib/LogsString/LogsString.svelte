<script>
  import { tryToParseLogString } from "../../utils/functions";
  import { toAnsiHtml } from "../../utils/ansi";
  import { chosenStatus } from "../../Stores/stores";
  import fetchApi from "../../utils/fetch";
  export let status = "";
  export let time = "";
  export let message = "";
  export let width = "";
  export let isHiglighted = false;
  export let sharedLinkCallBack = () => {};
  export let getLogsByTagOptions = {};

  import { store } from "../../Stores/stores.js";

  let activeStatus = "";
  $: parsedStr = tryToParseLogString(message);
  $: messageHtml = toAnsiHtml(message);
</script>

<tr
  class="logsString {isHiglighted ? 'new' : ''} {message?.trim().length === 0
    ? 'emptyLogsString'
    : ''}"
  style="width: {width}px"
>
  <td
    on:click={async () => {
      if ($chosenStatus !== status) {
        chosenStatus.set(status);
      } else {
        chosenStatus.set("");
      }
    }}
    class="status {status ? status : 'hidden'} {status === $chosenStatus
      ? 'chosenStatus'
      : ''}"><p><span> ◉ </span>{status.toUpperCase()}</p></td
  >

  <td class="time row_group"
    ><p>{message?.trim()?.length > 0 ? time : ""}</p>
    <div>
      {#if message?.trim()?.length > 0}
        <div
          id={`thumb-shared-${time}`}
          class="shareLinkButtonThumb"
          on:click={() => {
            sharedLinkCallBack();
          }}
        >
          <i class="log log-ShareLink" id={`shared-${time}`} />
        </div>{/if}
    </div>
  </td>
  <td class="message">
    {#if !parsedStr}<p>
        {@html messageHtml}
      </p>{:else if $store.transformJson}<p>{@html toAnsiHtml(parsedStr.startText)}</p>
      <pre>{@html parsedStr.html}</pre>
      <p>{@html toAnsiHtml(parsedStr.endText)}</p>
    {:else}<p>
        {@html messageHtml}
      </p>{/if}
  </td>
</tr>
