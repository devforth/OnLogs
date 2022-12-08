import {replace} from "svelte-spa-router"

class fetchApi {
    constructor() {
        this.url = document.location.host.includes("localhost") ? "http://localhost:2874/api/v1/" : "/api/v1/";
        this.wsUrl = document.location.host.includes("localhost") ? "wss://localhost:2874/api/v1/" : `wss://${document.location.host}/api/v1/`;
        this.authorized = true;
    }
    async doFetch(method, path, body = null) {
        if (body != null) {
            body = JSON.stringify(body)
        }
        const response = await fetch(path, {
            method: method,
            headers: {
                    Accept: "application/json",
                    "Content-Type" : "application/json",
                },
            credentials: "include",
            body: body,
        });

        if (response.status === 401) {
            this.authorized = false
            replace("/login")
            return null
        }
        return await response.json()
    }

    async checkCookie() {
        return await this.doFetch("GET", `${this.url}checkCookie`)
    }

    async login(login="", password="") {
        const result = await this.doFetch("POST", `${this.url}login`, {
            "login": login,
            "password": password
        })
        if (result["error"] == null) {
            this.authorized = true
            replace("/view")
            return true
        }
        return false
    }

    async logout() {
        return await this.doFetch("GET", `${this.url}logout`)
    }

    async getHosts() {
        return await this.doFetch("GET", `${this.url}getHost`)
    }

    async getLogs(containerName="", search="", limit=30, offset=0) {
        return await this.doFetch("GET",
            `${this.url}getLogs?id=${containerName}&search=${search}&limit=${limit}&offset=${offset}`
        )
    }
}

export default fetchApi;
