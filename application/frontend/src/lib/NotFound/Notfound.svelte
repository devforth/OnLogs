<script>
  import { navigate } from "svelte-routing";
  import { changeKey } from "../../utils/changeKey.js";
</script>

<div class="textContainer">
  <h1 class="tittle">404 This is not the web page you are looking for...</h1>
  <div class="linksContainer">
    <p
      onclick={() => {
        navigate(`${changeKey}/`, { replace: true });
      }}
    >
      Move to home page
    </p>
    <p
      onclick={() => {
        navigate(`${changeKey}/404`, { replace: true });
      }}
    >
      Stay here and keep browsing this page
    </p>
  </div>
</div>
