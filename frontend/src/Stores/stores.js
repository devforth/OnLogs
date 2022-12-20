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
