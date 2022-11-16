<script>
  export let status="debug";
  export let time="";
  export let message=""
  export let width=""
</script>


<div class="logsString" style="width: {width}px">
  <div class="status {status}"><p> <span> ◉ </span>{status.toUpperCase()}</p></div>
  <div class="time"><p>{time}</p></div>
  <div class="message"><p>{message}</p></div>
</div>




 