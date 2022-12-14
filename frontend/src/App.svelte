<script>
  import { Router, Link, Route } from "svelte-routing";
  import Main from "./Views/Main/Main.svelte";
  import Login from "./Views/Login/Login.svelte";
  import { theme } from "./Stores/stores.js";
  import { onMount } from "svelte";
  export let url = "";
  let themeState = "dark";
  theme.subscribe((v) => {
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
  });
</script>

<Router {url}>
  <div>
    <Route path="view/:host/:service" component={Main} let:params />
    <Route path="login" component={Login} />
    <Route path="users" component={Main} />

    <Route path="/"><Main /></Route>
  </div>
</Router>
