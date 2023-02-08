<script>
  import { toastIsVisible, toast } from "../../Stores/stores.js";
  import ProgressBar from "../ProgressBar/ProgressBar.svelte";
  import Button from "../Button/Button.svelte";
</script>

{#if $toastIsVisible}
  <div class="toastContainer {$toast.status}">
    <div class="toastIcoContainer"><i class="log log-{$toast.status}" /></div>
    <h4>{$toast.tittle}</h4>
    <p>{$toast.message}</p>

    <ProgressBar />

    <div class="toastButtonContainer">
      <Button
        title={"Close"}
        minHeight={24}
        CB={() => {
          toastIsVisible.set(false);
        }}
      />
    </div>
  </div>
{/if}
