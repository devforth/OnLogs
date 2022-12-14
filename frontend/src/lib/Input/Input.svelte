<script>
  export let width = 200;
  export let height = 40;
  export let value = "";
  export let customClass = "";
  export let thumbClass = "";
  export let placeholder = "";
</script>

<div class={thumbClass}>
  <input
    class={customClass}
    type="text"
    {value}
    placeholder="Password"
    style:width={`${width}px`}
    style:height={`${height}px`}
  />
</div>
