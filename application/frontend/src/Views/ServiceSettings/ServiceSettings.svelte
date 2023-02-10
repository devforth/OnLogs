<script>
  import General from "./General.svelte";
  import Acess from "./Acess.svelte";
  import { lastChosenSetting } from "../../Stores/stores.js";
</script>

<div class="serviceSettings">
  {#if $lastChosenSetting.trim() === "General"}
    <General />
  {/if}
  {#if $lastChosenSetting.trim() === "Acess"}
    <Acess />
  {/if}
</div>
