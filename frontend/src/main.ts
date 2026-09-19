import { createVaporApp, vaporInteropPlugin } from "vue";
import App from "./App.vue";
import "./styles.css";

// 全树 Vapor：根组件经 @vitejs/plugin-vue 的 features.vapor 编译为 Vapor 组件，
// 运行时通过 vite 别名解析到 vue 的 runtime-with-vapor 构建。
// vaporInteropPlugin 让 Vapor 组件树能够渲染仍基于 VDOM 的第三方组件
// （如 @phosphor-icons/vue 图标）。
createVaporApp(App).use(vaporInteropPlugin).mount("#app");
