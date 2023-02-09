<a href="https://devforth.io"><img src="./df_powered_by.svg" style="height:36px"/></a>

# OnLogs
OnLogs is light docker logs listener that makes your containers debugging much easier.

- 🧸 Simple
- 🔑 Secure
- ⏱ Fast
- ✨Almost perfect. Almost✨ 

### Benefits
- 🔑 Secure and simple way to access logs without server/ssh accession
- 🏎 Using Golang & Svelte.js to get maximum work speed
- 🧸 Simple setup as docker run command/compose snippet
- 🖱 Get every service logs with 1 click
- ⌚ Saves your time
- 📱 Manage your logs from smartphone (we know it happens)
- 🧾 Open-Source commercial friendly MIT license
- 💾 Small size (13.22 MB)

### Features
- 💻 One host can be used to manage logs from all other hosts
- 🔗 Share log messages via link
- 📊 Statistics
- 🔎 Search through logs (configurable case sensetivity)
- 👁 View parameters (parsing JSON, show local/UTC time for every logline)
- 🔴 Realtime logs updating

### Roadmap
- 💽 Clear docker logs to avoid dublicates and doubling logs size on disk
- 🗂 Grouping hosts
- 🏷 Search by tags (log status, time)
- 📊 Improved statistics

## Hello world & ussage
### Docker Compose example with traefik
```sh
  onlogs:
    image: devforth/onlogs
    restart: always
    environment:
      - PASSWORD=<any password>
      - PORT=<any port>
    #  - ONLOGS_PATH_PREFIX=/<any path prefix> if using with path prefix

    ports:
      - <any port>:<any port>
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.onlogs.rule=Host(`<your host>`)"  # if using on subdomain
    #  - traefik.http.routers.onlogs.rule=PathPrefix(`</any path prefix>`) # if using with path prefix
      - "traefik.http.services.onlogs.loadbalancer.server.port=<any port>"
    volumes:
     - /var/run/docker.sock:/var/run/docker.sock
     - /etc/hostname:/etc/hostname
     - onlogs-volume:/leveldb

volumes:
  onlogs-volume:
```

### Docker Run example with traefik
```sh
docker run --restart always -e PASSWORD=<any password> -e PORT=<any port> \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v /etc/hostname:/etc/hostname \
    -v onlogs-volume:/leveldb \ 
    --label traefik.enable=true \
    --label traefik.http.routers.onlogs.rule=Host\(\`<your host>\`\) \ 
    --label traefik.http.services.onlogs.loadbalancer.server.port=2874 devforth/onlogs
```

Once done, just go to <your host> and login as "admin" with <any password>.

## Available Environment Options:
| Environment Variable       | Description   | Defaults | Required |
|----------------------------|---------------------------------|--------|-----------------|
| PASSWORD           | Password for default user                        |                    | +
| PORT               | Port to listen on                                | `2874`             | +
| JWT_SECRET         | Secret for JWT tokens for users                  | Generates randomly | -
| ONLOGS_PATH_PREFIX | Base path if you using OnLogs not on subdomain   |                    | only if using on path prefix
| CLIENT             | Toggles client mode. If enabled, there will be no web interface available and all logs will be sent  and stored on HOST                                                      | `false`
| HOST               | Url to OnLogs host from protocol to domain name. |                    | if `CLIENT=true`
| ONLOGS_TOKEN       | Token that will use client to authorize and connect to HOST | Generates with OnLogs interface   | if `CLIENT=true`
