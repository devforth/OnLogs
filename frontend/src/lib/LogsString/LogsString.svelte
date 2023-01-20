<script>
  export let status = "";
  export let time = "";
  export let message = "";
  export let width = "";
  export let isHiglighted = false;
  import { lastLogTimestamp, store } from "../../Stores/stores.js";
  import { transformLogString } from "../../Views/Logs/functions.js";
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

  <td class="time"><p>{message?.trim()?.length > 0 ? time : ""}</p></td>
  <td class="message"><p>{message}</p></td>
</tr>
