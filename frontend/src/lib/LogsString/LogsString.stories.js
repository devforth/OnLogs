// @ts-ignore
import LogsString from "./LogsString.svelte";
import { action } from "@storybook/addon-actions";
import "../../assets/res/onLogsFont.css";



export default {
  component: LogsString,
  title: "LogsString",
  excludeStories: /.*Data$/,
  argTypes: {
    status: {
      type: "string",
      description: "Status of string",
      defaultValue: "debug",
      options: ["error", "debug", "warn", "info"],
      control: { type: "radio" },
    },
  },

  //👇 The argTypes are included so that they are properly displayed in the Actions Panel
};

const Template = ({ ...args }) => ({
  Component: LogsString,
  props: args,
  
});

export const Default = Template.bind({});
Default.args = {
  status: "debug",
  time: " 16 Jul 07:18:30.683",
  message:"starting PostgreSQL 14.0 (Debian 14.0-1.pgdg110+1) on x86_64-pc-linux-gnu, compiled by gcc (Debian 10.2.1-6) 10.2.1 20210110, 64-bit"

};
