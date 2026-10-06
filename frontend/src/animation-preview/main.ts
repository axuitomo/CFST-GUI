import "./preview.css";

type Demo = {
  id: string;
  index: string;
  category: string;
  title: string;
  description: string;
  duration: string;
};

const demos: Demo[] = [
  { id: "signal", index: "01", category: "进入", title: "信号扩散", description: "用连续的波纹提示一个任务或状态已经开始。", duration: "1.8s" },
  { id: "lift", index: "02", category: "进入", title: "卡片浮起", description: "内容从轻微的深度变化中出现，适合列表和结果卡片。", duration: "650ms" },
  { id: "scan", index: "03", category: "状态", title: "扫描光带", description: "一束光扫过内容，表达正在检查、同步或加载。", duration: "2.4s" },
  { id: "orbit", index: "04", category: "状态", title: "轨道进度", description: "用旋转轨道承载持续中的过程，不依赖跳动数字。", duration: "2.1s" },
  { id: "type", index: "05", category: "反馈", title: "逐字显现", description: "让一行短消息按节奏出现，用于提示或阶段性反馈。", duration: "1.2s" },
  { id: "breath", index: "06", category: "氛围", title: "呼吸强调", description: "极轻的亮度变化，适合引导注意力但不打断操作。", duration: "2.8s" },
  { id: "expand", index: "07", category: "页面", title: "页面展开", description: "从按钮原位展开为完整内容，适合详情面板和上下文预览。", duration: "700ms" },
  { id: "route", index: "08", category: "跳转", title: "页面跳转", description: "新页面沿着阅读方向进入，保留空间感但不制造拖沓。", duration: "560ms" },
  { id: "switch", index: "09", category: "切换", title: "页面切换", description: "同一容器内替换内容，适合设置页、标签页和工作台。", duration: "420ms" },
  { id: "drawer", index: "10", category: "面板", title: "抽屉展开", description: "从边缘进入的操作面板，适合筛选、详情和移动端菜单。", duration: "480ms" },
  { id: "toast", index: "11", category: "反馈", title: "提示出现", description: "短暂反馈从底部上浮，完成后自动退场，不遮挡主任务。", duration: "360ms" },
  { id: "stagger", index: "12", category: "列表", title: "列表渐入", description: "列表项按极短间隔出现，帮助用户建立内容顺序。", duration: "520ms" },
];

const renderDemo = (demo: Demo) => `
  <article class="demo-card" data-demo="${demo.id}">
    <div class="demo-card__meta">
      <span class="demo-card__index">${demo.index}</span>
      <span class="demo-card__category">${demo.category}</span>
      <span class="demo-card__duration">${demo.duration}</span>
    </div>
    <button class="demo-stage demo-stage--${demo.id}" type="button" aria-label="播放${demo.title}动画">
      <div class="demo-art" aria-hidden="true">
        ${demo.id === "signal" ? '<span class="signal-core"></span><span class="signal-ring signal-ring--one"></span><span class="signal-ring signal-ring--two"></span><span class="signal-ring signal-ring--three"></span>' : ""}
        ${demo.id === "lift" ? '<div class="lift-stack"><span></span><span></span><span></span></div>' : ""}
        ${demo.id === "scan" ? '<div class="scan-sheet"><span class="scan-line"></span><i></i><i></i><i></i><i></i></div>' : ""}
        ${demo.id === "orbit" ? '<div class="orbit-system"><span class="orbit-dot"></span><span class="orbit-core"></span></div>' : ""}
        ${demo.id === "type" ? '<div class="type-line"><span>结果已准备</span><b></b></div>' : ""}
        ${demo.id === "breath" ? '<div class="breath-mark"><span></span><span></span><span></span></div>' : ""}
        ${demo.id === "expand" ? '<div class="expand-scene"><span class="expand-origin"></span><span class="expand-panel"><i></i><i></i><i></i></span></div>' : ""}
        ${demo.id === "route" ? '<div class="route-scene"><span class="route-page route-page--back"></span><span class="route-page route-page--front"></span><b>→</b></div>' : ""}
        ${demo.id === "switch" ? '<div class="switch-scene"><span class="switch-view switch-view--old">A</span><span class="switch-view switch-view--new">B</span></div>' : ""}
        ${demo.id === "drawer" ? '<div class="drawer-scene"><span class="drawer-content"></span><span class="drawer-panel"><i></i><i></i></span></div>' : ""}
        ${demo.id === "toast" ? '<div class="toast-scene"><span class="toast-message">已保存</span><span class="toast-body"></span></div>' : ""}
        ${demo.id === "stagger" ? '<div class="stagger-scene"><i></i><i></i><i></i><i></i></div>' : ""}
      </div>
      <span class="demo-stage__hint">点击播放</span>
    </button>
    <div class="demo-card__body">
      <div>
        <h2>${demo.title}</h2>
        <p>${demo.description}</p>
      </div>
      <button class="demo-button" type="button" data-action="play" aria-label="播放${demo.title}">播放</button>
    </div>
    <button class="review-toggle" type="button" data-action="keep" aria-pressed="false">
      <span class="review-toggle__dot"></span><span data-review-label>标记为候选</span>
    </button>
  </article>
`;

const root = document.querySelector<HTMLDivElement>("#animation-preview");
if (!root) throw new Error("Animation preview root is missing");

root.innerHTML = `
  <main class="preview-shell">
    <header class="preview-header">
      <a class="brand" href="/" aria-label="返回 CFST-GUI">
        <span class="brand__mark">C</span><span>CFST<span class="brand__slash">/</span>motion</span>
      </a>
      <div class="review-counter" aria-live="polite"><span data-kept-count>0</span> 个候选</div>
    </header>
    <section class="preview-intro">
      <div class="intro-kicker"><span class="intro-kicker__line"></span> MOTION STUDIES / 2026</div>
      <h1>把状态，<em>动起来。</em></h1>
      <p>一组为 CFST-GUI 准备的动效草稿。逐个播放，标记你愿意带入正式界面的候选。</p>
      <div class="intro-note"><span>评审模式</span> 选择只是本页临时记录，不会修改现有前端。</div>
    </section>
    <section class="demo-grid" aria-label="动画效果列表">
      ${demos.map(renderDemo).join("")}
    </section>
    <footer class="preview-footer"><span>CFST / MOTION LAB</span><span>12 studies · click to replay</span></footer>
  </main>
`;

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
