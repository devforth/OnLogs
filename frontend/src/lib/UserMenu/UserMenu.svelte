<script>
  import Button from "../Button/Button.svelte";
</script>

<div>
  <h2>UserMenu</h2>
  <div>
    <h3>Users:</h3>
    <table class="userTable" role="list">
      <thead>
        <th scope="row">User</th><th>Role</th><th>Manage user</th>
      </thead>

      <tbody>
        <tr
          ><td><span>1</span></td><td><span>2</span></td><td
            ><span class="buttonSpanContainer"
              ><span class="buttonSpan"
                ><Button title={"Remove"} minWidth={86} highlighted /></span
              >
              <span class="buttonSpan"
                ><Button title={"Edit"} minWidth={86} highlighted /></span
              ></span
            ></td
          ></tr
        >
      </tbody>
    </table>
  </div>
</div>
