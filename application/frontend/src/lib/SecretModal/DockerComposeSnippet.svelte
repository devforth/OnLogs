<script>
  let { token = "", origin = "" } = $props();
</script>

<pre class="secretSnippet">services:
  example_onlogs:
    image: devforth/onlogs
    restart: always
    environment:
      - AGENT=true
      - HOST={origin}
      - ONLOGS_TOKEN={token}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /etc/hostname:/etc/hostname
      - /var/lib/docker/containers:/var/lib/docker/containers
      - onlogs-volume:/leveldb

volumes:
  onlogs-volume:
</pre>
