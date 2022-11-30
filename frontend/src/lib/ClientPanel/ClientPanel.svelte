<script>
    import { replace } from "svelte-spa-router";
    import fetchApi from "../../utils/fetch";

    let api = new fetchApi()
    async function logout() {
        await api.logout();
        replace("/login")
    }
</script>


<div class="clientPanel">
    <i class="log log-User" on:click={ async() => {await logout()}}/>
    <i class="log log-Wheel"/>
    <i class="log log-Moon"/>
</div>