<script>
  import {
    toastIsVisible,
    toast,
    toastTimeoutId,
  } from "../../Stores/stores.js";
  import Button from "../Button/Button.svelte";
  import { handleKeydown } from "../../utils/functions.js";
  import { onDestroy, onMount } from "svelte";
  import { fly } from "svelte/transition";
  // Reactive: the component is reused across toasts, so a non-reactive
  // destructure shows the previous message with the new icon.
  let { tittle, message, status, additionButton } = $derived($toast);

  onDestroy(() => {
    if ($toastTimeoutId) {
      clearTimeout($toastTimeoutId);
    }
  });
</script>

<div
  transition:fly={{ y: -200, duration: 200 }}
  class="toastContainer {status}"
>
  <div class="toastIcoContainer"><i class="log log-{$toast.status}"></i></div>
  <h4>{tittle}</h4>
  <p>{message}</p>

  <div class={additionButton?.isVisible ? "additionButtonContainer" : ""}>
    <div class="toastButtonContainer additionalButton">
      {#if additionButton?.isVisible}
        <Button
          id={`toast${additionButton.title}`}
          title={additionButton.title}
          minHeight={24}
          CB={() => {
            additionButton.CB();
          }}
        />
      {/if}
    </div>

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
</div>

<svelte:window
  onkeydown={(e) => {
    handleKeydown(e, "Escape", () => {
      toastIsVisible.set(false);
      if (toastTimeoutId) {
        clearTimeout($toastTimeoutId);
      }
    });
  }}
/>
