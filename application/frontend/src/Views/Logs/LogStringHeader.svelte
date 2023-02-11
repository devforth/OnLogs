<script>
  import Button from "../../lib/Button/Button.svelte";
  import { handleKeydown } from "../../utils/functions.js";
  import {
    chosenLogsString,
    toast,
    toastIsVisible,
    toastTimeoutId,
  } from "../../Stores/stores.js";
  import { copyCustomText } from "../../utils/functions.js";
</script>

<div class="logStringHeaderContainer">
  <div title="Share link">
    <Button
      icon={"log log-Share"}
      iconHeight={16}
      CB={() => {
        const copiedUrl = `${location.href}#${$chosenLogsString}`;
        copyCustomText(copiedUrl, () => {
          toast.set({
            tittle: "Success",
            message: "Url has been copied",
            position: "",
            status: "Success",
          });
          toastIsVisible.set(true);
          toastTimeoutId.set(
            setTimeout(() => {
              toastIsVisible.set(false);
            }, 3000)
          );
        });
      }}
      minHeight={40}
      minWidth={40}
    />
  </div>
</div>

<svelte:window
  on:keydown={(e) => {
    handleKeydown(e, "Escape", () => {
      chosenLogsString.set("");
    });
  }}
/>
