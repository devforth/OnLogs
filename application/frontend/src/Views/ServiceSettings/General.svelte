<script>
  // @ts-nocheck

  import Button from "../../lib/Button/Button.svelte";
  import {
    lastChosenHost,
    lastChosenService,
    confirmationObj,
    toast,
    toastIsVisible,
  } from "../../Stores/stores.js";
  import { navigate } from "svelte-routing";
  import { changeKey } from "../../utils/changeKey.js";

  import FetchApi from "../../utils/fetch";
  const fetchApi = new FetchApi();

  async function deleteService() {
    confirmationObj.set({
      action: async function () {
        let data;
        try {
          data = await fetchApi.deleteService(
            $lastChosenHost,
            $lastChosenService
          );
        } catch {
          data = { error: "Could not delete the service. Please try again later." };
        }
        if (data?.error) {
          toast.set({
            tittle: "Error",
            message: data.error,
            position: "",
            status: "Error",
          });
          toastIsVisible.set(true);
          return;
        }
        if (data) {
          const hosts = (await fetchApi.getHosts()) || [];
          const next =
            hosts.find((h) => h.host === $lastChosenHost && h.services.length) ||
            hosts.find((h) => h.services.length);
          const nextService = next?.services[0].serviceName;

          lastChosenHost.set(next?.host || "");
          lastChosenService.set(nextService || "");
          navigate(
            next ? `${changeKey}/view/${next.host}/${nextService}` : `${changeKey}/`,
            { replace: true }
          );
          confirmationObj.update((pv) => {
            return { ...pv, isVisible: false };
          });
        }
      },
      message: "OnLogs (the service and all its logs).",

      isVisible: true,
    });
  }
</script>

<div class="serviceSettings">
  <h3 class="title">Change settings for current service:</h3>
  <p class="text">HOST: <span>{$lastChosenHost}</span></p>
  <p class="text">SERVICE: <span>{$lastChosenService}</span></p>
  <div class="actionThumb">
    <p class="actionTitle">
      <span>Delete this service </span>
      Once you delete a service, there is no going back. Please be certain.
    </p>
    <div>
      <Button title={"Delete service"} minWidth={160} CB={deleteService} />
    </div>
  </div>
</div>
