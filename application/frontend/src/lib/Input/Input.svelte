<script>
  let {
    width = 200,
    height = 40,
    value = $bindable(""),
    customClass = "",
    thumbClass = "",
    placeholder = "",
    ...rest
  } = $props();
</script>

<div class={thumbClass}>
  <input
    bind:value
    class={customClass}
    type="text"
    {placeholder}
    {...rest}
    style:width={`${width}px`}
    style:height={`${height}px`}
  />
</div>
