<script>
    import Container from "./Container.svelte";
  /**
   * @typedef {Object} Props
   * @property {boolean} [newHighlighted]
   */

  /** @type {Props} */
  let { newHighlighted = false } = $props();
   
</script>

<Container highlighted={newHighlighted}>
<p>Slot 1</p>
  <p>Slot 2</p>
  <p>Slot 3</p>
</Container>