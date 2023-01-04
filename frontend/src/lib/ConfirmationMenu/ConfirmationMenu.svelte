<script>
  import Button from "../Button/Button.svelte";
  import { confirmationObj } from "../../Stores/stores.js";
  import Input from "../Input/Input.svelte";
  let confirmationWord = makeid(5);
  let inputValue = "";
  let error = false;
  function makeid(length) {
    var result = "";
    var characters =
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
    var charactersLength = characters.length;
    for (var i = 0; i < length; i++) {
      result += characters.charAt(Math.floor(Math.random() * charactersLength));
    }
    return result;
  }
</script>

<div class="confirmationContainer">
  <h3 class="confirmationName">Are you absolutely sure?</h3>
  <div class="attentionZone">{$confirmationObj.message}</div>

  <div class="confirmationText">
    Please type: <span class="boldText {error && 'error'}"
      >{confirmationWord}</span
    > to confirm.
  </div>
  <Input
    placeholder={"Confirm string"}
    customClass={"editInput"}
    bind:value={inputValue}
  />
  <div class="buttonsBox">
    <Button
      disabled={confirmationWord !== inputValue ? true : false}
      title={"Confirm"}
      highlighted={true}
      CB={() => {
        if (confirmationWord === inputValue) {
          $confirmationObj.action();
        } else {
          error = true;
        }
      }}
    /><Button
      title={"Cancel"}
      CB={() => {
        confirmationObj.update((pv) => {
          return { ...pv, isVisible: false };
        });
      }}
    />
  </div>
</div>
