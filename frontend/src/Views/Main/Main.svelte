<script>
  // @ts-ignore
  import Container from "@/lib/Container/Container.svelte";
  import HostList from "../../lib/HostList/HostList.svelte";
  import LogsView from "../Logs/LogsView.svelte";
  import Button from "../../lib/Button/Button.svelte";
  import fetchApi from "../../utils/fetch";
  import ClientPanel from "../../lib/ClientPanel/ClientPanel.svelte";
  import {
    userMenuOpen,
    addUserModalOpen,
    activeMenuOption,
  } from "@/Stores/stores.js";
  import UserMenu from "@/lib/UserMenu/UserMenu.svelte";
  import Modal from "../../lib/Modal/Modal.svelte";
  import UserManageForm from "../../lib/UserMenu/UserManageForm.svelte";
  import { navigate } from "svelte-routing";
  import { onMount, onDestroy } from "svelte";
  import {
    lastChosenHost,
    lastChosenService,
    theme,
  } from "../../Stores/stores";
  import ListWithChoise from "../../lib/ListWithChoise/ListWithChoise.svelte";
  import CommonList from "../../lib/CommonList/CommonList.svelte";

  let api = new fetchApi();
  let hostList = [];
  let servicesList = [];

  let userMenuState = false;
  let addUserModOpen = false;
  let newUserData = { login: "", password: "" };
  let userForAdding = "";
  export let host = "";
  export let service = "";

  function closeModal() {
    addUserModalOpen.set(false);
  }

  async function createUser() {
    if (newUserData.login && newUserData.password) {
      const data = await api.createUser(newUserData);
      if (!data.error) {
        userForAdding = newUserData.login;
      }
    }
  }

  async function logout() {
    await api.logout();
    navigate("/login", { replace: true });
  }

  userMenuOpen.subscribe((v) => {
    userMenuState = v;
  });
  addUserModalOpen.subscribe((v) => {
    addUserModOpen = v;
  });

  async function getHosts() {
    const data = await api.getHosts();

    if (Array.isArray(data) && data.at(0)) {
      hostList = [...data];
    }
    if (data.host) {
      hostList = [data];
    }
  }
  onMount(async () => {
    await getHosts();
    if (service) {
      lastChosenService.set(service);
    } else {
      lastChosenService.set(hostList.at(0)["services"].at(0));
    }
    if (host) {
      lastChosenHost.set(host);
    } else {
      lastChosenHost.set(hostList.at(0)["host"]);
    }
    // @ts-ignore}

    servicesList = hostList
      .filter((e) => {
        return e.host === $lastChosenHost;
      })
      .at(0)["services"];
  });
</script>

<div class="contentContainer">
  <div class="subContainerLeft subContainer">
    <Container highlighted={$theme !== "dark"} minHeightVh={79.3}>
      <div class="onLogsPanel">
        <div class="onLogsPanelHeader">
          <h1
            on:click={() => {
              navigate(`/view/${$lastChosenHost}/${$lastChosenService}`, {
                replace: true,
              });
              activeMenuOption.set("home");
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
        {#if location.pathname.includes("/view") || location.pathname === "/"}
          <ListWithChoise
            listData={hostList}
            headerButton={"Pencil"}
            listElementButton={"true"}
            activeElementName={host && service && service !== "undefined"
              ? `${host}-${service}`
              : ""}
          />{:else if location.pathname.includes("/users")}<CommonList
            listData={[{ name: "Logout", ico: "Logout", callBack: logout }]}
          />{/if}
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
      {:else}<LogsView />
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
