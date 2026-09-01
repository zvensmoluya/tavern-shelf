const state = { characters: [], query: "", selected: null };
const content = document.querySelector("#content");
const notice = document.querySelector("#notice");
const dialog = document.querySelector("#detail-dialog");

const escapeHTML = (value = "") => String(value).replace(/[&<>'"]/g, char => ({
  "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;"
}[char]));

function avatar(character, detail = false) {
  if (character.avatarUrl) {
    return `<img src="${character.avatarUrl}" alt="${escapeHTML(character.name)} 的角色卡头像">`;
  }
  const initial = Array.from(character.name || "?")[0] || "?";
  return `<div class="cover-fallback"${detail ? "" : " aria-hidden=\"true\""}>${escapeHTML(initial)}</div>`;
}

function showNotice(message = "") {
  notice.textContent = message;
  notice.classList.toggle("hidden", !message);
}

function render() {
  const query = state.query.trim().toLocaleLowerCase();
  const characters = state.characters.filter(character =>
    !query || `${character.name} ${character.creator} ${(character.tags || []).join(" ")}`.toLocaleLowerCase().includes(query)
  );
  if (!state.characters.length) {
    content.innerHTML = `<div class="empty"><div><div class="empty-art">✦</div><h2>书架还在等第一位角色</h2><p>把 SillyTavern 的 PNG 或 JSON 角色卡放进左侧 Inbox。文件稳定后，Shelf 会自动解析并收录。</p></div></div>`;
    return;
  }
  content.innerHTML = `
    <div class="collection-meta"><h2>${query ? "搜索结果" : "最近收录"}</h2><span>${characters.length} 位角色</span></div>
    <div class="grid">${characters.map(character => `
      <article class="card" tabindex="0" role="button" data-id="${character.id}" aria-label="打开 ${escapeHTML(character.name)}">
        <div class="cover">${avatar(character)}<span class="format-badge">${escapeHTML(character.sourceFormat)}</span></div>
        <h3 title="${escapeHTML(character.name)}">${escapeHTML(character.name)}</h3>
        <p>${escapeHTML(character.creator || "未知创作者")}</p>
      </article>`).join("")}</div>`;
  content.querySelectorAll(".card").forEach(card => {
    const open = () => openDetail(card.dataset.id);
    card.addEventListener("click", open);
    card.addEventListener("keydown", event => {
      if (event.key === "Enter" || event.key === " ") { event.preventDefault(); open(); }
    });
  });
}

async function loadLibrary({ quiet = false } = {}) {
  try {
    const response = await fetch("/api/characters", { cache: "no-store" });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    state.characters = await response.json();
    showNotice();
    render();
  } catch (error) {
    showNotice(`无法读取角色库：${error.message}`);
    if (!quiet) content.innerHTML = `<div class="empty"><div><h2>角色库暂时不可用</h2><p>后台服务没有响应，请稍后重试。</p></div></div>`;
  }
}

async function loadStatus() {
  try {
    const response = await fetch("/api/status", { cache: "no-store" });
    if (!response.ok) return;
    const status = await response.json();
    document.querySelector("#inbox-path").textContent = status.paths.inbox;
    document.querySelector("#inbox-path").title = status.paths.inbox;
    document.querySelector("#scanner-label").textContent = status.scanner.running ?
      (status.scanner.pending ? `正在观察 ${status.scanner.pending} 个文件` : "正在监视收件目录") : "扫描器已停止";
    document.querySelector("#autostart-setting").classList.toggle("hidden", !status.desktop.available);
    document.querySelector("#autostart").checked = Boolean(status.desktop.autoStart);
    document.querySelector("#open-inbox").textContent = status.desktop.available ? "打开收件目录" : "复制收件目录";
    document.querySelector("#open-inbox").dataset.desktop = status.desktop.available ? "true" : "false";
    if (status.scanner.lastError) showNotice(`${status.scanner.lastErrorFile} 暂未收录：${status.scanner.lastError}`);
  } catch (_) { /* The next poll will retry quietly. */ }
}

