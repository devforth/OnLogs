<script>
  export let token = "";
  export let origin = "";
</script>

<pre class="secretSnippet">example_onlogs:
    image: devforth/onlogs 
    restart: always
    environment:
      - CLIENT=true
      - HOST={origin}
      - ONLOGS_TOKEN={token}
    volumes:
     - /var/run/docker.sock:/var/run/docker.sock
     - /etc/hostname:/etc/hostname
     - onlogs-volume:/leveldb </pre>
