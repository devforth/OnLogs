<script>
  import {
    toastIsVisible,
    toast,
    toastTimeoutId,
  } from "../../Stores/stores.js";
  import ProgressBar from "../ProgressBar/ProgressBar.svelte";
  import Button from "../Button/Button.svelte";
  import { handleKeydown } from "../../utils/functions.js";
  import { onDestroy, onMount } from "svelte";
  import { fly } from "svelte/transition";

  onDestroy(() => {
    if ($toastTimeoutId) {
      clearTimeout($toastTimeoutId);
    }
  });
</script>

<div
  transition:fly={{ y: -200, duration: 200 }}
  class="toastContainer {$toast.status}"
>
  <div class="toastIcoContainer"><i class="log log-{$toast.status}" /></div>
  <h4>{$toast.tittle}</h4>
  <p>{$toast.message}</p>

  <!-- <ProgressBar /> -->

  <div class="toastButtonContainer">
    <Button
      id={"toastClose"}
      title={"Close"}
      minHeight={24}
      CB={() => {
        toastIsVisible.set(false);
      }}
    />
  </div>
</div>

<svelte:window
  on:keydown={(e) => {
    handleKeydown(e, "Escape", () => {
      toastIsVisible.set(false);
      if (toastTimeoutId) {
        clearTimeout($toastTimeoutId);
      }
    });
  }}
/>
