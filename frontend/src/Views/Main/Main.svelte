<script>
  // @ts-ignore
  import Container from "@/lib/Container/Container.svelte";
  import HostList from "../../lib/HostList/HostList.svelte";
  import NewLogsView from "../Logs/NewLogsView.svelte";
  import Button from "../../lib/Button/Button.svelte";
  import fetchApi from "../../utils/fetch";
  import ClientPanel from "../../lib/ClientPanel/ClientPanel.svelte";
  import {
    userMenuOpen,
    addUserModalOpen,
    activeMenuOption,
    lastChosenHost,
    lastChosenService,
    theme,
    snipetModalIsVisible,
    addHostMenuIsVisible,
    listScrollIsVisible,
    confirmationObj,
  } from "../../Stores/stores.js";
  import UserMenu from "../../lib/UserMenu/UserMenu.svelte";
  import Modal from "../../lib/Modal/Modal.svelte";
  import UserManageForm from "../../lib/UserMenu/UserManageForm.svelte";
  import { navigate } from "svelte-routing";
  import { onMount, onDestroy } from "svelte";

  import ListWithChoise from "../../lib/ListWithChoise/ListWithChoise.svelte";
  import CommonList from "../../lib/CommonList/CommonList.svelte";
  import { clickOutside } from "../../lib/OutsideClicker/OutsideClicker.js";
  import DropDownAddHost from "../../lib/DropDown/DropDownAddHost.svelte";
  import SecretModal from "../../lib/SecretModal/SecretModal.svelte";
  import LogsSize from "../../lib/LogsSize/LogsSize.svelte";
  import ConfirmationMenu from "../../lib/ConfirmationMenu/ConfirmationMenu.svelte";
  import ServiceSettings from "../ServiceSettings/ServiceSettings.svelte";
  import ServiceSettingsLeft from "../ServiceSettings/ServiceSettingsLeft.svelte";
  import { lastLogTimestamp } from "../../Stores/stores.js";

  let api = new fetchApi();
  let hostList = [];
  let servicesList = [];

  let userMenuState = false;
  let addUserModOpen = false;
  let newUserData = { login: "", password: "" };
  let userForAdding = "";
  let withoutRightPanel = false;
  let withoutRightPanelRoutesArr = ["users", "servicesettings"];
  function handleClick() {
    lastLogTimestamp.set(new Date("12.02.2122").getTime());
  }

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
      lastChosenService.set(hostList.at(0)["services"].at(0).serviceName);
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

  $: {
    if (withoutRightPanelRoutesArr.includes(location.pathname.split("/")[1])) {
      withoutRightPanel = true;
    }
  }
</script>

<div class="contentContainer">
  <div class="subContainerLeft subContainer ">
    <div
      class={$activeMenuOption === "burger" &&
        !location.pathname.includes("/servicesettings") &&
        "active"}
      id="listContainer"
      on:mouseenter={() => {
        listScrollIsVisible.set(true);
      }}
      on:mouseleave={() => {
        listScrollIsVisible.set(false);
      }}
    >
      <Container highlighted={$theme !== "dark"} paddingOff={true}>
        <div class="onLogsPanel">
          <div class="onLogsPanelHeader">
            <h1
              on:click={() => {
                navigate(
                  `/view/${$lastChosenHost}/${$lastChosenService}/offset=0/searchText=0`,
                  {
                    replace: true,
                  }
                );
                activeMenuOption.set("home");
              }}
            >
              onLogs
            </h1>
            <div
              style:position={"relative"}
              use:clickOutside
              on:click_outside={() => {
                addHostMenuIsVisible.set(false);
              }}
              class={withoutRightPanel && "visuallyHidden"}
            >
              <Button
                title=""
                border={false}
                highlighted
                minWidth={0}
                minHeight={0}
                icon="log log-Plus"
                iconHeight={18}
                CB={() => {
                  addHostMenuIsVisible.update((v) => !v);
                }}
              />
              {#if $addHostMenuIsVisible}
                <DropDownAddHost />{/if}
            </div>
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
          {#if location.pathname.includes("/servicesettings")}
            <ServiceSettingsLeft />{/if}
        </div>
      </Container>
    </div>
    <div class="clientPanelBox">
      <Container>
        <ClientPanel />
      </Container>
    </div>
  </div>
  <div
    class="subContainerMiddle subContainer {withoutRightPanel &&
      'withoutRightPanel'}"
  >
    <!-- <Container minHeightVh={17.36}>1213414</Container> -->
    <Container minHeightVh={92.6}>
      {#if location.pathname === "/users"}
        <UserMenu {userForAdding} />
      {:else if location.pathname.includes("/view") || location.pathname === "/"}<NewLogsView
        />
      {/if}
      {#if location.pathname.includes("/servicesettings")}
        <ServiceSettings />{/if}
    </Container>
  </div>
  {#if $snipetModalIsVisible}
    <SecretModal />
  {/if}
  <div
    class="subContainerRight  subContainer {withoutRightPanel
      ? 'visuallyHidden'
      : ''}"
  >
    <div class="logSizeInformation">
      <Container minHeightVh={15} max-height={20}
        ><LogsSize
          discribeText={"Space used by all logs"}
          isAllLogs={true}
        /></Container
      >
    </div>
    <div class="logSizeInformation">
      <Container minHeightVh={15}
        ><LogsSize discribeText={"Space used by service"} /></Container
      >
    </div>
    <!-- <div class="subContainerThumb">
      <Container minHeightVh={17.36}>1213414</Container>
      <Container minHeightVh={12.91} highlighted>1213414</Container>
    </div> -->
  </div>
</div>
<Modal
  modalIsOpen={$confirmationObj.isVisible}
  closeFunction={() => {
    confirmationObj.update((pv) => {
      return { ...pv, isVisible: false };
    });
  }}
>
  <ConfirmationMenu /></Modal
>
<Modal modalIsOpen={addUserModOpen} storeProp={addUserModalOpen}
  ><UserManageForm
    bind:userData={newUserData}
    createHandler={createUser}
    {closeModal}
  /></Modal
>
<svelte:window on:click={handleClick} />
