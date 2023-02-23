<script>
  import { tryToParseLogString } from "../../utils/functions";
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
        {@html message}
      </p>{:else if $store.transformJson}<p>{parsedStr.startText}</p>
      <pre>{@html parsedStr.html}</pre>
      <p>{parsedStr.endText}</p>
    {:else}<p>
        {@html message}
      </p>{/if}
  </td>
</tr>
