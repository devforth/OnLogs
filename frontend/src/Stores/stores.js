import { writable } from "svelte/store";

export const store = writable({
  UTCtime: true,
  breakLines: true,
});

export const userMenuOpen = writable(false);
export const addUserModalOpen = writable(false);
export const theme = writable("light");
