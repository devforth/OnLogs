<script>
  import fetchApi from "../../utils/fetch";
  import { navigate } from "svelte-routing";

  let api = new fetchApi();
  async function logout() {
    await api.logout();
    navigate("/login", { replace: true });
  }
</script>

<div class="clientPanel">
  <i
    class="log log-User"
    on:click={async () => {
      await logout();
      navigate("/login", { replace: true });
    }}
  />
  <!-- <i class="log log-Wheel"/>
    <i class="log log-Moon"/> -->
</div>
