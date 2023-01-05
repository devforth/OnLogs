<script>
  export let highlighted = false;
  export let minHeightVh = 10;
  export let maxHeightVh = 1;
  export let paddingOff = false;
</script>

<div
  class="container border block {highlighted ? 'highlighted' : ''} {paddingOff
    ? 'paddingOff'
    : ''}"
  style:min-height={`${minHeightVh}vh`}
  style="max-height: {maxHeightVh}vh;"
>
  <slot />
</div>
