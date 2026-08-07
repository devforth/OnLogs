<script>
  /**
   * @typedef {Object} Props
   * @property {string} [title]
   * @property {boolean} [border]
   * @property {boolean} [highlighted]
   * @property {number} [minWidth]
   * @property {number} [minHeight]
   * @property {string} [icon]
   * @property {any} [CB]
   * @property {number} [iconHeight]
   * @property {string} [type]
   * @property {boolean} [disabled]
   * @property {string} [id]
   */

  /** @type {Props} */
  let {
    title = "",
    border = true,
    highlighted = false,
    minWidth = 0,
    minHeight = 0,
    icon = "",
    CB = () => {},
    iconHeight = 12,
    type = "button",
    disabled = false,
    id = ""
  } = $props();
</script>

<button
  {id}
  {disabled}
  onclick={(e) => {
    CB();
    return e;
  }}
  style:min-width={`${minWidth}px`}
  style:min-height={`${minHeight}px`}
  class="button block {highlighted ? 'highlighted' : ''} {border
    ? 'border'
    : ''}"
>
  <i
    style:font-size={`${iconHeight}px`}
    style:line-height={"100%"}
    class={icon ? `${icon}` : ""}
    class:iconWidthText={title && icon}
></i>{title}
</button>
