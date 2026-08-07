<script>
  import fetchApi from "../../utils/fetch";
  import { navigate } from "svelte-routing";
  import {
    userMenuOpen,
    theme,
    activeMenuOption,
    lastChosenHost,
    lastChosenService,
  } from "../../Stores/stores.js";
  import { onDestroy } from "svelte";
  import { changeKey } from "../../utils/changeKey.js";

  let localTheme = $state("");
  let api = new fetchApi();

  const showUserMenu = !(window.DISABLE_AUTH ?? false);

  //store management
  function toggleUserMenu() {
    userMenuOpen.update((v) => !v);
    activeMenuOption.set("users");

    navigate(`${changeKey}/users`, { replace: true });
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
  function goToHome() {
    navigate(`${changeKey}/view/${$lastChosenHost}/${$lastChosenService}`, {
      replace: true,
    });
    activeMenuOption.set("home");
  }

  const unsubscribe = theme.subscribe((v) => {
    localTheme = v;
  });
  onDestroy(unsubscribe);
</script>

<div class="clientPanel">
  <ul class="clientPanelOptionsList">
    <li
      onclick={() => {
        if ($activeMenuOption === "burger") {
          activeMenuOption.set(location.pathname.split("/")[1] || "home");
        } else {
          activeMenuOption.set("burger");
        }
      }}
      class="{$activeMenuOption === 'burger' && 'active'} burger"
    >
      <i class="log log-Burger "></i>
      <div
        class="higlightedOverlay {$activeMenuOption === 'burger' && 'active'}"
></div>
    </li>
    <li onclick={goToHome} class={$activeMenuOption === "home" && "active"}>
      <i class="log log-Home "></i>
      <div
        class="higlightedOverlay {($activeMenuOption === 'home' && 'active') ||
          ($activeMenuOption === 'view' && 'active')}"
></div>
    </li>
    {#if showUserMenu}
    <li
      onclick={toggleUserMenu}
      class={$activeMenuOption === "users" && "active"}
    >
      <i class="log log-User"></i>
      <div
        class="higlightedOverlay {$activeMenuOption === 'users' && 'active'}"
></div>
    </li>
    {/if}

    <!-- <li class={$activeMenuOption === "wheel" && "active"}>
      <i class="log log-Wheel" />
    </li> -->
    <li onclick={toggleTheme}>
      <i class="log log-{localTheme === 'dark' ? 'Sun' : 'Moon'}"></i>
    </li>
  </ul>
</div>
