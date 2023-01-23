<script>
  import { Router, Link, Route } from "svelte-routing";
  import Main from "./Views/Main/Main.svelte";
  import Login from "./Views/Login/Login.svelte";
  import { theme, activeMenuOption } from "./Stores/stores.js";
  import { onMount, onDestroy } from "svelte";
  import Toast from "./lib/Toast/Toast.svelte";
  import Notfound from "./lib/NotFound/Notfound.svelte";
  export let url = "";
  let themeState = "dark";
  let basePathname = "";
  let availibleRoutes = ["view", "login", "users", "servicesettings"];
  import { changeKey } from "./utils/changeKey";

  const ASSET_URL = import.meta.env.VITE_ASSET_URL;

  const unsubscribe = theme.subscribe((v) => {
    themeState = v;
  });
  $: themeState && checkTheme(themeState);

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

    //set active menu option after page refresh
    if (location.pathname.includes("users")) {
      activeMenuOption.set("users");
    } else {
      activeMenuOption.set("home");
    }
    if (!availibleRoutes.includes(location.pathname.split("/")?.at(1))) {
      basePathname = location.pathname.split("/")?.at(1);
    }
  });
  onDestroy(unsubscribe);
  console.log(location.pathname.split("/")[1], "location");
</script>

<Router {url} basepath={`${changeKey}/`}>
  <div>
    <Route path="/view/:host/:service" component={Main} />
    <Route path={"/login"} component={Login} />
    <Route path="/users" component={Main} />
    <Route path="/servicesettings/:host/:service" component={Main} />
    <Route component={Notfound} />

    <Route path={`/`}><Main /></Route>
    <Route path={`/path`}><Main /></Route>
  </div>
  <Toast />
</Router>
