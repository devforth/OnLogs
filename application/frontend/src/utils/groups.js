import FetchApi from "./fetch.js";
import { groups } from "../Stores/stores.js";

// The one place the sidebar's group list is refilled from the server, so a
// mutation and the initial load can never drift apart.
export async function reloadGroups() {
  const data = await new FetchApi().getGroups();
  if (Array.isArray(data)) {
    groups.set(data);
  }
}
