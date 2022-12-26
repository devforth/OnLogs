import { navigate } from "svelte-routing";

class fetchApi {
  constructor() {
    this.url = document.location.host.includes("localhost")
      ? "http://localhost:2874/api/v1/"
      : "/api/v1/";
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
      navigate("/login", { replace: true });
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
      navigate("/", { replace: true });
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

  async getLogs(
    containerName = "",
    search = "",
    limit = 30,
    offset = 0,
    caseSens = false,
    startWith = ""
  ) {
    return await this.doFetch(
      "GET",
      `${
        this.url
      }getLogs?id=${containerName}&search=${search}&limit=${limit}&offset=${offset}${
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
    return await this.doFetch("POST", `${this.url}deleteUser`, {
      login,
      password,
    });
  }

  async createUser({ login, password }) {
    return await this.doFetch("POST", `${this.url}createUser`, {
      login,
      password,
    });
  }
}

export default fetchApi;
