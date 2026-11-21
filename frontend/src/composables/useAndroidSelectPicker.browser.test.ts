import { afterEach, expect, test } from "vitest";
import { useAndroidSelectPicker } from "./useAndroidSelectPicker";

let cleanup: (() => void) | undefined;
const host = document.createElement("div");
document.body.append(host);

function makeSelect(options: Array<{ disabled?: boolean; label: string; value: string }>, selectedValue: string) {
  const label = document.createElement("label");
  const span = document.createElement("span");
  span.className = "ui-label";
  span.textContent = "区域";
  const select = document.createElement("select");
  select.name = "region";
  for (const option of options) {
    const element = document.createElement("option");
    element.value = option.value;
    element.textContent = option.label;
    element.disabled = option.disabled ?? false;
    element.selected = option.value === selectedValue;
    select.append(element);
  }
  select.value = selectedValue;
  label.append(span, select);
  host.append(label);
  return select;
}

function setup(android = true) {
  const picker = useAndroidSelectPicker({ isAndroidApp: () => android });
  picker.installAndroidViewportTracking();
  cleanup = () => {
    picker.uninstallAndroidViewportTracking();
    host.replaceChildren();
  };
  return picker;
}

afterEach(() => {
  cleanup?.();
  cleanup = undefined;
  delete document.documentElement.dataset.cfstAndroid;
  delete document.documentElement.dataset.cfstAndroidKeyboard;
});

test("install 标记 Android 视口，pointerdown 打开仿真面板并映射选项", () => {
  const picker = setup();
  expect(document.documentElement.dataset.cfstAndroid).toBe("true");
  const select = makeSelect(
    [
      { label: "全部", value: "all" },
      { label: "电信", value: "ct" },
      { disabled: true, label: "禁用项", value: "off" },
    ],
    "all",
  );
  select.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
  expect(picker.androidSelectPicker.open).toBe(true);
  expect(picker.androidSelectPicker.title).toBe("区域");
  expect(picker.androidSelectPicker.value).toBe("all");
  expect(picker.androidSelectPicker.options).toEqual([
    { disabled: false, label: "全部", selected: true, value: "all" },
    { disabled: false, label: "电信", selected: false, value: "ct" },
    { disabled: true, label: "禁用项", selected: false, value: "off" },
  ]);
});

test("选择选项写回 select 并派发 input/change，面板关闭", () => {
  const picker = setup();
  const select = makeSelect(
    [
      { label: "全部", value: "all" },
      { label: "电信", value: "ct" },
    ],
    "all",
  );
  const events: string[] = [];
  select.addEventListener("input", () => events.push("input"));
  select.addEventListener("change", () => events.push("change"));
  select.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
  picker.chooseAndroidSelectOption(picker.androidSelectPicker.options[1]);
  expect(select.value).toBe("ct");
  expect(events).toEqual(["input", "change"]);
  expect(picker.androidSelectPicker.open).toBe(false);
  expect(picker.androidSelectPicker.options).toEqual([]);
});

test("禁用选项不写回，Escape 关闭面板", () => {
  const picker = setup();
  const select = makeSelect(
    [
      { label: "全部", value: "all" },
      { disabled: true, label: "禁用项", value: "off" },
    ],
    "all",
  );
  select.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
  picker.chooseAndroidSelectOption(picker.androidSelectPicker.options[1]);
  expect(select.value).toBe("all");
  expect(picker.androidSelectPicker.open).toBe(true);
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  expect(picker.androidSelectPicker.open).toBe(false);
});

test("disabled select 与非 Android 环境不打开面板", () => {
  const picker = setup();
  const disabledSelect = makeSelect([{ label: "全部", value: "all" }], "all");
  disabledSelect.disabled = true;
  disabledSelect.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
  expect(picker.androidSelectPicker.open).toBe(false);
  cleanup?.();
  cleanup = undefined;
  host.replaceChildren();

  const desktop = setup(false);
  const select = makeSelect([{ label: "全部", value: "all" }], "all");
  select.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
  expect(desktop.androidSelectPicker.open).toBe(false);
});

test("无 label 时标题回退 aria-label", () => {
  const picker = setup();
  const select = document.createElement("select");
  select.setAttribute("aria-label", "推送平台");
  const option = document.createElement("option");
  option.value = "cf";
  option.textContent = "Cloudflare";
  select.append(option);
  host.append(select);
  select.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
  expect(picker.androidSelectPicker.title).toBe("推送平台");
});

test("uninstall 清理 dataset 与监听，面板不再响应", () => {
  const picker = setup();
  const select = makeSelect([{ label: "全部", value: "all" }], "all");
  picker.uninstallAndroidViewportTracking();
  expect(document.documentElement.dataset.cfstAndroid).toBeUndefined();
  expect(document.documentElement.dataset.cfstAndroidKeyboard).toBeUndefined();
  select.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
  expect(picker.androidSelectPicker.open).toBe(false);
  cleanup = undefined;
  host.replaceChildren();
});