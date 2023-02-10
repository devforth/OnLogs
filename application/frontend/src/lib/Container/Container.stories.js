// @ts-ignore
import ContainerView from "./ContainerView.svelte";
import Container from "./Container.svelte";
import "../../assets/res/onLogsFont.css";



export default {
  component: ContainerView,
  title: "Container",
  excludeStories: /.*Data$/,
  //👇 The argTypes are included so that they are properly displayed in the Actions Panel
  argTypes: {
 
  },
};

const Template = ({ ...args }) => ({
  Component: ContainerView,
  props: args,
});

export const Default = Template.bind({});
Default.args = {
 
 
  newHighlighted: false,
 
};
