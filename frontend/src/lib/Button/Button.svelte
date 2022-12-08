<script>

   import { createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  function clickButton() {
    dispatch('onClickButton');
  }
 
  export let title="";
  export let border=true;
  export let highlighted =false;
  export let minWidth=0;
  export let minHeight=0;
  export let icon='';
 
  export let iconHeight=12;

</script>


<button on:click={() => {clickButton()} } style:min-width={`${minWidth}px`} style:min-height={`${minHeight}px`}  class="button block {highlighted?'highlighted':''} {border?'border':''}" >
   <i style:font-size={`${iconHeight}px`} style:line-height={"100%"}
  class="{icon?`${icon}`:''}"
  class:iconWidthText={title&&icon}
  
  
  ></i>{title}
 
</button>




 