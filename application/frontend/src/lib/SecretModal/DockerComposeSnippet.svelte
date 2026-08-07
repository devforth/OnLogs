<script>
  /**
   * @typedef {Object} Props
   * @property {string} [token]
   * @property {string} [origin]
   */

  /** @type {Props} */
  let { token = "", origin = "" } = $props();
</script>

<pre class="secretSnippet">example_onlogs:
    image: devforth/onlogs 
    restart: always
    environment:
      - AGENT=true
      - HOST={origin}
      - ONLOGS_TOKEN={token}
    volumes:
     - /var/run/docker.sock:/var/run/docker.sock
     - /etc/hostname:/etc/hostname
     - onlogs-volume:/leveldb </pre>
