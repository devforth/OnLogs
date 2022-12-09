<script>
  import DropDownRow from "./dropDownRow.svelte";
  export let DropDownIsvisible = false;
</script>

<div class="dropDownContainer {!DropDownIsvisible ? 'visuallyHidden' : ''}">
  <table class="dropDownTable">
    <DropDownRow
      rowTitle={"Break long lines"}
      rowImage={"log log-Break"}
      storeValue={"breakLines"}
    />
    <DropDownRow
      rowTitle={"Log time in UTC"}
      rowImage={"log log-Clock"}
      storeValue={"UTCtime"}
      isLast={true}
    />
  </table>
</div>
