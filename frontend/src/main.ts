import { createVaporApp, vaporInteropPlugin } from "vue";
import App from "./App.vue";
import { resolveBridgeMode } from "./lib/bridge";
import "./styles.css";

// 全树 Vapor：根组件经 @vitejs/plugin-vue 的 features.vapor 编译为 Vapor 组件，
// 运行时通过 vite 别名解析到 vue 的 runtime-with-vapor 构建。
// vaporInteropPlugin 让 Vapor 组件树能够渲染仍基于 VDOM 的第三方组件
// （如 @phosphor-icons/vue 图标）。
// 先定通道再挂载：Wails 宿主的运行时是导航完成后才注入的，若在就绪前挂载，首批调用会
// 误判成没有宿主（历史上表现为桌面端右下角弹出「WebUI 请求失败 (404)」，版本号停在占位
// 值）。通道判定只在 Wails 宿主页面上等一次运行时注入，其余页面立即挂载且不发起网络请求。
void resolveBridgeMode()
  .catch(() => undefined)
  .then(() => {
    createVaporApp(App).use(vaporInteropPlugin).mount("#app");
  });
