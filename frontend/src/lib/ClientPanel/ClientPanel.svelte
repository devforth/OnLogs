<script>
  import fetchApi from "../../utils/fetch";
  import { navigate } from "svelte-routing";
  import { userMenuOpen, theme } from "../../Stores/stores.js";
  import { onDestroy } from "svelte";

  let localTheme = "";
  let api = new fetchApi();
  async function logout() {
    await api.logout();
    navigate("/login", { replace: true });
  }
  //store management
  function toggleUserMenu() {
    userMenuOpen.update((v) => !v);

    navigate("/users", { replace: true });
  }
  function toggleTheme() {
    theme.update((v) => {
      if (v === "light") {
        window.localStorage.setItem("theme", "dark");
        return "dark";
      } else {
        window.localStorage.setItem("theme", "light");
        return "light";
      }
    });
  }

  const unsubscribe = theme.subscribe((v) => {
    localTheme = v;
  });
  onDestroy(unsubscribe);
</script>

<div class="clientPanel">
  <ul class="clientPanelOptionsList">
    <li>
      <i class="log log-User" on:click={toggleUserMenu} />
    </li>
    <li>
      <i class="log log-Wheel" />
    </li>
    <li>
      <i
        class="log log-{localTheme === 'dark' ? 'Sun' : 'Moon'}"
        on:click={toggleTheme}
      />
    </li>
  </ul>
  <!-- <i class="log log-Wheel"/>
    <i class="log log-Moon"/> -->
</div>
