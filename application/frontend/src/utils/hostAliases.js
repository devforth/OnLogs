import FetchApi from "./fetch.js";
import { hostAliases } from "../Stores/stores.js";

export async function reloadHostAliases() {
  const data = await new FetchApi().getHostAliases();
  if (data && !data.error) {
    hostAliases.set(data);
  }
}
