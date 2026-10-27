import { get, writable } from "svelte/store";

export const store = writable({
  UTCtime: true,
  breakLines: true,
  // used insensitive prop coz for now default value MUST be true
  caseInSensitive: true,
  transformJson: true,
  deleteFromDocker: false,
});

//chosen logs string
export const chosenLogsString = writable("");

//modals state
export const userMenuOpen = writable(false);
export const userDeleteOpen = writable(false);
export const addUserModalOpen = writable(false);
export const editUserOpen = writable(false);

export const theme = writable("light");

// hosts service

export const lastChosenHost = writable("");
export const lastChosenService = writable("");

export const hostAliases = writable({});
export const hostBeingRenamed = writable(null);

// service groups, loaded from /api/v1/getGroups
export const groups = writable([]);
export const groupModalIsVisible = writable(false);
// The group being edited, or null when the modal is creating a new one.
export const groupBeingEdited = writable(null);

// toast state

export const toast = writable({
  tittle: "",
  message: "",
  position: "",
  status: "",
  additionButton: {},
});
// A toast hides itself TOAST_MS after it was last shown, unless it offers an
// action. While that timer runs set(false) is ignored: it comes from a timer
// armed for an earlier toast. A timer registered in toastTimeoutId takes over
// the hiding. dismiss() is the user closing the toast.
const TOAST_MS = 5000;
let autoHide = null;
let registeredHide = null;

function stopToastTimers() {
  clearTimeout(autoHide);
  clearTimeout(registeredHide);
  autoHide = null;
  registeredHide = null;
}

const toastVisible = writable(false);
export const toastIsVisible = {
  subscribe: toastVisible.subscribe,
  set(visible) {
    if (!visible) {
      if (!autoHide) toastVisible.set(false);
      return;
    }
    stopToastTimers();
    if (!get(toast).additionButton?.isVisible) {
      autoHide = setTimeout(() => {
        autoHide = null;
        toastVisible.set(false);
      }, TOAST_MS);
    }
    toastVisible.set(true);
  },
  dismiss() {
    stopToastTimers();
    toastVisible.set(false);
  },
};

const toastTimeout = writable(null);
export const toastTimeoutId = {
  subscribe: toastTimeout.subscribe,
  set(id) {
    clearTimeout(autoHide);
    autoHide = null;
    registeredHide = id;
    toastTimeout.set(id);
  },
};

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
  // null means "the default clear-logs flow"; a function replaces it.
  action: null,
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
export const lastStatisticPeriod = writable("hour");

//spiner

export const isPending = writable(false);

//url hash

export const urlHash = writable("");

//cancel fetch (for bed connection)

export const isFeatching = writable(false);
export const isSearching = writable(false);

//status for serching logs by status

export const chosenStatus = writable("");

//last logTime for logsStream
export const lastLogTime = writable("");

// webSocket isMuted
export const WSisMuted = writable(false);
export const manuallyUnmuted = writable(false);
