<script>
  import { navigate } from "svelte-routing";
</script>

<div class="textContainer">
  <h1 class="tittle">404 This is not the web page you are looking for...</h1>
  <div class="linksContainer">
    <p
      on:click={() => {
        navigate("/", { replace: true });
      }}
    >
      Move to home page
    </p>
    <p
      on:click={() => {
        navigate("/404", { replace: true });
      }}
    >
      Stay here and keep browsing this page
    </p>
  </div>
</div>
