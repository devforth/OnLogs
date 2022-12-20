<script>
  export let toastOptions = {
    message: "",
    tittle: "",
    position: "",
    status: "",
  };
  let toastsIsVisible = false;
</script>

{#if !toastsIsVisible}
  <div class="toastContainer {toastOptions.status}">
    <h4>{toastOptions.tittle}</h4>
    <p>{toastOptions.message}</p>
  </div>
{/if}
