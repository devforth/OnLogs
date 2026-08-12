import "./main.scss";
import { mount } from "svelte";
import App from "./App.svelte";
import "@/assets/res/onLogsFont.css";

const app = mount(App, {
  target: document.getElementById("app"),
});

export default app;
