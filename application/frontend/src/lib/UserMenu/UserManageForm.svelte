<script>
  import Button from "../Button/Button.svelte";

  let { userData = $bindable({ login: "", password: "" }), createHandler = () => {}, closeModal = () => {} } = $props();
</script>

<div class="userCreateForm">
  <h3>Create user</h3>
  <div class="inputContainer">
    <label for="username">Username</label>
    <input
      name="username"
      type="text"
      placeholder="User1"
      class="input"
      bind:value={userData.login}
    />
  </div>
  <div class="inputContainer">
    <label for="userpassword">Password</label>
    <input
      name="userpassword"
      type="password"
      placeholder="UserPassword"
      autocomplete="new-password"
      class="input"
      bind:value={userData.password}
    />
  </div>
  <div class="buttonsContainer">
    <div>
      <Button
        title={"Create"}
        minWidth={86}
        highlighted
        CB={() => {
          createHandler();
        }}
      />
    </div>
    <div>
      <Button title={"Cancel"} minWidth={86} highlighted CB={closeModal} />
      <div></div>
    </div>
  </div>
</div>
