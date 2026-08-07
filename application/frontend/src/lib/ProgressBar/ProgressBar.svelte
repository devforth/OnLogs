<script>
  //   import { progress } from "../../Stores/progressBar.js";
  import { tweened } from "svelte/motion";
  import { cubicOut } from "svelte/easing";
  import { onMount } from "svelte";
  const progress = tweened(0, {
    duration: 5000,
    // easing: cubicOut,
  });
  onMount(() => {
    progress.set(100);
  });
</script>

<div class="progressBarContainer">
  <div style="width: {$progress}%; " class="progressBarValue"></div>
</div>
