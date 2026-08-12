import FetchApi from "./fetch.js";
import { groups } from "../Stores/stores.js";

export async function reloadGroups() {
  const data = await new FetchApi().getGroups();
  if (Array.isArray(data)) {
    groups.set(data);
  }
}
