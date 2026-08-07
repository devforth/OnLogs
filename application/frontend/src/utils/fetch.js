import { navigate } from "svelte-routing";

import { changeKey } from "../utils/changeKey.js";

class fetchApi {
  constructor() {
    const isLocalhost = document.location.host.includes("localhost:5173");
    const protocol = window.location.protocol === "https:" ? "wss://" : "ws://";

    this.url = isLocalhost
      ? "http://localhost:2874/api/v1/"
      : `${changeKey}/api/v1/`;

    this.wsUrl = isLocalhost
      ? "ws://localhost:2874/api/v1/"
      : `${protocol}${document.location.host}${changeKey}/api/v1/`;
    this.authorized = true;
  }
  async doFetch(method, path, body = null, signal) {
    if (body !== null) {
      body = JSON.stringify(body);
    }
    const response = await fetch(path, {
      method: method,
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      credentials: "include",
      body,
      signal,
    });

    if (response.status === 401) {
      this.authorized = false;
      navigate(`${changeKey}/login`, { replace: true });
      return null;
    }
    if (!response.ok) {
      // Rejections carry {"error": "..."} and the caller has to be able to show
      // it; only a body that says nothing is worth throwing over.
      const rejected = await response.json().catch(() => null);
      if (rejected && typeof rejected === "object" && "error" in rejected) {
        return rejected;
      }
      throw new Error(`${method} ${path} failed with ${response.status}`);
    }
    return await response.json();
  }

  async checkCookie() {
    return await this.doFetch("GET", `${this.url}checkCookie`);
  }

  async login(login = "", password = "") {
    const result = await this.doFetch("POST", `${this.url}login`, {
      login: login,
      password: password,
    });
    if (result["error"] === null) {
      this.authorized = true;
      navigate(`${changeKey}`, { replace: true });
      return true;
    }
    return false;
  }

  async logout() {
    return await this.doFetch("GET", `${this.url}logout`);
  }

  async getHosts() {
    return await this.doFetch("GET", `${this.url}getHosts`);
  }

  async getLogs({
    containerName = "",
    search = "",
    limit = 30,
    status = "",

    caseSens = false,
    startWith = "",
    hostName = "",
    signal,
  }) {
    const params = new URLSearchParams({
      host: hostName,
      id: containerName,
      search,
      status,
      limit,
      startWith,
    });
    if (search) {
      params.set("caseSens", caseSens);
    }
    return await this.doFetch("GET", `${this.url}getLogs?${params}`, null, signal);
  }

  async getPrevLogs({
    containerName = "",
    search = "",
    limit = 30,
    offset = 0,
    status = "",
    caseSens = false,
    startWith = "",
    hostName = "",
  }) {
    const params = new URLSearchParams({
      host: hostName,
      id: containerName,
      search,
      status,
      limit,
      offset,
      startWith,
      caseSens,
    });
    return await this.doFetch("GET", `${this.url}getPrevLogs?${params}`);
  }

  async getUsers() {
    return await this.doFetch("GET", `${this.url}getUsers`);
  }

  async removeUser(login) {
    return await this.doFetch("POST", `${this.url}deleteUser`, { login });
  }

  async editUser(login, password) {
    return await this.doFetch("POST", `${this.url}editUser`, {
      login,
      password,
    });
  }

  async getSecret() {
    return await this.doFetch("GET", `${this.url}getSecret`);
  }

  async getAllLogsSize() {
    return await this.doFetch("GET", `${this.url}getSizeByAll`);
  }
  async getServiceLogsSize(host, service) {
    const params = new URLSearchParams({ host, service });
    return await this.doFetch("GET", `${this.url}getSizeByService?${params}`);
  }
  async cleanLogs(host, service) {
    return await this.doFetch("POST", `${this.url}deleteContainerLogs`, {
      host,
      service,
    });
  }

  async createUser({ login, password }) {
    return await this.doFetch("POST", `${this.url}createUser`, {
      login,
      password,
    });
  }
  async deleteService(host, service) {
    return await this.doFetch("POST", `${this.url}deleteContainer`, {
      host,
      service,
    });
  }
  async changeFavorite(host, service) {
    return await this.doFetch("POST", `${this.url}changeFavorite`, {
      host,
      service,
    });
  }
  async getGroups() {
    return await this.doFetch("GET", `${this.url}getGroups`);
  }
  async createGroup(name, members) {
    return await this.doFetch("POST", `${this.url}createGroup`, {
      name,
      members,
    });
  }
  async updateGroup(name, newName, members) {
    return await this.doFetch("POST", `${this.url}updateGroup`, {
      name,
      newName,
      members,
    });
  }
  async deleteGroup(name) {
    return await this.doFetch("POST", `${this.url}deleteGroup`, { name });
  }
  async getStats(options) {
    return await this.doFetch("POST", `${this.url}getStats`, options);
  }
  async getChartData({ host, service, unit, unitsAmount }) {
    return await this.doFetch("POST", `${this.url}getChartData`, {
      host,
      service,
      unit,
      unitsAmount,
    });
  }
  async getLogsWithPrev({
    containerName = "",

    limit = 30,

    startWith = "",
    hostName = "",
  }) {
    const params = new URLSearchParams({
      host: hostName,
      id: containerName,
      limit,
      startWith,
    });
    return await this.doFetch("GET", `${this.url}getLogWithPrev?${params}`);
  }

  async cleanDockerLogs(host, service) {
    return await this.doFetch("POST", `${this.url}deleteDockerLogs`, {
      host,
      service,
    });
  }

  async updateSettings(options) {
    return await this.doFetch("POST", `${this.url}updateUserSettings`, {
      ...options,
    });
  }

  async getUserSettings() {
    return await this.doFetch("GET", `${this.url}getUserSettings`);
  }

  async getLogsByTag({ host, containerName, limit, status, message }) {
    const params = new URLSearchParams({
      host,
      id: containerName,
      limit,
      status,
      message,
    });
    return await this.doFetch("GET", `${this.url}getUserSettings?${params}`);
  }
}

export default fetchApi;
