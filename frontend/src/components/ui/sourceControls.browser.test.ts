import { afterEach, expect, test, vi } from "vitest";
import { page } from "vitest/browser";
import { createVaporApp, nextTick, reactive, vaporInteropPlugin } from "vue";
import SourceAddMenu from "./SourceAddMenu.vue";
import SourceFeedback from "./SourceFeedback.vue";
import SourceGroupControls from "./SourceGroupControls.vue";
import { normalizeSourceConfig } from "../../lib/bridge/config";
import "../../styles.css";

let unmount: (() => void) | undefined;
const host = document.createElement("div");
document.body.append(host);

afterEach(() => {
  unmount?.();
  host.replaceChildren();
  delete document.documentElement.dataset.theme;
});

test("reactively replaces source feedback without leaving stale live regions", async () => {
  const props = reactive({
    source: normalizeSourceConfig({ kind: "inline", content: "1.1.1.1", enabled: true }, 0),
    busy: false,
    error: "",
    preview: undefined as { totalCount: number; invalidCount: number } | undefined,
  });
  const app = createVaporApp(SourceFeedback, { $: [() => props] }).use(vaporInteropPlugin);
  app.mount(host);
  unmount = () => app.unmount();
  await expect.element(page.getByRole("status")).not.toBeInTheDocument();

  props.busy = true;
  await expect.element(page.getByRole("status")).toHaveTextContent("正在读取并检查来源…");
  props.busy = false;
  props.error = "来源服务器返回 503";
  await expect.element(page.getByRole("status")).toHaveTextContent("来源服务器返回 503 请检查来源内容或地址后重试。");
  props.error = "";
  props.preview = { totalCount: 2, invalidCount: 1 };
  await expect.element(page.getByRole("status")).toHaveTextContent("已读取 2 条候选。 忽略 1 条非法输入，请检查原始名单。");
  expect(host.querySelectorAll('[role="status"]')).toHaveLength(1);
  props.preview = undefined;
  await expect.element(page.getByRole("status")).not.toBeInTheDocument();
  props.source.enabled = false;
  await expect.element(page.getByRole("status")).not.toBeInTheDocument();
});

test("resets the add menu so the same source type can be added twice", async () => {
  const onAdd = vi.fn();
  const app = createVaporApp(SourceAddMenu, { compact: true, onAdd: () => onAdd });
  app.mount(host);
  unmount = () => app.unmount();
  const menu = page.getByRole("combobox", { name: "添加输入源" });
  await menu.selectOptions("url");
  await expect.element(menu).toHaveValue("");
  await menu.selectOptions("url");
  expect(onAdd.mock.calls).toEqual([["url"], ["url"]]);
  const bounds = host.querySelector("select")!.getBoundingClientRect();
  expect(bounds.width).toBe(36);
  expect(bounds.height).toBe(36);
});

test("updates group controls and renders actual light/dark OKLab styles on phone", async () => {
  const props = reactive({ mode: "traverse" as "traverse" | "mcis", canCopy: false, coloFilter: "", coloFilterMode: "allow" as const });
  const onModeChange = vi.fn((mode: "traverse" | "mcis") => {
    props.mode = mode;
  });
  const app = createVaporApp(SourceGroupControls, { $: [() => props], "onMode-change": () => onModeChange });
  app.mount(host);
  unmount = () => app.unmount();
  await expect.element(page.getByRole("button", { name: "复制当前组并编辑" })).toBeDisabled();
  await page.getByRole("combobox", { name: "输入组 IP 模式" }).selectOptions("mcis");
  expect(onModeChange).toHaveBeenCalledWith("mcis");
  await expect.element(page.getByText("本组所有来源先随机抽查候选", { exact: false })).toBeVisible();

  const field = host.querySelector("select")!;
  expect(CSS.supports("color", "oklab(0.5 0 0)")).toBe(true);
  const light = getComputedStyle(field).backgroundColor;
  document.documentElement.dataset.theme = "dark";
  await nextTick();
  await expect.poll(() => getComputedStyle(field).backgroundColor).not.toBe(light);
  await expect.poll(() => getComputedStyle(field).backgroundColor).toBe("oklab(0.273936 0.0015127 -0.0052642)");
  expect(getComputedStyle(document.documentElement).colorScheme).toBe("dark");
  const transitionProbe = document.createElement("div");
  transitionProbe.className = "ui-reveal-enter-active";
  host.append(transitionProbe);
  expect(getComputedStyle(transitionProbe).transitionDuration).toBe("0s");
  expect(document.documentElement.scrollWidth).toBe(390);
  document.documentElement.dataset.theme = "light";
  await expect.poll(() => getComputedStyle(field).backgroundColor).toBe(light);
});
