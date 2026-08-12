// mount() takes a plain object, which is not reactive. Wrapping it in $state
// lets a test drive props after mount and observe $bindable writes coming back.
export function reactiveProps(initial) {
  let props = $state(initial);
  return props;
}
