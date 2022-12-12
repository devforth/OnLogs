<script>
  import Button from "../Button/Button.svelte";
</script>

<div>
  <h2>UserMenu</h2>
  <div>
    <h3>Users:</h3>
    <table class="userTable">
      <thead>
        <th scope="row">User</th><th>Role</th><th>Manage user</th>
      </thead>

      <tbody>
        <tr
          ><td><span>1</span></td><td><span>2</span></td><td
            ><span
              ><span class="buttonSpan"
                ><Button title={"Remove"} minWidth={86} /></span
              >
              <span class="buttonSpan"
                ><Button title={"Edit"} minWidth={86} /></span
              ></span
            ></td
          ></tr
        >
      </tbody>
    </table>
  </div>
</div>
