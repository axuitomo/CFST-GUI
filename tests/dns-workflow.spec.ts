import { expect, test } from "@playwright/test";
import { normalizeSourceConfig } from "../packages/frontend/src/lib/bridge/config";
import { sourceBackend } from "./helpers/source-backend";

// DNS 读取作用域卡片：整张卡都要可点，只有右侧的读取来源切换器例外。
test("applies the DNS scope card from any blank area click", async ({ page }) => {
  await sourceBackend(page, [
    normalizeSourceConfig({ id: "url-1", name: "回归测试远程源", kind: "url", url: "https://example.com/regression-ips.txt", enabled: true }, 0),
  ]);
  await page.goto("/");
  await page.getByRole("navigation").getByRole("button", { name: "DNS 读取", exact: true }).click();

  const cards = page.locator("div.flex.flex-col.rounded-xl.border");
  await expect(cards).toHaveCount(3);
  const [zoneCard, configuredCard, customCard] = [cards.nth(0), cards.nth(1), cards.nth(2)];

  // 标题文字下方的空白区域（切换器左侧）也要能选中卡片。
  await zoneCard.click({ position: { x: 10, y: 100 } });
  await expect(zoneCard).toHaveClass(/border-primary/);

  await configuredCard.click({ position: { x: 10, y: 100 } });
  await expect(configuredCard).toHaveClass(/border-primary/);

  // 切换读取来源不应该丢掉卡片选中态，也不能反过来把卡片切回另一个作用域。
  await configuredCard.getByRole("button", { name: "分流记录名" }).click();
  await expect(configuredCard).toHaveClass(/border-primary/);
  await expect(configuredCard.getByRole("button", { name: "分流记录名" })).toHaveAttribute("aria-pressed", "true");

  await customCard.click({ position: { x: 10, y: 20 } });
  await expect(customCard).toHaveClass(/border-primary/);
  await expect(configuredCard).not.toHaveClass(/border-primary/);

  // 键盘仍然走卡片里的按钮：Enter 冒泡到卡片容器完成选中。
  await page.getByRole("button", { name: /当前配置记录/ }).focus();
  await page.keyboard.press("Enter");
  await expect(configuredCard).toHaveClass(/border-primary/);
});