function openDetail(id) {
  const character = state.characters.find(item => item.id === id);
  if (!character) return;
  state.selected = id;
  const imported = new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(character.importedAt));
  const size = character.sourceSize < 1024 * 1024 ? `${Math.ceil(character.sourceSize / 1024)} KB` : `${(character.sourceSize / 1024 / 1024).toFixed(1)} MB`;
  const features = [
    character.hasWorldbook && "包含 Worldbook", character.hasRegex && "包含 Regex",
    character.hasInteractive && "包含交互扩展", character.hasExtensions && "包含扩展数据"
  ].filter(Boolean);
  document.querySelector("#detail-content").innerHTML = `<div class="detail">
    <div class="detail-cover">${avatar(character, true)}</div>
    <div class="detail-info">
      <div class="eyebrow">CHARACTER CARD</div>
      <h2>${escapeHTML(character.name)}</h2>
      <p class="creator">${escapeHTML(character.creator ? `by ${character.creator}` : "未知创作者")}</p>
      <div class="tags">${(character.tags || []).length ? character.tags.map(tag => `<span class="tag">${escapeHTML(tag)}</span>`).join("") : '<span class="tag">暂无标签</span>'}</div>
      <div class="facts">
        <div class="fact"><small>格式</small><span>${escapeHTML(character.sourceFormat.toUpperCase())}${character.specVersion ? ` · ${escapeHTML(character.specVersion)}` : ""}</span></div>
        <div class="fact"><small>收录时间</small><span>${escapeHTML(imported)}</span></div>
        <div class="fact"><small>原始文件</small><span title="${escapeHTML(character.sourceFilename)}">${escapeHTML(character.sourceFilename)}</span></div>
        <div class="fact"><small>文件大小</small><span>${size}</span></div>
      </div>
      <div class="features">${features.length ? features.map(feature => `<span class="yes">${feature}</span>`).join("") : "未检测到附加功能"}</div>
      <div class="detail-actions">
        <a class="button" href="${character.sourceUrl}?download=1">导出原始卡</a>
        <button class="button danger" id="delete-character" type="button">移至 Shelf Trash</button>
      </div>
    </div>
  </div>`;
  document.querySelector("#delete-character").addEventListener("click", () => deleteCharacter(character));
  dialog.showModal();
}

async function deleteCharacter(character) {
  if (!confirm(`确认从角色库移除“${character.name}”？\n\n原始卡会移入 Tavern Shelf 自己的 Trash，不会触碰其他文件。`)) return;
  const response = await fetch(`/api/characters/${character.id}`, { method: "DELETE" });
  if (!response.ok) { showNotice("删除失败，原始卡仍被保留。"); return; }
  dialog.close();
  await loadLibrary();
}

document.querySelector("#search").addEventListener("input", event => { state.query = event.target.value; render(); });
document.querySelector("#refresh").addEventListener("click", () => loadLibrary());
document.querySelector("#close-detail").addEventListener("click", () => dialog.close());
dialog.addEventListener("click", event => { if (event.target === dialog) dialog.close(); });
document.querySelector("#open-inbox").addEventListener("click", async event => {
  const path = document.querySelector("#inbox-path").textContent;
  if (event.currentTarget.dataset.desktop === "true") {
    const response = await fetch("/api/desktop/open-inbox", { method: "POST" });
    if (!response.ok) showNotice("无法打开收件目录，请复制路径后手动打开。");
    return;
  }
  try { await navigator.clipboard.writeText(path); event.currentTarget.textContent = "已复制"; }
  catch (_) { showNotice(`Inbox：${path}`); }
  setTimeout(() => { event.currentTarget.textContent = "复制收件目录"; }, 1400);
});
document.querySelector("#autostart").addEventListener("change", async event => {
  const enabled = event.currentTarget.checked;
  const response = await fetch("/api/desktop/autostart", {
    method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ enabled })
  });
  if (!response.ok) { event.currentTarget.checked = !enabled; showNotice("无法更新开机自启设置。"); }
});

loadLibrary();
loadStatus();
setInterval(() => { loadLibrary({ quiet: true }); loadStatus(); }, 3000);
