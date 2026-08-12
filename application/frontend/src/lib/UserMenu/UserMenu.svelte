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

  let usersList = $state([]);

  let chosenUserLogin = $state("");
  let deleteModalIsOpen = $state(false);
  let editModalIsOpen = $state(false);
  let userPasswordValue = $state("");

  let { userForAdding = "" } = $props();


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
  $effect(() => {
    userForAdding && addUser(userForAdding);
  });
</script>

<div>
  <h2 class="userMenu">UserMenu</h2>

  {#if usersList}<div>
      <div class="usersHeaderContainer">
        <h3>Users:</h3>
        <div class="addUserButton" onclick={showAddUserMenu}>
          <i class="log log-Plus"></i>
        </div>
      </div>
      <table class="userTable" role="list">
        <thead>
          <tr>
            <th scope="col">User</th><th style="opacity:0">Role</th><th
              style="opacity:0">Manage user</th
            >
          </tr>
        </thead>

        <tbody>
          {#each usersList as user, i}
            <tr
              ><td><span>{user.username}</span></td><td><span></span></td><td>
                {#if user.editable}
                <span class="buttonSpanContainer"
                  ><span
                    class="buttonSpan"
                    onclick={() => {
                      setChosenUserLogin(user.username);
                      showUserEditing();
                    }}><Button title={"Edit"} minWidth={86} /></span
                  >
                  <span
                    class="buttonSpan"
                    onclick={() => {
                      setChosenUserLogin(user.username);
                      showUserDeleting();
                    }}><Button title={"Delete"} minWidth={86} /></span
                  ></span>
                {/if}
                </td
              ></tr
            >
          {/each}
        </tbody>
      </table>
    </div>{/if}
  <Modal modalIsOpen={deleteModalIsOpen} storeProp={userDeleteOpen}
    ><h3 style="text-align: center; margin: 32px 0 0 0">
      Are you sure you want to delete this user?
    </h3>
    <span class="buttonModalContainer" style="justify-content: space-between; margin-top: 20px; margin-bottom: 0"
      ><span
        class="buttonSpan"
        onclick={() => {
          removeUser(chosenUserLogin);
        }}><Button title={"Delete"} minWidth={86} highlighted /></span
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
    >
    <div style="width: auto;">
        <h3 style="text-align: center;">Please type new password</h3>
        <Input
        placeholder={"Password"}
        customClass={"editInput"}
        thumbClass={"editInputContainer"}
        bind:value={userPasswordValue}
        width={300}
        />

        <span class="buttonModalContainer" style="justify-content: space-between; margin-top: 20px; margin-bottom: 0;"
        ><span
            class="buttonSpan"
            onclick={() => {
            editUser(chosenUserLogin, userPasswordValue);
            }}><Button title={"Confirm"} minWidth={86} highlighted /></span
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
    </div>
  </Modal>
</div>
