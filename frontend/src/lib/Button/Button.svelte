<script>

   import { createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  function clickButton() {
    dispatch('onClickButton');
  }

   
 
  export let title=null;
  export let border=true;
  export let highlighted =false;
  export let width=90;
  export let height=32;
  export let icon='';
  
</script>


<button style:width={`${width}px`} style:height={`${height}px`}  class="button block {highlighted?'highlighted':''} {border?'border':''}" >{title}
  <i class="{icon?`${icon}`:'log log-Arrow'}"></i>
</button>




 