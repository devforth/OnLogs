<script>
  // @ts-ignore
  import Container from "@/lib/Container/Container.svelte";
  import HostList from "../lib/HostList/HostList.svelte";
  import LogsView from "../Views/LogsView/LogsView.svelte";
  import Button from "../lib/Button/Button.svelte";
  import fetchApi from "../utils/fetch";

  const listMargins = { marginTop: "6.68vh" }
  let api = new fetchApi()

  $: selectedService = ""
  async function getHosts() {
    return await api.getHosts()
  }
</script>

<div class="contentContainer">
  <div class="subContainerLeft subContainer">
    <Container highlighted minHeightVh={79.3}>
      <div class="onLogsPanel">
        <div class="onLogsPanelHeader">
          <h1>onLogs</h1>
          <Button
            title=""
            border={false}
            highlighted
            minWidth={0}
            minHeight={0}
            icon="log log-Plus"
            iconHeight={18}
          />
        </div>
          {#await getHosts()}
            <p>loading...</p>
          {:then hosts}
            {#each hosts as host}
            <HostList bind:selectedName={selectedService} hostName={host["host"]} servicesData={host["services"]} {...listMargins}/>
            {/each}
          {:catch}
            <p style="margin-top: 15px;">Error</p>
          {/await}
      </div></Container>
    <!-- <Container minHeightVh={10.97}>1213414</Container> -->
  </div>
  <div class="subContainerMiddle subContainer">
    <!-- <Container minHeightVh={17.36}>1213414</Container> -->
    <Container minHeightVh={72.77}>
      <LogsView bind:serviceName={selectedService}/>
    </Container>
  </div>
  <div class="subContainerRight  subContainer">
    <!-- <Container minHeightVh={24.44}>1213414</Container> -->
    <!-- <Container minHeightVh={21.52}>1213414</Container> -->
    <div class="subContainerThumb">
      <!-- <Container minHeightVh={17.36}>1213414</Container> -->
      <!-- <Container minHeightVh={12.91} highlighted>1213414</Container> -->
    </div>
  </div>
</div>
