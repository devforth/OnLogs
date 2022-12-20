<script>
  import DropDownRow from "./dropDownRow.svelte";
</script>

<div class="dropDownContainer ">
  <table class="dropDownTable">
    <thbody>
      <DropDownRow
        rowTitle={"Break long lines"}
        rowImage={"log log-Break"}
        storeValue={"breakLines"}
        isFirst={true}
      />
      <DropDownRow
        rowTitle={"Log time in UTC"}
        rowImage={"log log-Clock"}
        storeValue={"UTCtime"}
      />
      <DropDownRow
        rowTitle={"Case insensitive search"}
        rowImage={"log log-Leters"}
        storeValue={"caseInSensitive"}
      />
    </thbody>
  </table>
</div>
