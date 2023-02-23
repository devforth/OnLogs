<script>
  import Button from "../Button/Button.svelte";
  import {
    confirmationObj,
    store,
    lastChosenHost,
    lastChosenService,
    toast,
    toastIsVisible,
    toastTimeoutId,
    theme,
  } from "../../Stores/stores.js";
  import Input from "../Input/Input.svelte";
  import Checkbox from "../CheckBox/Checkbox.svelte";
  import fetchApi from "../../utils/fetch.js";

  let confirmationWord = "I understand that data will be lost";
  let tipsIsVisible = false;
  let inputValue = "";
  let error = false;
  function changeMessage(triger) {
    confirmationObj.set({
      ...$confirmationObj,
      message: `  
      ${triger ? "Docker" : "OnLogs"}. `,
    });
  }
  const apiFetch = new fetchApi();

  function closeMenu() {
    confirmationObj.set({ ...$confirmationObj, isVisible: false });
  }

  async function deletelogs() {
    let data = {};
    if ($store.deleteFromDocker) {
      data = await apiFetch.cleanDockerLogs(
        $lastChosenHost,
        $lastChosenService
      );
    } else {
      data = await apiFetch.cleanLogs($lastChosenHost, $lastChosenService);
    }
    if (!data.error) {
      closeMenu();
    } else {
      toast.set({
        tittle: "Error",
        message: "Something went wrong. Please try again later",
        position: "",
        status: "Error",
      });
      if (!$toastIsVisible) {
        toastIsVisible.set(true);
        toastTimeoutId.set(
          setTimeout(() => {
            toastIsVisible.set(false);
          }, 3000)
        );
      }
    }
  }

  $: {
    changeMessage($store.deleteFromDocker);
  }
</script>

<div class="deleteModalContainer">
  <div class="tipsContainer">
    <i
      class="log log-Tips"
      on:mouseenter={() => {
        tipsIsVisible = true;
      }}
      on:mouseleave={() => {
        tipsIsVisible = false;
      }}
    />
    {#if tipsIsVisible}
      <div class="tipsText container">
        <span class="boldText">Delete Docker logs </span> - when the option is
        <span class="boldText">disabled </span>
        you can only delete duplicates of logs, that onLogs uses to present logs
        to you. Logs will be available in docker containers, but not for onLogs.
        When
        <span class="boldText"> enabled </span>, each deletion of logs will
        clear logs from both onLogs and the
        <span class="boldText">Docker container</span>
        .
      </div>{/if}
  </div>

  <h3 class="deleteModalTitle">Delete logs options</h3>
  <div class="optionsBox">
    <div class="optionBox">
      <p>Delete Docker logs</p>
      <div class="checkboxContainerThumb">
        <Checkbox storeValue={"deleteFromDocker"} />
      </div>
    </div>
  </div>
  <div class="attentionZone">
    <p style="margin-bottom:12px;">
      {`You want to delete logs. Host: ${
        $lastChosenHost ? $lastChosenHost : "host"
      }  service:${
        $lastChosenService ? $lastChosenService : "service"
      }  from: `}

      <span style="font-weight: bold;margin-top: 12px;"
        >{$confirmationObj.message}</span
      >
    </p>
    <p>This data will be lost. This action cannot be undone.</p>
  </div>

  <div class="confirmationText">
    Please type:" <span class="boldText {error && 'error'}"
      >{confirmationWord}"</span
    > to confirm.
  </div>
  <div style="color:white">
    <Input
      placeholder={"Confirm string"}
      customClass={"editInput"}
      width={300}
      bind:value={inputValue}
    />
  </div>
  <div class="buttonsBox">
    <Button
      disabled={confirmationWord !== inputValue ? true : false}
      title={"Delete"}
      highlighted={true}
      CB={() => {
        if (confirmationWord === inputValue) {
          deletelogs();
        } else {
          error = true;
        }
      }}
    /><Button
      title={"Cancel"}
      CB={() => {
        confirmationObj.update((pv) => {
          return { ...pv, isVisible: false };
        });
      }}
    />
  </div>
</div>
<div class="modalOverlay" on:click={closeMenu} />

<svelte:window
  on:keydown={({ key }) => {
    key === "Escape" && closeMenu();
    
  }}
/>
