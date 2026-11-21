// 页面 markup 全部写实于 frontend/animation-preview.html（13 张 demo 卡片 + 外壳），
// 样式按分区拆在 ./styles/*.css，由 preview.css 汇总 @import。本文件只保留行为。
import "./preview.css";

const root = document.querySelector<HTMLDivElement>("#animation-preview");
if (!root) throw new Error("Animation preview root is missing");

const count = root.querySelector<HTMLElement>("[data-kept-count]");
const updateCount = () => {
  if (count) count.textContent = String(root.querySelectorAll('[data-action="keep"][aria-pressed="true"]').length);
};

const replay = (card: HTMLElement) => {
  card.classList.remove("is-playing");
  void card.offsetWidth;
  card.classList.add("is-playing");
};

root.addEventListener("click", (event) => {
  const target = event.target as HTMLElement;
  const card = target.closest<HTMLElement>("[data-demo]");
  if (!card) return;

  const action = target.closest<HTMLButtonElement>("[data-action]")?.dataset.action;
  if (action === "keep") {
    const button = target.closest<HTMLButtonElement>("[data-action=keep]");
    if (!button) return;
    const kept = button.getAttribute("aria-pressed") === "true";
    button.setAttribute("aria-pressed", String(!kept));
    const label = button.querySelector<HTMLElement>("[data-review-label]");
    if (label) label.textContent = kept ? "标记为候选" : "已加入候选";
    updateCount();
    return;
  }

  replay(card);
});

root.querySelectorAll<HTMLElement>("[data-demo]").forEach((card) => {
  card.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      replay(card);
    }
  });
});