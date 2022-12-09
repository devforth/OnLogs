<script>
  export let status = "";
  export let time = "";
  export let message = "";
  export let width = "";
</script>

<tr
  class="logsString {message.trim().length === 0 ? 'emptiLogsString' : ''}"
  style="width: {width}px"
>
  <td class="status {status ? status : 'hidden'}"
    ><p><span> ◉ </span>{status.toUpperCase()}</p></td
  >

  <td class="time"><p>{message.trim().length > 0 ? time : ""}</p></td>
  <td class="message"><p>{message}</p></td>
</tr>
