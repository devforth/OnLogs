<script>
  import { Moon, Clock } from "svelte-loading-spinners";
  import { theme } from "../../Stores/stores";
  let spinnerColor = "";
  $: if ($theme === "light") {
    spinnerColor = "#4e49da";
  } else {
    spinnerColor = "#c244db";
  }
</script>

<div style="width:100%;height:100%" class="flex">
  <Moon size="60" color={spinnerColor} duration="1s" unit="px" />
</div>
