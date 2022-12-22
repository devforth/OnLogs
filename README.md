# OnLogs

Light docker logs listener that makes easier to debug your containers

## docker-compose.yml example
```
  example_onlogs:
    image: devforth/onlogs
    restart: always
    environment:
      - PASSWORD=<any password>
      - JWT_TOKEN=<any token>
      - PORT=<any port>
    ports:
      - <any port>:<any port>
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.example_onlogs.rule=(Host(`your.host.name`)"
      - "traefik.http.services.example_onlogs.loadbalancer.server.port=<any port>"
    volumes:
     - /var/run/docker.sock:/var/run/docker.sock
     - /etc/hostname:/etc/hostname
     - onlogs-logs-volume:/leveldb  # save logs after onlogs restart
     - onlogs-users-volume:/backend/onlogsdb  # save users after onlogs restart
```
