<script>
  import DropDownRow from "./dropDownRow.svelte";
  import {
    snipetModalIsVisible,
    addHostMenuIsVisible,
    groupModalIsVisible,
    groupBeingEdited,
  } from "../../Stores/stores.js";
</script>

<div class="dropDownContainer addHost " id="addHost">
  <table class="dropDownTable">
    <tbody>
      <DropDownRow
        rowTitle={"Create services group"}
        rowImage={"log log-Group"}
        storeValue={""}
        isFirst={true}
        disableCheckbox={true}
        titleCallBack={() => {
          groupBeingEdited.set(null);
          groupModalIsVisible.set(true);
          addHostMenuIsVisible.set(false);
        }}
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
    </tbody>
  </table>
</div>
