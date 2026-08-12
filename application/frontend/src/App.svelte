<script>
  import { Router, Link, Route } from "svelte-routing";
  import Main from "./Views/Main/Main.svelte";
  import Login from "./Views/Login/Login.svelte";
  import {
    theme,
    activeMenuOption,
    toastIsVisible,
    lastChosenHost,
    lastChosenService,
    urlHash,
  } from "./Stores/stores.js";
  import { onMount } from "svelte";
  import Toast from "./lib/Toast/Toast.svelte";
  import Notfound from "./lib/NotFound/Notfound.svelte";
  import { changeKey } from "./utils/changeKey";
  import { navigate } from "svelte-routing";
  let { url = "" } = $props();

  function checkTheme(t) {
    const bodyEl = document.querySelector("body");
    if (t === "dark") {
      if (!bodyEl.classList.contains("dark-mode")) {
        bodyEl.classList.add("dark-mode");
      }
    } else {
      bodyEl.classList.remove("dark-mode");
    }
  }

  onMount(() => {
    const LStheme = window.localStorage.getItem("theme");
    if (LStheme) {
      theme.set(LStheme);
    }
    const routeNamesContainer =
      /\/(view|stats|servicesettings)\/[^/]+\/[^/]+/.test(location.pathname);
    const LSHostData = window.localStorage.getItem("lsthd");
    if (LSHostData && !routeNamesContainer) {
      try {
        const data = JSON.parse(LSHostData);
        lastChosenHost.set(data.h);
        lastChosenService.set(data.s);
      } catch (e) {}
    }
    if (location.href.includes("#")) {
      urlHash.set(location.hash);

      navigate(location.href.split("#")[0], { replace: true });
    }

    //set active menu option after page refresh
    if (location.pathname.includes("users")) {
      activeMenuOption.set("users");
    } else {
      activeMenuOption.set("home");
    }
  });

  function writeLastChosenHostToLS() {
    window.localStorage.setItem(
      "lsthd",
      JSON.stringify({ h: $lastChosenHost, s: $lastChosenService })
    );
  }

  $effect(() => {
    checkTheme($theme);
  });

  $effect(() => {
    if ($lastChosenHost && $lastChosenService) {
      writeLastChosenHostToLS();
    }
  });
</script>

<Router {url} basepath={`${changeKey}/`}>
  <div>
    <!-- Snippet form, not component={}: svelte-routing detects Svelte 4 class
         components with `startsWith("class ")`, and Svelte 5 compiles to
         functions, so it calls them instead of rendering them. -->
    <Route path="/view/:host/:service">
      {#snippet children({ params })}
        <Main host={params.host} service={params.service} />
      {/snippet}
    </Route>
    <Route path={"/login"}><Login /></Route>
    <Route path="/users"><Main /></Route>
    <Route path="/servicesettings/:host/:service">
      {#snippet children({ params })}
        <Main host={params.host} service={params.service} />
      {/snippet}
    </Route>
    <Route path="/stats/:host/:service">
      {#snippet children({ params })}
        <Main host={params.host} service={params.service} />
      {/snippet}
    </Route>
    <Route><Notfound /></Route>

    <Route path={`/`}><Main /></Route>
  </div>
  {#if $toastIsVisible} <Toast />{/if}
</Router>
