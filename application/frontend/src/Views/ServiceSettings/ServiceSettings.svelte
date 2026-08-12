<script>
  import General from "./General.svelte";
  import { lastChosenSetting } from "../../Stores/stores.js";
</script>

<div class="serviceSettings">
  {#if $lastChosenSetting.trim() === "General"}
    <General />
  {/if}
</div>
