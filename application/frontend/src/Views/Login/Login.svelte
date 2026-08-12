<script>
  // @ts-nocheck
  import fetchApi from "../../utils/fetch";
  import Container from "../../lib/Container/Container.svelte";
  import Button from "../../lib/Button/Button.svelte";
  import { navigate } from "svelte-routing";
  import { changeKey } from "../../utils/changeKey";

  let api = new fetchApi();
  let result = true;
  let wrong = $state(""),
    message = $state("");
  let cookies = document.cookie.split(";");

  for (const cookie of cookies) {
    let c = cookie.trim();
    if (c.startsWith("onlogs-cookie=")) {
      (async () => {
        if ((await api.checkCookie())["error"] == null) {
          navigate(`${changeKey}/`, { replace: true });
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
    } else {
      navigate(`${changeKey}/`, { replace: true });
    }
  }

  async function handleKeydown(event) {
    if (event.key === "Enter") {
      await confirm();
    }
  }
</script>

<div class="login loginContainer">
  <div class="loginForm">
    <h1 id="title">onLogs</h1>
    <form action="">
      <input
        id="login"
        name="login"
        class={wrong}
        placeholder="login"
        onclick={() => {
          wrong = "";
        }}
      />
      <input
        type="password"
        name="password"
        class={wrong}
        id="password"
        placeholder="password"
        onclick={() => {
          wrong = "";
        }}
      />
    </form>
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
</div>
<svelte:window onkeydown={handleKeydown} />
