<script>
  // jsdom has no viewport, so a real IntersectionObserver could never fire.
  // Instances register themselves so a test can drive intersection explicitly.
  import { onMount } from "svelte";

  export let element = null;
  export let once = false;
  export let intersecting = false;
  export let root = null;
  export let rootMargin = "0px";
  export let threshold = 0;
  export let entry = null;
  export let observer = null;

  onMount(() => {
    const handle = {
      get element() {
        return element;
      },
      set: (value) => {
        intersecting = value;
      },
    };
    if (!globalThis.__onlogsObservers) globalThis.__onlogsObservers = [];
    globalThis.__onlogsObservers.push(handle);

    return () => {
      const index = globalThis.__onlogsObservers.indexOf(handle);
      if (index >= 0) globalThis.__onlogsObservers.splice(index, 1);
    };
  });
</script>

<slot />
