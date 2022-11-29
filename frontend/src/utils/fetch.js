class fetchApi {
    constructor() {
      this.BASE_LOCAL_URL = "http://localhost:2874/api/v1/";
      this.isAuthorized = false;
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
            this.isAuthorized = false;
            return "401 - Unauthorized!"
        }

        this.isAuthorized = true;
        return await response.json();
    }

    async login(login="admin", password="aboba") {
        return await this.doFetch("POST", `${this.BASE_LOCAL_URL}login`, {
            "login": login,
            "password": password
        })
    }

    async getHosts() {
        return await this.doFetch("GET", `${this.BASE_LOCAL_URL}getHost`)
    }

    async getLogs(containerName="", search="", limit=30, offset=0) {
        return await this.doFetch("GET",
            `${this.BASE_LOCAL_URL}getLogs?id=${containerName}&search=${search}&limit=${limit}&offset=${offset}`
        )
    }
}

export default fetchApi;
