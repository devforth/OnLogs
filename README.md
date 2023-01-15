# OnLogs

Light docker logs listener that makes easier to debug your containers

## docker-compose.yml example
```
  onlogs:
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
      - "traefik.http.routers.onlogs.rule=Host(`<any host name>`)"
      - "traefik.http.services.onlogs.loadbalancer.server.port=<any port>"
    volumes:
     - /var/run/docker.sock:/var/run/docker.sock
     - /etc/hostname:/etc/hostname
     - onlogs-volume:/leveldb

volumes:
  onlogs-volume:
```


## Notes
cf problem bot fight mode
