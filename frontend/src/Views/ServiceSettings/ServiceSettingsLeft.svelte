<script>
  import CommonList from "../../lib/CommonList/CommonList.svelte";
  import { lastChosenSetting } from "../../Stores/stores.js";
</script>

<CommonList
  listData={[{ name: "General " }, { name: "Acess" }]}
  isRowClickable={true}
  storeProp={lastChosenSetting}
/>
