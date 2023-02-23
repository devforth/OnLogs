import { navigate } from "svelte-routing";

import { changeKey } from "../utils/changeKey.js";

class fetchApi {
  constructor() {
    this.url = document.location.host.includes("localhost:5173")
      ? "http://localhost:2874/api/v1/"
      : `${changeKey}/api/v1/`;

    this.wsUrl = document.location.host.includes("localhost:5173")
      ? "ws://localhost:2874/api/v1/"
      : `wss://${document.location.host}/api/v1/`;
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
    return await this.doFetch(
      "GET",
      `${
        this.url
      }getLogs?host=${hostName}&id=${containerName}&search=${search}&status=${status}&limit=${limit}&startWith=${startWith}${
        search ? `&caseSens=${caseSens}` : ""
      }`,
      null,
      signal
    );
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
    return await this.doFetch(
      "GET",
      `${this.url}getPrevLogs?host=${hostName}&id=${containerName}&search=${search}&status=${status}&limit=${limit}&offset=${offset}&startWith=${startWith}&caseSens=${caseSens}`
    );
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
    return await this.doFetch(
      "GET",
      `${this.url}getSizeByService?host=${host}&service=${service}`
    );
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
    return await this.doFetch(
      "GET",
      `${this.url}getLogWithPrev?host=${hostName}&id=${containerName}&limit=${limit}&startWith=${startWith}`
    );
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
    return await this.doFetch(
      "GET",
      `${this.url}getUserSettings?host=${host}&id=${containerName}&limit=${limit}&status=${status}&message=${message}`
    );
  }
}

export default fetchApi;
