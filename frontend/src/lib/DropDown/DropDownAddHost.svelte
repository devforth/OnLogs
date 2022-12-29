<script>
  import DropDownRow from "./dropDownRow.svelte";
</script>

<div class="dropDownContainer addHost ">
  <table class="dropDownTable">
    <thbody>
      <DropDownRow
        rowTitle={"Create services group"}
        rowImage={"log log-Group"}
        storeValue={""}
        isFirst={true}
        disableCheckbox={true}
        titleCallBack={() => {
          console.log("click");
        }}
      />
      <DropDownRow
        rowTitle={"Connect new host"}
        rowImage={"log log-Server"}
        storeValue={""}
        disableCheckbox={true}
        titleCallBack={() => {
          console.log("click");
        }}
      />
    </thbody>
  </table>
</div>
