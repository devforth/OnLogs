<script>
  export let status="";
  export let time="";
  export let message=""
  let blankStyle = " border-top: solid 1px $lines-color;"

  let blank = message.localeCompare("") == 0 || message.localeCompare(" ") == 0
  if (blank) {
    blankStyle = " border-top: 0px; height: 10px;"
  }
</script>

<div class="logsString" style="{blankStyle}">
  {#if status != ""} 
    <div class="status {status}"><p><span> ◉ </span>{status.toUpperCase()}</p></div>
  {/if}

  {#if status != ""}
    <div class="time"><p>{time}</p></div>
  {:else}
    <div class="time" style="margin-left: 85px;"><p>{time}</p></div>
  {/if}
  <div class="message"><p>{message}</p></div>
</div>




 