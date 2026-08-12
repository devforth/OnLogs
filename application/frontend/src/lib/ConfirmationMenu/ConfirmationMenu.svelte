<script>
  import { untrack } from "svelte";
  import Button from "../Button/Button.svelte";
  import {
    confirmationObj,
    store,
    lastChosenHost,
    lastChosenService,
    toast,
    toastIsVisible,
    toastTimeoutId,
  } from "../../Stores/stores.js";
  import Input from "../Input/Input.svelte";
  import Checkbox from "../CheckBox/Checkbox.svelte";
  import fetchApi from "../../utils/fetch.js";

  let confirmationWord = "I understand that logs will be lost";
  let tipsIsVisible = $state(false);
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

  // Only the clear-logs flow owns the message; a caller-supplied action
  // brings its own. changeMessage writes confirmationObj, so the read of it
  // stays untracked or the effect would retrigger itself forever.
  $effect(() => {
    const deleteFromDocker = $store.deleteFromDocker;
    untrack(() => {
      if (!$confirmationObj.action) {
        changeMessage(deleteFromDocker);
      }
    });
  });

  async function confirm() {
    if (typeof $confirmationObj.action === "function") {
      await $confirmationObj.action();
      return;
    }
    await deletelogs();
  }
</script>

<div class="deleteModalContainer">
  <div class="tipsContainer">
    <i
      class="log log-Tips"
      onmouseenter={() => {
        tipsIsVisible = true;
      }}
      onmouseleave={() => {
        tipsIsVisible = false;
      }}
></i>
    {#if tipsIsVisible}
      <div class="tipsText container">
        <span class="boldText">Delete Docker logs </span> - when the option is set to
        <span class="boldText">"OFF" </span>
        logs will be deleted only from onLogs. Logs will be available in docker containers, but not for onLogs.
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
    <p>
      {`Host: ${
        $lastChosenHost ? $lastChosenHost : "host"
      }`}
    </p>
    <p>
        {`Service: ${
          $lastChosenService ? $lastChosenService : "service"
        }, from: `}

        <span style="font-weight: bold;margin-top: 12px;"
          >{$confirmationObj.message}</span
        >
      </p>
    <p style="margin-top: 12px">This data will be lost. This action cannot be undone.</p>
  </div>
  <div class="buttonsBox">
    <Button
      title={"Delete"}
      highlighted={true}
      CB={() => {
        confirm();
      }}
    />
    <Button
      title={"Cancel"}
      CB={() => {
        confirmationObj.update((pv) => {
          return { ...pv, isVisible: false };
        });
      }}
    />
  </div>
</div>
<div class="modalOverlay" onclick={closeMenu}></div>

<svelte:window
  onkeydown={({ key }) => {
    key === "Escape" && closeMenu();

  }}
/>
