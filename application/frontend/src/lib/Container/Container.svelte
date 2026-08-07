<script>
  let {
    highlighted = false,
    minHeightVh = 0,
    maxHeightVh = 100,
    paddingOff = false,
    noShadows = false,
    children
  } = $props();
</script>

<div
  class="container {noShadows ? '' : 'border'} block {highlighted
    ? 'highlighted'
    : ''} {paddingOff ? 'paddingOff' : ''}"
  style:min-height={`${minHeightVh}vh`}
  style={maxHeightVh !== 0 && `max-height: ${maxHeightVh}vh`}
>
  {@render children?.()}
</div>
