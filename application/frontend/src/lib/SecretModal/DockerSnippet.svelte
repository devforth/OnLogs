<script>
  export let token = "";
  export let origin = "";
</script>

<pre class="secretSnippet">
  docker run --restart always --name onlogs \
  -e CLIENT=true \
  -e HOST={origin} \
  -e ONLOGS_TOKEN={token} \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v /etc/hostname:/etc/hostname \
  -v onlogs-volume:/leveldb \
devforth/onlogs
  </pre>
