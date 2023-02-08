<script>
  import Button from "../Button/Button.svelte";
  import {
    snipetModalIsVisible,
    addHostMenuIsVisible,
    toast,
    toastIsVisible,
    currentSnippedOption,
  } from "../../Stores/stores";
  import { clickOutside } from "../../lib/OutsideClicker/OutsideClicker.js";
  import { onMount } from "svelte";
  import FetchApi from "../../utils/fetch.js";
  import { handleKeydown, copyText } from "../../utils/functions.js";
  import { changeKey } from "../../utils/changeKey";
  import DockerSnippet from "./DockerSnippet.svelte";
  import DockerComposeSnippet from "./DockerComposeSnippet.svelte";

  let token = "";
  let origin = `${location.origin}${changeKey}`;
  const api = new FetchApi();
  async function getSecret() {
    try {
      const data = await api.getSecret();
      if (data.token) {
        return data.token;
      }
    } catch {
      return "2345678901234567";
    }
  }

  function choseSnippetOption(opt = "") {
    currentSnippedOption.set(opt);
  }

  onMount(async () => {
    token = await getSecret();
  });
</script>

<div
  class="secretModalContainer"
  use:clickOutside
  on:click_outside={() => {
    snipetModalIsVisible.set(false);
  }}
>
  <h3 class="secretMoalTitle">Connect new host</h3>
  <div class="labelsBox flex">
    <div
      class={`labelItem clickable ${
        $currentSnippedOption === "Docker" ? "active" : ""
      }`}
      on:click={() => {
        choseSnippetOption("Docker");
      }}
    >
      Docker
    </div>
    <div
      class={`labelItem clickable ${
        $currentSnippedOption === "DockerCompose" ? "active" : ""
      }`}
      on:click={() => {
        choseSnippetOption("DockerCompose");
      }}
    >
      Docker Compose
    </div>
  </div>
  <div class={`snippetContainer `}>
    {#if $currentSnippedOption === "Docker"}
      <DockerSnippet {token} {origin} />
    {/if}
    {#if $currentSnippedOption === "DockerCompose"}
      <DockerComposeSnippet {token} {origin} />
    {/if}
    <div class="coppyButton">
      <Button
        icon={"log log-Copy"}
        minHeight={40}
        minWidth={55}
        border={true}
        CB={() => {
          copyText(document.querySelector("pre"), () => {
            toast.set({
              tittle: "Success",
              message: "Text has been copied",
              position: "",
              status: "Success",
            });
            toastIsVisible.set(true);
            setTimeout(() => {
              toastIsVisible.set(false);
            }, 5000);
          });
        }}
      />
    </div>
  </div>
  <div class="buttonWrapper">
    <Button
      title={"Close"}
      highlighted={true}
      minWidth={100}
      CB={() => {
        snipetModalIsVisible.set(false);
        addHostMenuIsVisible;
      }}
    />
  </div>
</div>
<div class="modalOverlay" id="modalOverlay" />
<svelte:window
  on:keydown={(e) => {
    handleKeydown(e, "Escape", () => {
      snipetModalIsVisible.set(false);
    });
  }}
/>
