<script>
    import MainView from "./Views/Main/Main.svelte";
    import Router from "svelte-spa-router";
    import Main from "./Views/Main/Main.svelte";
    import Login from "./Views/Login/Login.svelte";

    let routes = {
        "/view": Main,
        "/login": Login,
    }
</script>

<Router {routes}/>