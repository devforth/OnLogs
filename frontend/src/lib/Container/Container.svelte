<script>
 export let highlighted =false;
 export let minHeightVh = 10
 
  
</script>

<div 
class="container border block {highlighted?'highlighted':''}"style:min-height={`${minHeightVh}vh`}><slot></slot></div>






 