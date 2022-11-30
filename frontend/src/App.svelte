<script>
    import Router from "svelte-spa-router";
    import Main from "./Views/Main/Main.svelte";
    import Login from "./Views/Login/Login.svelte";

    let routes = {
        "/*": Main,
        "/view": Main,
        "/login": Login,
    }
</script>

<Router {routes}/>