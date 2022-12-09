import { writable } from "svelte/store";

export const store = writable({
  UTCtime: true,
  breakLines: true,
});
