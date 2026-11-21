  // 页面有两个状态：ready（配置）和 installing（日志 + 进度）。
  // 真实运行由 Go 后端通过 /api/* 驱动；带 ?demo=installing 时用假数据渲染，
  // 方便在没有编译整个安装器的机器上比对设计稿。
  const $ = (id) => document.getElementById(id);
  const demo = new URLSearchParams(location.search).get('demo') === 'installing';
  const els = {
    log: $('console'), bar: $('bar'), status: $('statusText'), meta: $('statusMeta'),
    install: $('install'), cancel: $('cancel'), close: $('close'),
    target: $('target'), desktop: $('desktop'), launch: $('launch'), browse: $('browse'),
    legalOpen: $('legalOpen'), legalOverlay: $('legalOverlay'), legalClose: $('legalClose'), legalBody: $('legalBody'),
    runningOverlay: $('runningOverlay'), runningConfirm: $('runningConfirm'), runningCancel: $('runningCancel'),
    runningClose: $('runningClose'), runningNote: $('runningNote'),
  };
  let stream = null;

  const esc = (s) => String(s).replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]));
  const clock = () => {
    const d = new Date();
    return String(d.getMinutes()).padStart(2, '0') + ':' + String(d.getSeconds()).padStart(2, '0') + '.' +
      String(d.getMilliseconds()).padStart(3, '0').slice(0, 2);
  };

  function renderLine(text, tail, tailCls) {
    const row = document.createElement('div');
    row.innerHTML = '<span class="t">[' + clock() + ']</span> ' + esc(text) +
      (tail ? '<span class="' + tailCls + '">' + esc(tail) + '</span>' : '');
    els.log.appendChild(row);
    els.log.scrollTop = els.log.scrollHeight;
  }

  function setPhase(phase) { document.body.dataset.phase = phase; }

  function ready() {
    setPhase('ready');
    els.log.textContent = '';
    els.bar.style.width = '0';
    els.status.textContent = '等待安装';
    els.status.style.color = '';
    els.meta.textContent = '';
    els.install.disabled = false;
    els.install.textContent = '立即安装';
    els.cancel.textContent = '取消';
  }

  function post(path) {
    return fetch(path, { method: 'POST' }).catch(() => {});
  }

  async function openLegal(name) {
    els.legalOverlay.hidden = false;
    els.legalBody.textContent = '加载中…';
    for (const tab of document.querySelectorAll('.legal-tab')) {
      tab.setAttribute('aria-selected', String(tab.dataset.name === name));
    }
    if (demo) {
      els.legalBody.textContent = '演示模式：安装包构建后在此显示内嵌许可证原文。';
      return;
    }
    try {
      const res = await fetch('/api/legal?name=' + encodeURIComponent(name), { cache: 'no-store' });
      if (!res.ok) throw new Error((await res.text()).trim() || ('HTTP ' + res.status));
      els.legalBody.textContent = await res.text();
    } catch (err) {
      els.legalBody.textContent = '无法读取许可证：' + ((err && err.message) ? err.message : err);
    }
  }

  function closeLegal() {
    els.legalOverlay.hidden = true;
  }

  function finish(ok, message) {
    if (stream) { stream.close(); stream = null; }
    els.status.textContent = ok ? '安装完成' : '安装失败';
    els.status.style.color = ok ? '' : 'var(--danger)';
    els.meta.textContent = '';
    els.install.disabled = false;
    els.install.textContent = '关闭';
    els.install.dataset.action = 'close';
    if (message) renderLine(message, ok ? '成功' : '失败', ok ? 'ok' : 'warn');
  }

  // 安装前询问：目标程序正在运行时，先征得用户同意再关闭它。
  function askToCloseRunningApp() {
    return new Promise((resolve) => {
      const finishAsk = (value) => {
        els.runningOverlay.hidden = true;
        els.runningConfirm.disabled = false;
        els.runningConfirm.onclick = null;
        els.runningCancel.onclick = null;
        els.runningClose.onclick = null;
        resolve(value);
      };
      els.runningNote.textContent = '安装会先结束该安装目录下的应用进程。';
      els.runningConfirm.disabled = false;
      els.runningOverlay.hidden = false;
      els.runningCancel.onclick = () => finishAsk(false);
      els.runningClose.onclick = () => finishAsk(false);
      els.runningConfirm.onclick = async () => {
        els.runningConfirm.disabled = true;
        els.runningNote.textContent = '正在关闭应用…';
        try {
          const res = await fetch('/api/close-app?dir=' + encodeURIComponent(els.target.value), { method: 'POST' })
            .then((r) => r.json());
          if (!res || !res.closed) throw new Error((res && res.error) || '未能关闭应用');
          finishAsk(true);
        } catch (err) {
          els.runningConfirm.disabled = false;
          els.runningNote.textContent = '关闭失败：' + ((err && err.message) ? err.message : err) + '。请手动退出应用后重试。';
        }
      };
    });
  }

  async function closeRunningAppIfNeeded() {
    if (demo) return true;
    try {
      const state = await fetch('/api/running?dir=' + encodeURIComponent(els.target.value), { cache: 'no-store' })
        .then((r) => r.json());
      if (!state || !state.running) return true;
    } catch (_) {
      return true;
    }
    return askToCloseRunningApp();
  }

  async function startInstall() {
    if (els.install.disabled) return;
    els.install.disabled = true;
    if (!(await closeRunningAppIfNeeded())) {
      els.install.disabled = false;
      renderLine('已取消安装', '', '');
      return;
    }
    setPhase('installing');
    els.install.textContent = '安装中...';
    els.status.textContent = '正在安装...';
    els.meta.textContent = '';
    els.cancel.textContent = '取消';

    if (demo) {
      const lines = [
        ['正在检查 Microsoft Edge WebView2 运行时... ', '正常'],
        ['正在解压 cfst-gui.exe 到 ' + els.target.value + String.fromCharCode(92), ''],
        ['正在创建桌面快捷方式... ', '完成'],
        ['正在写入系统卸载信息... ', '完成'],
      ];
      lines.forEach(([t, tail], i) => setTimeout(() => {
        renderLine(t, tail, 'ok');
        els.bar.style.width = ((i + 1) * 20) + '%';
      }, 90 * i));
      setTimeout(() => { finish(true, '安装完成'); els.bar.style.width = '100%'; }, 520);
      return;
    }

    const q = new URLSearchParams({
      dir: els.target.value,
      desktop: String(els.desktop.checked),
      launch: String(els.launch.checked),
    });
    stream = new EventSource('/api/install?' + q.toString());
    stream.onmessage = (event) => {
      const data = JSON.parse(event.data);
      if (data.line) renderLine(data.line, '', '');
      if (typeof data.p === 'number') els.bar.style.width = data.p + '%';
      if (data.done) finish(!!data.ok, data.error || (data.cancelled ? '已取消' : ''));
    };
    stream.onerror = () => finish(false, '连接中断');
  }

  els.install.addEventListener('click', () => {
    if (els.install.dataset.action === 'close') { post('/api/close'); return; }
    startInstall();
  });
  // 无边框窗口拖动：交给 Go 端触发 Windows 原生移动。
  // 提权窗口里 WebView2 的拖拽消息转发不可靠，不能依赖 Wails 运行时。
  const titlebar = document.querySelector('.titlebar');
  if (titlebar) {
    titlebar.addEventListener('mousedown', (event) => {
      if (event.button !== 0 || (event.target.closest && event.target.closest('.win-close'))) return;
      event.preventDefault();
      fetch('/api/drag', { method: 'POST' }).catch(() => {});
    });
  }
  els.cancel.addEventListener('click', () => {
    if (document.body.dataset.phase === 'installing') {
      if (!demo) post('/api/cancel');
      return;
    }
    post('/api/close');
  });

  els.close.addEventListener('click', () => post('/api/close'));

  els.legalOpen.addEventListener('click', () => openLegal('LICENSE'));
  els.legalClose.addEventListener('click', closeLegal);
  els.legalOverlay.addEventListener('click', (event) => {
    if (event.target === els.legalOverlay) closeLegal();
  });
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !els.legalOverlay.hidden) closeLegal();
  });
  for (const tab of document.querySelectorAll('.legal-tab')) {
    tab.addEventListener('click', () => openLegal(tab.dataset.name));
  }

  els.browse.addEventListener('click', async () => {
    if (demo) return;
    const res = await fetch('/api/browse', { method: 'POST' }).then((r) => r.json()).catch(() => null);
    if (res && res.dir) els.target.value = res.dir;
  });

  ready();
  if (demo) startInstall();
