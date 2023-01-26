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
export const currentSnippedOption = writable("Docker");

//hosts list scroll is visible
export const listScrollIsVisible = writable(false);

//confirmation menu
export const confirmationObj = writable({
  action: function () {},
  message:
    "You want to delete host service logs. This data will be lost. This action cannot be undone.",

  isVisible: false,
});

//serviceSettings
export const lastChosenSetting = writable("General");

//make highlightsLogs

export const lastLogTimestamp = writable(0);

//stats
export const lastStatsPeriod = writable(2);
