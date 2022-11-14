class fetchApi {
    constructor() {
      this.BASE_LOCAL_URL = "http://localhost:2874/api/v1/";
      this.BASE_PROD_URL = "/api/v1";
    //   this.path =
    //     document.location.host === "coposter.me" ||
    //     document.location.host === "dev.coposter.me"
    //       ? this.BASE_PROD_URL
    //       : this.BASE_LOCAL_URL;
    }
    async login(login, password) {
        let path = `${this.BASE_LOCAL_URL}login`;
        const response = await fetch(path, {
            method: "GET",
            headers: {
                    Accept: "application/json",
                    "Content-Type": "application/json",
                },
            credentials: "include",
            body: JSON.stringify({
                "login": login,
                "password": password
            }),
        });

        return await response.json();
    }

    async getHosts() { // TODO should work only with cookie
        let path = `${this.BASE_LOCAL_URL}getHost`;
        const response = await fetch(path, {
            method: "GET",
            headers: {
                    Accept: "application/json",
                    "Content-Type": "application/json",
                },
            credentials: "same-origin",
        });

        // if (response.status === 401) { // TODO logout when status 401
            // methods.logOut();
        // }

        const hosts = []
        hosts.push(await response.json())
        return hosts
    }

    async getLogs() { // TODO should work only with cookie
        let path = `${this.BASE_LOCAL_URL}getLogs?limit=50`;
        const response = await fetch(path, {
            method: "GET",
            headers: {
                    Accept: "application/json",
                    "Content-Type": "application/json",
                },
            credentials: "same-origin",
        });

        // if (response.status === 401) { // TODO logout when status 401
            // methods.logOut();
        // }

        const logs = []
        logs.push(await response.json())
        console.log(logs)
        return logs
    }
}

export default fetchApi;
