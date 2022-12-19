# OnLogs

Light docker logs listener that makes easier to debug your containers

## docker-compose.yml example
```
  coposter_onlogs:
    image: devforth/onlogs
    restart: always
    environment:
      - PASSWORD=notqwertypls
      - JWT_TOKEN=amogus
      - PORT=2874
    ports:
      - 2874:2874
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.coposter_onlogs.rule=(Host(`your.host.name`)"
      - "traefik.http.services.coposter_onlogs.loadbalancer.server.port=2874"
    volumes:
     - /var/run/docker.sock:/var/run/docker.sock
     - /etc/hostname:/etc/hostname
     - onlogs-logs-volume:/leveldb  # save logs after onlogs restart
     - onlogs-users-volume:/backend/onlogsdb  # save users after onlogs restart
```
