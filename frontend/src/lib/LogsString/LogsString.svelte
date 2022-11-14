<script>
  export let status="debug";
  export let time="";
  export let message=""
</script>


<div class="logsString">
  <div class="status {status}"><p> <span> ◉ </span>{status.toUpperCase()}</p></div>
  <div class="time"><p>{time}</p></div>
  <div class="message"><p>{message}</p></div>
</div>




 