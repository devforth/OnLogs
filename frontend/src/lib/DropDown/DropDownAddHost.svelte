<script>
  import DropDownRow from "./dropDownRow.svelte";
  import {
    snipetModalIsVisible,
    addHostMenuIsVisible,
  } from "../../Stores/stores.js";
</script>

<div class="dropDownContainer addHost " id="addHost">
  <table class="dropDownTable">
    <thbody>
      <DropDownRow
        rowTitle={"Create services group"}
        rowImage={"log log-Group"}
        storeValue={""}
        isFirst={true}
        disableCheckbox={true}
        titleCallBack={() => {}}
      />
      <DropDownRow
        rowTitle={"Connect new host"}
        rowImage={"log log-Server"}
        storeValue={""}
        disableCheckbox={true}
        titleCallBack={() => {
          snipetModalIsVisible.set(true);
          addHostMenuIsVisible.set(false);
        }}
      />
    </thbody>
  </table>
</div>
