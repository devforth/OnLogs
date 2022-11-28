class fetchApi {
    constructor() {
      this.BASE_LOCAL_URL = "http://localhost:2874/api/v1/";
    }

    async doFetch(method, path, body = null) {
        const response = await fetch(path, {
            method: method,
            headers: {
                    Accept: "application/json",
                    "Content-Type" : "application/json",
                },
            credentials: "include",
            body: JSON.stringify(body),
        });

        if (response.status === 401) { // TODO logout when status 401
            console.log("sraka")
            return null
        }

        return await response.json();
    }

    async login(login="admin", password="aboba") {
        return await this.doFetch("POST", `${this.BASE_LOCAL_URL}login`, {
            "login": login,
            "password": password
        })
    }

    async getHosts() {
        await this.login()  // REMOVE
        return await this.doFetch("GET", `${this.BASE_LOCAL_URL}getHost`)
    }

    async getLogs(containerName="", search="", limit=30, offset=0) {
        return await this.doFetch("GET",
            `${this.BASE_LOCAL_URL}getLogs?
            id=${containerName}&
            search=${search}&
            limit=${limit}&
            offset=${offset}`
        )
    }
}

export default fetchApi;
