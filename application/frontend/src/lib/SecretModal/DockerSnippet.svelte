<script>
  export let token = "";
  export let origin = "";
</script>

<pre class="secretSnippet">docker run --restart always \
-e CLIENT=true \
-e HOST={origin} \
-e ONLOGS_TOKEN={token} \
 </pre>
