<script>
  import { toastIsVisible, toast } from "../../Stores/stores.js";
</script>

{#if $toastIsVisible}
  <div class="toastContainer {$toast.status}">
    <h4>{$toast.tittle}</h4>
    <p>{$toast.message}</p>
  </div>
{/if}
