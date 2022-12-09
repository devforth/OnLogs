<script>
  // @ts-nocheck
  import fetchApi from "../../utils/fetch";
  import Container from "../../lib/Container/Container.svelte";
  import Button from "../../lib/Button/Button.svelte";
  import { replace } from "svelte-spa-router";

  let api = new fetchApi();
  let result = true;
  let wrong = "",
    message = "";
  let cookies = document.cookie.split(";");

  for (const cookie of cookies) {
    let c = cookie.trim();
    if (c.startsWith("onlogs-cookie=")) {
      (async () => {
        if ((await api.checkCookie())["error"] == null) {
          replace("/view");
        }
      })();
    }
  }

  async function confirm() {
    const login = document.getElementById("login").value;
    const password = document.getElementById("password").value;
    result = await api.login(login.trim(), password.trim());
    if (!result) {
      wrong = "wrong";
      message = "Wrong password or login!";
    }
  }

  async function handleKeydown(event) {
    if (event.key === "Enter") {
      await confirm();
    }
  }
</script>

<div class="login contentContainer">
  <Container minHeightVh={25}>
    <div class="loginForm">
      <h1 id="title">onLogs</h1>
      <input
        id="login"
        class={wrong}
        placeholder="login"
        on:click={() => {
          wrong = "";
        }}
      />
      <input
        type="password"
        class={wrong}
        id="password"
        placeholder="password"
        on:click={() => {
          wrong = "";
        }}
      />
      <div class="bottom">
        <p class={wrong}>{message}</p>
        <div class="confirmButton">
          <Button
            CB={async () => {
              await confirm();
            }}
            title="Login"
            highlighted
          />
        </div>
      </div>
    </div>
  </Container>
</div>
<svelte:window on:keydown={handleKeydown} />
