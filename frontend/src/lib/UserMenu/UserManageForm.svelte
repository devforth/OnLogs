<script>
  import Button from "../Button/Button.svelte";
</script>

<div class="userCreateForm">
  <h3>Create user</h3>
  <div class="inputContainer">
    <label for="username">Username</label>
    <input name="username" type="text" placeholder="User1" class="input" />
  </div>
  <div class="inputContainer">
    <label for="userpassword">Password</label>
    <input
      name="userpassword"
      type="password"
      placeholder="UserPassword"
      class="input"
    />
  </div>
  <div class="buttonsContainer">
    <div><Button title={"Save"} minWidth={86} highlighted /></div>
    <div>
      <Button title={"Cancel"} minWidth={86} highlighted />
      <div />
    </div>
  </div>
</div>
