<script>
  import { tryToParseLogString } from "../../utils/functions";
  export let status = "";
  export let time = "";
  export let message = "";
  export let width = "";
  export let isHiglighted = false;
  export let sharedLinkCallBack = () => {};

  $: parsedStr = tryToParseLogString(message);
  import { store } from "../../Stores/stores.js";
</script>

<tr
  class="logsString {isHiglighted ? 'new' : ''} {message?.trim().length === 0
    ? 'emptyLogsString'
    : ''}"
  style="width: {width}px"
>
  <td class="status {status ? status : 'hidden'}"
    ><p><span> ◉ </span>{status.toUpperCase()}</p></td
  >

  <td class="time row_group"
    ><p>{message?.trim()?.length > 0 ? time : ""}</p>
    {#if message?.trim()?.length > 0}
      <div
        id={`thumb-shared-${time}`}
        class="shareLinkButtonThumb"
        on:click={() => {
          console.log("click to");
          sharedLinkCallBack();
        }}
      >
        <i class="log log-ShareLink" id={`shared-${time}`} />
      </div>{/if}
  </td>
  <td class="message"
    >{#if !parsedStr}<p>
        {message}
      </p>{:else}
      {#if $store.transformJson}<p>{parsedStr.startText}</p>
        <pre>{@html parsedStr.html}</pre>
        <p>{parsedStr.endText}</p>
      {:else}<p>
          {message}
        </p>{/if}{/if}
  </td>
</tr>
