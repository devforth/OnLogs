<script>
  // @ts-ignore
  import Container from "@/lib/Container/Container.svelte";
  import HostList from "../../lib/HostList/HostList.svelte";
  import LogsView from "../Logs/LogsView.svelte";
  import Button from "../../lib/Button/Button.svelte";
  import fetchApi from "../../utils/fetch";
  import ClientPanel from "../../lib/ClientPanel/ClientPanel.svelte";
  import { userMenuOpen, addUserModalOpen } from "@/Stores/stores.js";
  import UserMenu from "@/lib/UserMenu/UserMenu.svelte";
  import Modal from "../../lib/Modal/Modal.svelte";
  import UserManageForm from "../../lib/UserMenu/UserManageForm.svelte";
  import { navigate } from "svelte-routing";

  const listMargins = { marginTop: "6.68vh" };
  let api = new fetchApi();
  let userMenuState = false;
  let addUserModOpen = false;
  let newUserData = { login: "", password: "" };
  let userForAdding = "";

  function closeModal() {
    addUserModalOpen.set(false);
  }

  async function createUser() {
    if (newUserData.login && newUserData.password) {
      const data = await api.createUser(newUserData);
      if (!data.error) {
        console.log("added");
        userForAdding = newUserData.login;
      }
    }
  }

  userMenuOpen.subscribe((v) => {
    userMenuState = v;
  });
  addUserModalOpen.subscribe((v) => {
    addUserModOpen = v;
  });

  $: selectedService = "";
  async function getHosts() {
    let hostList = [await api.getHosts()]; // TODO remove [] when backend will be able to send array of hosts
    selectedService = hostList[0]["services"][0]; // TODO pick the last choosen service
    return hostList;
  }
</script>

<div class="contentContainer">
  <div class="subContainerLeft subContainer">
    <Container highlighted minHeightVh={79.3}>
      <div class="onLogsPanel">
        <div class="onLogsPanelHeader">
          <h1
            on:click={() => {
              navigate("/", { replace: true });
            }}
          >
            onLogs
          </h1>
          <Button
            title=""
            border={false}
            highlighted
            minWidth={0}
            minHeight={0}
          />
          <!-- icon="log log-Plus"
          iconHeight={18} -->
        </div>
        {#await getHosts()}
          <p>loading...</p>
        {:then hosts}
          {#each hosts as host}
            <HostList
              bind:selectedName={selectedService}
              hostName={host["host"]}
              servicesData={host["services"]}
              {...listMargins}
            />
          {/each}
        {:catch}
          <p style="margin-top: 15px;">Error</p>
        {/await}
      </div></Container
    >
    <Container minHeightVh={10.97}>
      <ClientPanel />
    </Container>
  </div>
  <div class="subContainerMiddle subContainer">
    <!-- <Container minHeightVh={17.36}>1213414</Container> -->
    <Container minHeightVh={92.6}>
      {#if location.pathname === "/users"}
        <UserMenu {userForAdding} />
        <Modal modalIsOpen={addUserModOpen} storeProp={addUserModalOpen}
          ><UserManageForm
            bind:userData={newUserData}
            createHandler={createUser}
            {closeModal}
          /></Modal
        >
      {:else}<LogsView bind:serviceName={selectedService} />
      {/if}
    </Container>
  </div>
  <!-- <div class="subContainerRight  subContainer">
    <Container minHeightVh={24.44}>1213414</Container>
    <Container minHeightVh={21.52}>1213414</Container>
    <div class="subContainerThumb">
      <Container minHeightVh={17.36}>1213414</Container>
      <Container minHeightVh={12.91} highlighted>1213414</Container>
    </div>
  </div> -->
</div>
