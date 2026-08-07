<script>
  /**
   * @typedef {Object} Props
   * @property {string} [token]
   * @property {string} [origin]
   */

  /** @type {Props} */
  let { token = "", origin = "" } = $props();
</script>

<pre class="secretSnippet">
docker run -d --restart always --name onlogs \
  -e AGENT=true \
  -e HOST={origin} \
  -e ONLOGS_TOKEN={token} \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v /etc/hostname:/etc/hostname \
  -v /var/lib/docker/containers:/var/lib/docker/containers \
  -v onlogs-volume:/leveldb \
devforth/onlogs
  </pre>
