# OnLogs

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
