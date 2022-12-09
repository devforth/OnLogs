<script>
  import { Router, Link, Route } from "svelte-routing";
  import Main from "./Views/Main/Main.svelte";
  import Login from "./Views/Login/Login.svelte";
  export let url = "";
</script>

<Router {url}>
  <div>
    <Route path="view" component={Main} />
    <Route path="login" component={Login} />

    <Route path="/"><Main /></Route>
  </div>
</Router>
