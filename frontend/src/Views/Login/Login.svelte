<script>
// @ts-nocheck
    import fetchApi from "../../utils/fetch";
    import Container from "../../lib/Container/Container.svelte";
    import Button from "../../lib/Button/Button.svelte";

    let api = new fetchApi();
    let result = true;
    let wrong = "", message = "";
    async function confirm() {
        const login = document.getElementById("login").value
        const password = document.getElementById("password").value
        result = await api.login(login.trim(), password.trim())
        if (!result) {
            wrong = "wrong"
            message = "Wrong password or login!"
        }
    }
</script>

<div class="login contentContainer">
    <Container minHeightVh={25}>
        <div class="loginForm">
            <h1 id="title">onLogs</h1>
            <input id="login" class="{wrong}" placeholder="login" on:click={ () => {wrong = ""} }>
            <input type="password" class="{wrong}" id="password" placeholder="password" on:click={ () => {wrong = ""} }>
            <div class="bottom">
                <p class="{wrong}">{message}</p>
                <div class="confirmButton">
                    <Button on:onClickButton={ async() => {await confirm()} } title="Login" highlighted/>
                </div>
            </div>
        </div>
    </Container>
</div>