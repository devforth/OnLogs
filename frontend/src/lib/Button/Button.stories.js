// @ts-ignore
import Button from "./Button.svelte";
import { action } from "@storybook/addon-actions";
import "../../assets/res/onLogsFont.css";

export const actionsData = {
  onCkickButton: action("onClickButton"),
};

export default {
  component: Button,
  title: "Button",
  excludeStories: /.*Data$/,
  //👇 The argTypes are included so that they are properly displayed in the Actions Panel
  argTypes: {
    onCkickButton: { action: "onClickButton" },
  },
};

const Template = ({ onCkickButton, ...args }) => ({
  Component: Button,
  props: args,
  on: {
    ...actionsData,
  },
});

export const Default = Template.bind({});
Default.args = {
  title: "Text",
  border: true,
  highlighted: false,
  minWidth: 90,
  minHeight: 32,
  icon: "",
  state: "BUTTON_TEXT",
  iconHeight: 12,
  
};
