<script>
  import Modal from "../Modal/Modal.svelte";
  import Button from "../Button/Button.svelte";
  import FetchApi from "../../utils/fetch.js";
  import { reloadGroups } from "../../utils/groups.js";
  import {
    groupModalIsVisible,
    groupBeingEdited,
    toast,
    toastIsVisible,
  } from "../../Stores/stores.js";

  let { hosts = [] } = $props();

  const api = new FetchApi();
  // Mirrors the server's limit; the server stays the authority.
  const MAX_NAME_LENGTH = 64;

  let name = $state("");
  let selected = $state([]);
  let editing = $state(null);

  $effect(() => {
    if ($groupModalIsVisible) {
      // Read from the store, never back from `editing`, or the write reruns the
      // effect that made it.
      const group = $groupBeingEdited;
      editing = group;
      name = group?.name || "";
      selected = (group?.members || []).map((m) => `${m.host}/${m.service}`);
    }
  });

  // Neither half of a member can contain "/", so this is unambiguous.
  function isSelected(host, service) {
    return selected.includes(`${host}/${service}`);
  }

  function toggle(host, service) {
    const key = `${host}/${service}`;
    selected = selected.includes(key)
      ? selected.filter((e) => e !== key)
      : [...selected, key];
  }

  // A member whose service is gone still has to appear, or saving an unrelated
  // edit would silently drop it.
  let tree = $derived.by(() => {
    const byHost = hosts.map((h) => ({
      host: h.host,
      services: (h.services || []).map((s) => ({
        name: s.serviceName,
        missing: false,
      })),
    }));

    for (const member of editing?.members || []) {
      let entry = byHost.find((h) => h.host === member.host);
      if (!entry) {
        entry = { host: member.host, services: [] };
        byHost.push(entry);
      }
      if (!entry.services.some((s) => s.name === member.service)) {
        entry.services.push({ name: member.service, missing: true });
      }
    }
    return byHost;
  });

  // Built from the tree rather than the click order, so the payload is stable.
  let members = $derived(
    tree.flatMap((h) =>
      h.services
        .filter((s) => isSelected(h.host, s.name))
        .map((s) => ({ host: h.host, service: s.name }))
    )
  );

  function notify(status, tittle, message) {
    toast.set({ message, tittle, status, position: "" });
    toastIsVisible.set(true);
    setTimeout(() => {
      toastIsVisible.set(false);
    }, 5000);
  }

  function close() {
    groupModalIsVisible.set(false);
    groupBeingEdited.set(null);
  }

  async function save() {
    const trimmed = name.trim();
    if (!trimmed || trimmed.length > MAX_NAME_LENGTH) {
      notify(
        "error",
        "Error",
        `Group name has to be between 1 and ${MAX_NAME_LENGTH} characters.`
      );
      return;
    }

    let data;
    try {
      data = editing
        ? await api.updateGroup(editing.name, trimmed, members)
        : await api.createGroup(trimmed, members);
    } catch (e) {
      notify("error", "Error", e.message);
      return;
    }

    if (!data || data.error) {
      notify("error", "Error", data?.error || "Could not save the group.");
      return;
    }

    await reloadGroups();
    close();
  }

  async function remove() {
    let data;
    try {
      data = await api.deleteGroup(editing.name);
    } catch (e) {
      notify("error", "Error", e.message);
      return;
    }

    if (!data || data.error) {
      notify("error", "Error", data?.error || "Could not delete the group.");
      return;
    }

    await reloadGroups();
    close();
  }
</script>

<Modal modalIsOpen={$groupModalIsVisible} closeFunction={close}>
  <div class="groupModal">
    <h3>{editing ? "Edit services group" : "Create services group"}</h3>

    <input
      class="groupNameInput"
      type="text"
      placeholder="Group name"
      bind:value={name}
    />

    <div class="groupModalTree">
      {#each tree as host}
        <p class="groupModalHost">
          <i class="log log-Server"></i>{host.host}
        </p>
        {#each host.services as service}
          <label class="groupModalService clickable">
            <input
              type="checkbox"
              data-host={host.host}
              data-service={service.name}
              checked={isSelected(host.host, service.name)}
              onchange={() => toggle(host.host, service.name)}
            />
            <span class={service.missing ? "disabled" : ""} title={service.name}
              >{service.name}</span
            >
          </label>
        {/each}
      {/each}
    </div>

    <div class="groupModalButtons">
      {#if editing}
        <Button id="groupModalDelete" title="Delete" minWidth={86} CB={remove} />
      {/if}
      <Button
        id="groupModalSave"
        title={editing ? "Save" : "Create"}
        highlighted
        minWidth={86}
        CB={save}
      />
    </div>
  </div>
</Modal>
