<script>
    import Container from "./Container.svelte";
    export let newHighlighted=false
   
</script>

<Container highlighted={newHighlighted}>
<p>Slot 1</p>
  <p>Slot 2</p>
  <p>Slot 3</p>
</Container>