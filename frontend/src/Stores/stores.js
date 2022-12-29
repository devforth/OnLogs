import { writable } from "svelte/store";

export const store = writable({
  UTCtime: true,
  breakLines: true,
  // used insensitive prop coz for now default value MUST be true
  caseInSensitive: true,
});

//modals state
export const userMenuOpen = writable(false);
export const userDeleteOpen = writable(false);
export const addUserModalOpen = writable(false);
export const editUserOpen = writable(false);

export const theme = writable("light");

// hosts service

export const lastChosenHost = writable("");
export const lastChosenService = writable("");

// toast state

export const toast = writable({
  tittle: "",
  message: "",
  position: "",
  status: "",
});
export const toastIsVisible = writable(false);

// active menu option
export const activeMenuOption = writable("home");

//add host menu
export const addHostMenuIsVisible = writable(false);

//snippet modal
export const snipetModalIsVisible = writable(false);
