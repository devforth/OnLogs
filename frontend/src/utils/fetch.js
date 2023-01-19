import { navigate } from "svelte-routing";
i;
import { changeKey } from "../utils/changeKey.js";
let value = "";

console.log(value, "value");

class fetchApi {
  constructor() {
    this.url = document.location.host.includes("localhost")
      ? // ? "http://localhost:2874/api/v1/"
        `${changeKey}/api/v1/`
      : "";
    this.wsUrl = document.location.host.includes("localhost")
      ? "ws://localhost:2874/api/v1/"
      : `wss://${document.location.host}/api/v1/`;
    this.authorized = true;
  }
  async doFetch(method, path, body = null) {
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
      body: body,
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
    offset = 0,
    caseSens = false,
    startWith = "",
    hostName = "",
  }) {
    return await this.doFetch(
      "GET",
      `${
        this.url
      }getLogs?host=${hostName}&id=${containerName}&search=${search}&limit=${limit}&offset=${offset}${
        search ? `&startWith=${startWith}&caseSens=${caseSens}` : ""
      }`
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
}

export default fetchApi;
