<script>
  import Button from "../Button/Button.svelte";

  // Resolves to an error message, or "" on success.
  let { createHandler = async () => "", closeModal = () => {} } = $props();
  let userData = $state({ login: "", password: "" });
  let error = $state("");

  async function submit() {
    error = await createHandler(userData);
  }

  function submitOnEnter(e) {
    if (e.key === "Enter") {
      submit();
    }
  }
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
      onkeydown={submitOnEnter}
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
      onkeydown={submitOnEnter}
    />
  </div>
  {#if error}<p class="error">{error}</p>{/if}
  <div class="buttonsContainer">
    <div>
      <Button title={"Create"} minWidth={86} highlighted CB={submit} />
    </div>
    <div>
      <Button title={"Cancel"} minWidth={86} highlighted CB={closeModal} />
      <div></div>
    </div>
  </div>
</div>
