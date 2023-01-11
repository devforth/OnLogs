<script>
  import Button from "../Button/Button.svelte";
  import {
    addUserModalOpen,
    userDeleteOpen,
    editUserOpen,
    toast,
    toastIsVisible,
  } from "../../Stores/stores.js";
  import fetchApi from "../../utils/fetch";
  import { onMount, onDestroy } from "svelte";
  import Modal from "../Modal/Modal.svelte";
  import Input from "../Input/Input.svelte";

  let usersList = [];

  let chosenUserLogin = "";
  let deleteModalIsOpen = false;
  let editModalIsOpen = false;
  let userPasswordValue = "";

  export let userForAdding = "";

  $: userForAdding && addUser(userForAdding);

  function addUser(u) {
    if (u) {
      usersList = [...usersList, u];
    }
  }

  const api = new fetchApi();

  async function getUsers() {
    const data = await api.getUsers();
    if (data?.users.at(0)) {
      usersList = data.users;
    }
  }

  async function editUser(login, password) {
    if (password) {
      const data = await api.editUser(login, password);

      if (!data.error) {
        editUserOpen.update((v) => !v);
        toastIsVisible.set(true);
        toast.set({
          message: "Password was changed",
          tittle: "Success",
          status: "debug",
          position: "",
        });
        setTimeout(() => {
          toastIsVisible.set(false);
        }, 5000);
      } else {
        toastIsVisible.set(true);
        toast.set({
          message: data.error,
          tittle: "Error",
          status: "error",
          position: "",
        });
        setTimeout(() => {
          toastIsVisible.set(false);
        }, 5000);
      }
    } else {
      toastIsVisible.set(true);
      toast.set({
        message: "Type a new password.",
        tittle: "Error",
        status: "error",
        position: "",
      });
      setTimeout(() => {
        toastIsVisible.set(false);
      }, 5000);
    }
  }

  async function removeUser(login) {
    const data = await api.removeUser(login);

    if (!data.error) {
      usersList = usersList.filter((u) => {
        return u !== login;
      });

      userDeleteOpen.set(false);
    } else {
      toastIsVisible.set(true);
      toast.set({
        message: data.error,
        tittle: "Error",
        status: "error",
        position: "",
      });
      setTimeout(() => {
        toastIsVisible.set(false);
      }, 5000);
    }
  }

  function showAddUserMenu() {
    addUserModalOpen.update((v) => !v);
  }

  function showUserDeleting() {
    userDeleteOpen.update((v) => !v);
  }

  function showUserEditing() {
    editUserOpen.update((v) => !v);
  }

  function setChosenUserLogin(e) {
    chosenUserLogin = e;
  }

  const delUnsubscribe = userDeleteOpen.subscribe((v) => {
    deleteModalIsOpen = v;
  });

  const editUnsubscribe = editUserOpen.subscribe((v) => {
    editModalIsOpen = v;
  });

  onMount(() => {
    getUsers();
  });
  onDestroy(() => {
    delUnsubscribe(), editUnsubscribe();
  });
</script>

<div>
  <h2 class="userMenu">UserMenu</h2>
  {#if usersList}<div>
      <div class="usersHeaderContainer">
        <h3>Users:</h3>
        <div class="addUserButton" on:click={showAddUserMenu}>
          <i class="log log-Plus" />
        </div>
      </div>
      <table class="userTable" role="list">
        <thead>
          <th scope="row">User</th><th style="opacity:0">Role</th><th
            style="opacity:0">Manage user</th
          >
        </thead>

        <tbody>
          {#each usersList as user, i}
            <tr
              ><td><span>{user}</span></td><td><span /></td><td
                ><span class="buttonSpanContainer"
                  ><span
                    class="buttonSpan"
                    on:click={() => {
                      setChosenUserLogin(user);
                      showUserDeleting();
                    }}><Button title={"Remove"} minWidth={86} /></span
                  >
                  <span
                    class="buttonSpan"
                    on:click={() => {
                      setChosenUserLogin(user);
                      showUserEditing();
                    }}><Button title={"Edit"} minWidth={86} /></span
                  ></span
                ></td
              ></tr
            >
          {/each}
        </tbody>
      </table>
    </div>{/if}
  <Modal modalIsOpen={deleteModalIsOpen} storeProp={userDeleteOpen}
    ><h3 style="text-align: center;">
      Are you sure you want to delete this user?
    </h3>
    <span class="buttonModalContainer"
      ><span
        class="buttonSpan"
        on:click={() => {
          removeUser(chosenUserLogin);
        }}><Button title={"Remove"} minWidth={86} highlighted /></span
      >
      <span class="buttonSpan"
        ><Button
          title={"Cancel"}
          minWidth={86}
          highlighted
          CB={showUserDeleting}
        /></span
      ></span
    >
  </Modal>
  <Modal modalIsOpen={editModalIsOpen} storeProp={editUserOpen}
    ><h3 style="text-align: center;">Please type new password</h3>
    <Input
      placeholder={"Password"}
      customClass={"editInput"}
      thumbClass={"editInputContainer"}
      bind:value={userPasswordValue}
    />

    <span class="buttonModalContainer"
      ><span
        class="buttonSpan"
        on:click={() => {
          editUser(chosenUserLogin, userPasswordValue);
        }}><Button title={"Edit"} minWidth={86} highlighted /></span
      >
      <span class="buttonSpan"
        ><Button
          title={"Cancel"}
          minWidth={86}
          highlighted
          CB={showUserEditing}
        /></span
      ></span
    >
  </Modal>
</div>
