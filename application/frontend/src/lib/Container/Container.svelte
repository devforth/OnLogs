<script>
  export let highlighted = false;
  export let minHeightVh = 0;
  export let maxHeightVh = 100;
  export let paddingOff = false;
  export let noShadows = false;
</script>

<div
  class="container {noShadows ? '' : 'border'} block {highlighted
    ? 'highlighted'
    : ''} {paddingOff ? 'paddingOff' : ''}"
  style:min-height={`${minHeightVh}vh`}
  style={maxHeightVh !== 0 && `max-height: ${maxHeightVh}vh`}
>
  <slot />
</div>
