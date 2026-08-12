<script>
  // jsdom has no viewport, so a real IntersectionObserver could never fire.
  // Instances register themselves so a test can drive intersection explicitly.
  // Runes + $bindable mirrors the real package: binding a hole into
  // `intersecting` has to fail here the same way it does in a browser.
  import { onMount } from "svelte";

  let {
    element = null,
    once = false,
    intersecting = $bindable(false),
    root = null,
    rootMargin = "0px",
    threshold = 0,
    entry = $bindable(null),
    observer = $bindable(null),
    children,
  } = $props();

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

{@render children?.()}
