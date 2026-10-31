/* Vorticis Monitor — interface (sem dependências externas). */
"use strict";
const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const brl = v => (v || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
const num = v => (v || 0).toLocaleString("pt-BR", { maximumFractionDigits: 4 });
const dt = s => { if (!s) return ""; const d = new Date(s); return isNaN(d) ? s : d.toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" }); };
const dia = s => { if (!s) return ""; const d = new Date(s); return isNaN(d) ? s : d.toLocaleDateString("pt-BR"); };
const docFmt = d => { d = String(d || ""); return d.length === 14 ? d.replace(/^(..)(...)(...)(....)(..)$/, "$1.$2.$3/$4-$5") : d.length === 11 ? d.replace(/^(...)(...)(...)(..)$/, "$1.$2.$3-$4") : d; };
const iso = d => new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 10);
const ST = { AUTORIZADA: "Autorizada", CANCELADA: "Cancelada", DENEGADA: "Denegada", REJEITADA: "Rejeitada", PENDENTE: "Pendente", CONTINGENCIA: "Contingência", BLOQUEADA: "Bloqueada", RESUMO: "Resumo (aguarda XML)", INUTILIZADA: "Inutilizada" };
const MOD = { NFE: "NF-e", NFCE: "NFC-e", CTE: "CT-e", MDFE: "MDF-e", NFSE: "NFS-e" };
const MANIF = { CIENCIA: "Ciência", CONFIRMADA: "Confirmada", DESCONHECIDA: "Desconhecida", NAO_REALIZADA: "Não realizada" };
const stTag = s => `<span class="st ${esc(s)}">${esc(ST[s] || s)}</span>`;
const svTag = s => `<span class="st sv-${esc(s)}">${esc({ ERRO: "Erro", ALERTA: "Atenção", INFO: "Info" }[s] || s)}</span>`;

const IC = {
  painel: '<path d="M4 13h6V4H4zM14 20h6v-9h-6zM4 20h6v-3H4zM14 7h6V4h-6z"/>',
  docs: '<path d="M7 3h7l5 5v13H7z"/><path d="M14 3v5h5M10 13h6M10 17h6"/>',
  saida: '<path d="M7 3h7l5 5v13H7z"/><path d="M10 15l3-3 3 3M13 12v7"/>',
  entrada: '<path d="M7 3h7l5 5v13H7z"/><path d="M10 14l3 3 3-3M13 17v-6"/>',
  servico: '<path d="M14.7 6.3a4 4 0 01-5.4 5.4L4 17v3h3l5.3-5.3a4 4 0 015.4-5.4l-2.6 2.6-2.4-.6-.6-2.4z"/>',
  transporte: '<path d="M3 6h11v10H3zM14 10h4l3 3v3h-7"/><circle cx="7" cy="18" r="2"/><circle cx="17" cy="18" r="2"/>',
  empresa: '<path d="M4 21V5l8-3 8 3v16M9 21v-5h6v5M8 9h2M14 9h2M8 13h2M14 13h2"/>',
  config: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 00.3 1.8l.1.1a2 2 0 11-2.8 2.8l-.1-.1a1.7 1.7 0 00-1.8-.3 1.7 1.7 0 00-1 1.5V21a2 2 0 11-4 0v-.1a1.7 1.7 0 00-1.1-1.5 1.7 1.7 0 00-1.8.3l-.1.1a2 2 0 11-2.8-2.8l.1-.1a1.7 1.7 0 00.3-1.8 1.7 1.7 0 00-1.5-1H3a2 2 0 110-4h.1a1.7 1.7 0 001.5-1.1 1.7 1.7 0 00-.3-1.8l-.1-.1a2 2 0 112.8-2.8l.1.1a1.7 1.7 0 001.8.3H9a1.7 1.7 0 001-1.5V3a2 2 0 114 0v.1a1.7 1.7 0 001 1.5 1.7 1.7 0 001.8-.3l.1-.1a2 2 0 112.8 2.8l-.1.1a1.7 1.7 0 00-.3 1.8V9a1.7 1.7 0 001.5 1H21a2 2 0 110 4h-.1a1.7 1.7 0 00-1.5 1z"/>',
  ok: '<path d="M20 6L9 17l-5-5"/>', x: '<path d="M18 6L6 18M6 6l12 12"/>', relogio: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  alerta: '<path d="M12 9v4m0 4h.01M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z"/>', vazio: '<path d="M3 7l9-4 9 4-9 4zM3 7v10l9 4 9-4V7"/>',
  pdf: '<path d="M7 3h7l5 5v13H7z"/><path d="M14 3v5h5"/>', xml: '<path d="M8 9l-4 3 4 3M16 9l4 3-4 3M13 6l-2 12"/>', busca: '<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/>',
  chave: '<circle cx="8" cy="15" r="4"/><path d="M11 12l9-9M16 7l3 3"/>', mais: '<path d="M12 5v14M5 12h14"/>',
};
const svg = n => `<svg viewBox="0 0 24 24">${IC[n] || ""}</svg>`;

const MENU = [
  { grupo: "Visão geral" }, { v: "painel", t: "Painel", i: "painel", sub: "Resumo dos documentos no período" },
  { grupo: "Documentos" },
  { v: "documentos", t: "Todos os documentos", i: "docs", sub: "Consulta geral de documentos fiscais" },
  { v: "emitidos", t: "Emitidos", i: "saida", sub: "Documentos emitidos pela empresa" },
  { v: "recebidos", t: "Recebidos", i: "entrada", sub: "NF-e destinadas à empresa e manifestação" },
  { v: "servicos", t: "Serviços (NFS-e)", i: "servico", sub: "NFS-e prestadas e tomadas" },
  { v: "transporte", t: "Transporte", i: "transporte", sub: "CT-e e MDF-e" },
  { grupo: "Cadastros" },
  { v: "empresas", t: "Empresas", i: "empresa", sub: "Empresas monitoradas e certificados digitais" },
  { v: "config", t: "Configurações", i: "config", sub: "Usuários, parâmetros e integração" },
];

const hoje = new Date();
const estado = {
  token: localStorage.getItem("vtx_token") || "", empresa: localStorage.getItem("vtx_emp") || "", view: "painel", empresas: [],
  de: iso(new Date(hoje.getFullYear(), hoje.getMonth(), 1)), ate: iso(hoje), preset: "mes",
  busca: "", status: "", modelo: "", direcao: "", pagina: 1, sel: new Set(), abaCfg: "usuarios",
};
let es = null;

async function api(path, opt = {}) {
  opt.headers = Object.assign({ Authorization: "Bearer " + estado.token }, opt.headers || {});
  if (opt.json !== undefined) { opt.body = JSON.stringify(opt.json); opt.headers["Content-Type"] = "application/json"; }
  const r = await fetch("/api/v1" + path, opt);
  if (r.status === 401 && !path.startsWith("/auth")) { sair(); throw new Error("Sessão expirada"); }
  if (opt.blob) { if (!r.ok) throw new Error((await r.json().catch(() => ({}))).erro || r.statusText); return r; }
  const ct = r.headers.get("content-type") || "";
  const body = ct.includes("json") ? await r.json() : await r.text();
  if (!r.ok) throw new Error(body.erro || body || r.statusText);
  return body;
}
function toast(msg, cor) {
  const t = document.createElement("div"); t.className = "toast"; if (cor) t.style.borderLeftColor = cor;
  t.innerHTML = msg; $("#toasts").appendChild(t); setTimeout(() => t.remove(), 5500);
}
const link = p => "/api/v1" + p + (p.includes("?") ? "&" : "?") + "token=" + encodeURIComponent(estado.token);
async function baixar(resp, nomePadrao) {
  const cd = resp.headers.get("content-disposition") || "";
  const nome = (cd.match(/filename="([^"]+)"/) || [])[1] || nomePadrao;
  const url = URL.createObjectURL(await resp.blob());
  const a = Object.assign(document.createElement("a"), { href: url, download: nome }); document.body.appendChild(a); a.click(); a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 30000);
}

/* ---------- login ---------- */
async function iniciar() {
  const e = await fetch("/api/v1/auth/estado").then(r => r.json());
  if (e.precisa_setup) {
    $("#login").classList.remove("hidden"); $("#setup-campos").classList.remove("hidden");
    $("#login-sub").textContent = "Primeiro acesso: crie o usuário administrador do programa"; $("#btn-login").textContent = "Criar administrador";
    $("#form-login").dataset.setup = "1"; return;
  }
  if (!estado.token) { $("#login").classList.remove("hidden"); return; }
  try { await carregarEmpresas(); entrar(); } catch { $("#login").classList.remove("hidden"); }
}
$("#form-login").onsubmit = async ev => {
  ev.preventDefault();
  const f = new FormData(ev.target), setup = ev.target.dataset.setup === "1";
  try {
    const r = await api(setup ? "/auth/setup" : "/auth/login", { method: "POST", json: { login: f.get("login"), senha: f.get("senha"), nome: f.get("nome") } });
    estado.token = r.token; localStorage.setItem("vtx_token", r.token);
    $("#login").classList.add("hidden"); await carregarEmpresas(); entrar();
    if (!estado.empresas.length) { mostrar("empresas"); novaEmpresa(); }
  } catch (e) { $("#login-erro").textContent = e.message; }
};
function sair() { localStorage.removeItem("vtx_token"); estado.token = ""; location.reload(); }
$("#sair").onclick = sair;
$("#recolher").onclick = () => { $("#side").classList.toggle("mini"); localStorage.setItem("vtx_mini", $("#side").classList.contains("mini") ? "1" : ""); };
if (localStorage.getItem("vtx_mini")) $("#side").classList.add("mini");

function entrar() {
  $("#app").classList.remove("hidden");
  $("#menu").innerHTML = MENU.map(m => m.grupo ? `<div class="grupo">${m.grupo}</div>` : `<a data-v="${m.v}" title="${m.t}">${svg(m.i)}<span>${m.t}</span></a>`).join("");
  $$("#menu a").forEach(a => a.onclick = () => { estado.pagina = 1; estado.status = ""; estado.modelo = ""; mostrar(a.dataset.v); });
  conectarSSE(); mostrar(estado.view);
}
async function carregarEmpresas() {
  estado.empresas = await api("/empresas");
  const sel = $("#empresa-sel");
  sel.innerHTML = `<option value="">Todas as empresas</option>` + estado.empresas.map(e => `<option value="${e.id}">${esc(e.razao_social)}</option>`).join("");
  sel.value = estado.empresa;
  const sem = estado.empresas.filter(e => !e.tem_certificado), b = $("#banner");
  if (!estado.empresas.length) { b.innerHTML = `${svg("empresa")} Nenhuma empresa cadastrada. <a onclick="mostrar('empresas');novaEmpresa()">Cadastrar empresa pelo CNPJ ou certificado</a>`; b.classList.remove("hidden"); }
  else if (sem.length) { b.innerHTML = `${svg("chave")} ${sem.length} empresa(s) sem certificado digital — necessário para consultar a SEFAZ e baixar os XMLs recebidos. <a onclick="mostrar('empresas')">Incluir certificado</a>`; b.classList.remove("hidden"); }
  else b.classList.add("hidden");
}
$("#empresa-sel").onchange = e => { estado.empresa = e.target.value; localStorage.setItem("vtx_emp", estado.empresa); estado.pagina = 1; mostrar(estado.view); };
$("#upload").onchange = async ev => {
  const fd = new FormData(); [...ev.target.files].forEach(f => fd.append("arquivo", f, f.name));
  try {
    const r = await api("/documentos/importar", { method: "POST", body: fd });
    const err = r.resultados.filter(x => x.status === "ERRO").length;
    toast(`<b>${r.processados}</b> arquivo(s) importado(s)${err ? ` · <b style="color:var(--er)">${err} com erro</b>` : ""}`);
    mostrar(estado.view, true);
  } catch (e) { toast(esc(e.message), "var(--er)"); }
  ev.target.value = "";
};

/* ---------- tempo real ---------- */
let tRefresh;
function conectarSSE() {
  if (es) es.close();
  es = new EventSource(link("/eventos/stream"));
  const rt = $("#rt");
  es.addEventListener("conectado", () => { rt.classList.add("on"); rt.lastElementChild.textContent = "tempo real"; });
  es.onerror = () => { rt.classList.remove("on"); rt.lastElementChild.textContent = "reconectando"; };
  const refresh = () => { clearTimeout(tRefresh); tRefresh = setTimeout(() => { if (VIEWS_DOC.includes(estado.view) && $("#drawer").classList.contains("hidden")) mostrar(estado.view, true); }, 800); };
  es.addEventListener("documento.novo", e => { const d = JSON.parse(e.data).dados; toast(`${MOD[d.modelo] || d.modelo} nº ${esc(d.numero)} recebido ${stTag(d.status)}`); refresh(); });
  es.addEventListener("documento.status", refresh);
}

/* ---------- navegação ---------- */
const VIEWS = {};
const VIEWS_DOC = ["painel", "documentos", "emitidos", "recebidos", "servicos", "transporte"];
async function mostrar(v, silencioso) {
  estado.view = v;
  const m = MENU.find(x => x.v === v) || MENU[1];
  $("#titulo").textContent = m.t; $("#subtitulo").textContent = m.sub;
  $$("#menu a").forEach(a => a.classList.toggle("ativo", a.dataset.v === v));
  const el = $("#view");
  if (!silencioso) el.innerHTML = `<p class="muted">Carregando…</p>`;
  try { await (VIEWS[v] || VIEWS.painel)(el); } catch (e) { el.innerHTML = `<div class="card erro">${esc(e.message)}</div>`; }
  atualizarSelecao();
}

/* ---------- filtro de período (todas as consultas) ---------- */
const PRESETS = {
  hoje: () => [iso(hoje), iso(hoje)],
  d7: () => [iso(new Date(Date.now() - 6 * 864e5)), iso(hoje)],
  d30: () => [iso(new Date(Date.now() - 29 * 864e5)), iso(hoje)],
  mes: () => [iso(new Date(hoje.getFullYear(), hoje.getMonth(), 1)), iso(hoje)],
  mesant: () => [iso(new Date(hoje.getFullYear(), hoje.getMonth() - 1, 1)), iso(new Date(hoje.getFullYear(), hoje.getMonth(), 0))],
  ano: () => [iso(new Date(hoje.getFullYear(), 0, 1)), iso(hoje)],
};
const ROT_PRESET = { hoje: "Hoje", d7: "7 dias", d30: "30 dias", mes: "Este mês", mesant: "Mês anterior", ano: "Este ano" };
function filtroHTML(extras = "") {
  return `<div class="filtros">
    <label>De<input type="date" id="f-de" value="${estado.de}"></label>
    <label>Até<input type="date" id="f-ate" value="${estado.ate}"></label>
    <div class="presets">${Object.keys(PRESETS).map(k => `<button type="button" data-p="${k}" class="${estado.preset === k ? "on" : ""}">${ROT_PRESET[k]}</button>`).join("")}</div>
    ${extras}
    <button class="btn primary" id="f-ok">${svg("busca")} Consultar</button>
  </div>`;
}
function ligarFiltro(el, extrasLer) {
  const aplicar = () => {
    estado.de = $("#f-de", el).value; estado.ate = $("#f-ate", el).value;
    if (estado.de && estado.ate && estado.de > estado.ate) { toast("A data inicial é posterior à final", "var(--er)"); return; }
    if (extrasLer) extrasLer();
    estado.pagina = 1; mostrar(estado.view, true);
  };
  $$(".presets button", el).forEach(b => b.onclick = () => { [estado.de, estado.ate] = PRESETS[b.dataset.p](); estado.preset = b.dataset.p; $("#f-de", el).value = estado.de; $("#f-ate", el).value = estado.ate; aplicar(); });
  $("#f-de", el).onchange = $("#f-ate", el).onchange = () => { estado.preset = ""; };
  $("#f-ok", el).onclick = aplicar;
  $$(".filtros input:not([type=date])", el).forEach(i => i.onkeydown = e => { if (e.key === "Enter") aplicar(); });
}
function qsBase(extra = {}) {
  const q = new URLSearchParams({ de: estado.de, ate: estado.ate, ...extra });
  if (estado.empresa) q.set("empresa_id", estado.empresa);
  for (const [k, v] of [...q]) if (!v) q.delete(k);
  return q;
}

/* ---------- Painel ---------- */
VIEWS.painel = async el => {
  const [r, lista] = await Promise.all([api("/dashboard/resumo?" + qsBase()), api("/documentos?" + qsBase({ por_pagina: 2000 }))]);
  const s = r.por_status || {};
  const kpi = (st, ic, cor) => `<div class="card kpi" onclick="estado.status='${st}';mostrar('documentos')"><div class="ic ${cor}">${svg(ic)}</div><div><div class="n">${s[st] || 0}</div><div class="l">${ST[st]}</div></div></div>`;
  const porDia = {};
  lista.itens.forEach(d => { const k = (d.dh_emissao || "").slice(0, 10); if (k) porDia[k] = (porDia[k] || 0) + 1; });
  const dias = []; for (let d = new Date(estado.de + "T12:00"); iso(d) <= estado.ate && dias.length < 120; d.setDate(d.getDate() + 1)) dias.push(iso(d));
  const mx = Math.max(1, ...dias.map(d => porDia[d] || 0));
  el.innerHTML = filtroHTML() + `
  <div class="grid g4">${kpi("AUTORIZADA", "ok", "c-ok")}${kpi("CANCELADA", "x", "c-gr")}${kpi("PENDENTE", "relogio", "c-in")}${kpi("CONTINGENCIA", "alerta", "c-wa")}${kpi("REJEITADA", "x", "c-er")}${kpi("DENEGADA", "alerta", "c-er")}</div>
  <div class="grid g2" style="margin-top:16px">
    <div class="card"><h3>Documentos por dia</h3><div class="barra">${dias.map(d => `<div style="height:${(porDia[d] || 0) / mx * 100}%" data-t="${dia(d + "T12:00")}: ${porDia[d] || 0}"></div>`).join("")}</div>
      <div class="muted" style="display:flex;justify-content:space-between;font-size:12px;margin-top:4px"><span>${dia(estado.de + "T12:00")}</span><span>${r.total} documento(s)</span><span>${dia(estado.ate + "T12:00")}</span></div></div>
    <div class="card"><h3>Movimento no período</h3><div class="kv">
      <b>Emitidos (autorizados)</b><span><b>${brl(r.valor_por_direcao?.S)}</b></span><b>Recebidos (autorizados)</b><span><b>${brl(r.valor_por_direcao?.E)}</b></span>
      ${Object.entries(r.por_modelo || {}).map(([k, v]) => `<b>${MOD[k] || k}</b><span>${v} documento(s)</span>`).join("")}
      <b>Sem manifestação conclusiva</b><span><a onclick="mostrar('recebidos')">${r.sem_manifestacao} NF-e recebida(s)</a></span></div></div>
  </div>
  <h3 style="margin:22px 0 10px">Últimos documentos do período</h3>${gridDocs(lista.itens.slice(0, 10), false)}`;
  ligarFiltro(el); ligarGrid(el);
};

/* ---------- grid de documentos ---------- */
function gridDocs(itens, comSelecao = true, opts = {}) {
  if (!itens.length) return `<div class="card vazio">${svg("vazio")}<div>Nenhum documento no período informado.</div><div style="font-size:12.5px">Ajuste as datas ou importe XMLs pelo botão <b>Importar</b>.</div></div>`;
  const todos = comSelecao && itens.every(d => estado.sel.has(d.chave));
  return `<div class="tbl"><table><thead><tr>${comSelecao ? `<th class="chk"><input type="checkbox" id="sel-todos" ${todos ? "checked" : ""} title="Selecionar página"></th>` : ""}
  <th>Situação</th><th>Documento</th><th>Emissão</th><th>${opts.parte || "Emitente / Destinatário"}</th><th class="n">Valor</th>${opts.manif ? "<th>Manifestação</th>" : ""}<th>Chave</th></tr></thead><tbody>
  ${itens.map(d => `<tr data-chave="${esc(d.chave)}" class="${estado.sel.has(d.chave) ? "sel" : ""}">
    ${comSelecao ? `<td class="chk"><input type="checkbox" class="sel-doc" ${estado.sel.has(d.chave) ? "checked" : ""}></td>` : ""}
    <td>${stTag(d.status)}</td>
    <td><div class="doc-nome">${MOD[d.modelo] || esc(d.modelo)} ${esc(d.numero)}</div><div class="doc-sub">${d.direcao === "E" ? "Recebido" : "Emitido"}${d.serie ? " · série " + esc(d.serie) : ""}</div></td>
    <td>${dt(d.dh_emissao)}</td>
    <td><div class="doc-nome">${esc(d.direcao === "E" ? d.emit_nome : d.dest_nome) || "—"}</div><div class="doc-sub">${docFmt(d.direcao === "E" ? d.emit_doc : d.dest_doc)}</div></td>
    <td class="n"><b>${brl(d.valor_total)}</b></td>
    ${opts.manif ? `<td>${d.manifestacao ? esc(MANIF[d.manifestacao] || d.manifestacao) : `<span class="muted">Pendente</span>`}</td>` : ""}
    <td class="doc-sub" style="font-family:Consolas,monospace;font-size:11.5px">${esc(d.chave.slice(0, 22))}<br>${esc(d.chave.slice(22))}</td></tr>`).join("")}</tbody></table></div>`;
}
function ligarGrid(el) {
  $$("tbody tr[data-chave]", el).forEach(tr => tr.onclick = e => {
    const c = tr.dataset.chave;
    if (e.target.classList.contains("sel-doc")) { e.target.checked ? estado.sel.add(c) : estado.sel.delete(c); tr.classList.toggle("sel", e.target.checked); atualizarSelecao(); return; }
    abrirDoc(c);
  });
  const t = $("#sel-todos", el);
  if (t) t.onchange = () => { $$("tbody tr[data-chave]", el).forEach(tr => { t.checked ? estado.sel.add(tr.dataset.chave) : estado.sel.delete(tr.dataset.chave); tr.classList.toggle("sel", t.checked); $(".sel-doc", tr).checked = t.checked; }); atualizarSelecao(); };
}
function atualizarSelecao() {
  const n = estado.sel.size, b = $("#selecao");
  b.classList.toggle("hidden", !n || !VIEWS_DOC.includes(estado.view));
  $("#sel-qtd").textContent = `${n} documento${n > 1 ? "s" : ""} selecionado${n > 1 ? "s" : ""}`;
}
$("#sel-limpar").onclick = () => { estado.sel.clear(); mostrar(estado.view, true); };
$$("#selecao [data-exp]").forEach(b => b.onclick = () => exportar([...estado.sel], b.dataset.exp));
async function exportar(chaves, formato) {
  if (!chaves.length) return;
  toast(`Gerando arquivo com ${chaves.length} documento(s)…`);
  try { await baixar(await api("/documentos/exportar", { method: "POST", json: { chaves, formato }, blob: true }), "documentos.zip"); toast("Exportação concluída — arquivo salvo em Downloads", "var(--ok)"); }
  catch (e) { toast(esc(e.message), "var(--er)"); }
}

/* ---------- consultas de documentos ---------- */
async function consultaDocs(el, fixo, opts = {}) {
  const extras = `
    <label class="busca">Pesquisar<input id="f-q" value="${esc(estado.busca)}" placeholder="Número, chave, CNPJ ou nome"></label>
    ${opts.modelos ? `<label>Modelo<select id="f-mod"><option value="">${opts.modelos.length > 1 ? "Todos" : MOD[opts.modelos[0]]}</option>${opts.modelos.length > 1 ? opts.modelos.map(m => `<option value="${m}" ${estado.modelo === m ? "selected" : ""}>${MOD[m]}</option>`).join("") : ""}</select></label>` : ""}
    ${opts.direcao ? `<label>Movimento<select id="f-dir"><option value="">Todos</option><option value="S" ${estado.direcao === "S" ? "selected" : ""}>Emitidos</option><option value="E" ${estado.direcao === "E" ? "selected" : ""}>Recebidos</option></select></label>` : ""}
    <label>Situação<select id="f-st"><option value="">Todas</option>${Object.entries(ST).map(([k, v]) => `<option value="${k}" ${estado.status === k ? "selected" : ""}>${v}</option>`).join("")}</select></label>`;
  const q = qsBase({ por_pagina: 50, pagina: estado.pagina, q: estado.busca, status: estado.status, ...fixo });
  if (opts.modelos && opts.modelos.length > 1 && estado.modelo) q.set("modelo", estado.modelo);
  if (opts.direcao && estado.direcao) q.set("direcao", estado.direcao);
  const d = await api("/documentos?" + q);
  const pags = Math.max(1, Math.ceil(d.total / 50));
  el.innerHTML = (opts.antes || "") + filtroHTML(extras) + `
  <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px"><span class="muted">${d.total} documento(s) entre ${dia(estado.de + "T12:00")} e ${dia(estado.ate + "T12:00")}</span>
  ${d.total ? `<button class="btn sm" id="sel-filtro">Selecionar todos os ${d.total}</button>` : ""}</div>
  ${gridDocs(d.itens, true, opts)}
  <div class="paginacao"><button class="btn sm" ${estado.pagina <= 1 ? "disabled" : ""} id="pg-ant">‹ Anterior</button><span class="muted">Página ${estado.pagina} de ${pags}</span><button class="btn sm" ${estado.pagina >= pags ? "disabled" : ""} id="pg-prox">Próxima ›</button></div>`;
  ligarFiltro(el, () => { estado.busca = $("#f-q").value.trim(); estado.status = $("#f-st").value; if ($("#f-mod")) estado.modelo = $("#f-mod").value; if ($("#f-dir")) estado.direcao = $("#f-dir").value; });
  ligarGrid(el);
  $("#pg-ant").onclick = () => { estado.pagina--; mostrar(estado.view, true); };
  $("#pg-prox").onclick = () => { estado.pagina++; mostrar(estado.view, true); };
  const sf = $("#sel-filtro");
  if (sf) sf.onclick = async () => { q.set("por_pagina", 5000); q.set("pagina", 1); const all = await api("/documentos?" + q); all.itens.forEach(x => estado.sel.add(x.chave)); mostrar(estado.view, true); };
  if (opts.depois) opts.depois(el);
}
VIEWS.documentos = el => consultaDocs(el, {}, { modelos: ["NFE", "NFCE", "CTE", "MDFE", "NFSE"], direcao: true });
VIEWS.emitidos = el => consultaDocs(el, { direcao: "S", modelo: estado.modelo || "" }, { modelos: ["NFE", "NFCE", "CTE", "MDFE", "NFSE"], parte: "Destinatário" });
VIEWS.servicos = el => consultaDocs(el, { modelo: "NFSE" }, { modelos: ["NFSE"], direcao: true, parte: "Prestador / Tomador" });
// Transporte: alterna entre CT-e e MDF-e pelo filtro de modelo.
VIEWS.transporte = async el => {
  if (!estado.modelo) estado.modelo = "CTE";
  await consultaDocs(el, { modelo: estado.modelo }, { modelos: ["CTE", "MDFE"], direcao: true });
  const s = $("#f-mod"); if (s) { s.firstElementChild.remove(); s.value = estado.modelo; }
};
VIEWS.recebidos = async el => {
  const nsu = await api("/captura/status").catch(() => []);
  const emps = estado.empresas.filter(e => !estado.empresa || String(e.id) === estado.empresa);
  const antes = `<div class="grid g2" style="margin-bottom:16px">${emps.map(e => { const n = nsu.find(x => x.empresa_id === e.id) || {}; return `<div class="card" style="display:flex;gap:14px;align-items:center">
    <div class="kpi"><div class="ic ${e.tem_certificado ? "c-pr" : "c-wa"}">${svg(e.tem_certificado ? "entrada" : "chave")}</div></div>
    <div style="flex:1"><b>${esc(e.razao_social)}</b><div class="doc-sub">${e.tem_certificado ? `Busca automática na SEFAZ · última consulta ${dt(n.updated_at) || "ainda não realizada"}${n.ultimo_motivo ? " · " + esc(n.ultimo_motivo) : ""}` : "Inclua o certificado digital para baixar as notas destinadas à empresa"}</div></div>
    ${e.tem_certificado ? `<button class="btn sm" onclick="capturar(${e.id})">Buscar agora</button>` : `<button class="btn sm" onclick="mostrar('empresas')">Incluir certificado</button>`}</div>`; }).join("")}</div>`;
  await consultaDocs(el, { direcao: "E", modelo: "NFE" }, { modelos: ["NFE"], parte: "Emitente", manif: true, antes });
};
async function capturar(id) {
  try { const r = await api(`/captura/${id}/executar?forcar=1`, { method: "POST" }); toast(`${r.documentos} documento(s) recebido(s) · ${esc(r.motivo)}`); mostrar("recebidos", true); }
  catch (e) { toast(esc(e.message), "var(--er)"); }
}

/* ---------- detalhe ---------- */
async function abrirDoc(chave) {
  const r = await api("/documentos/" + encodeURIComponent(chave));
  const d = r.documento, ocs = (r.ocorrencias || []).filter(o => o.severidade !== "INFO");
  $("#drawer-body").innerHTML = `<div style="display:flex;justify-content:space-between;align-items:flex-start;gap:12px">
    <div><div class="muted">${d.direcao === "E" ? "Documento recebido" : "Documento emitido"} · ${esc(d.tipo_operacao)}</div><h2 style="margin:2px 0 0">${MOD[d.modelo] || d.modelo} nº ${esc(d.numero)} ${stTag(d.status)}</h2></div>
    <button class="btn icone" onclick="fecharDrawer()" title="Fechar">${svg("x")}</button></div>
  <div class="acoes">
    ${d.resumo ? "" : `<button class="btn primary" onclick="window.open(link('/documentos/${chave}/pdf'),'_blank')">${svg("pdf")} Visualizar PDF</button>`}
    ${d.resumo ? "" : `<button class="btn" onclick="exportar(['${chave}'],'pdf')">Baixar PDF</button>`}
    <button class="btn" onclick="exportar(['${chave}'],'xml')">${svg("xml")} Baixar XML</button>
    ${d.empresa_id ? `<button class="btn" onclick="acao('${chave}','consultar-sefaz')">Consultar na SEFAZ</button>` : ""}
    ${d.direcao === "E" && d.modelo === "NFE" ? `<button class="btn" onclick="manifestar('${chave}')">Manifestar</button>` : ""}
  </div>
  ${d.resumo ? `<div class="aviso">Somente o resumo foi recebido. Registre a <b>Ciência da Operação</b> em "Manifestar" para baixar o XML completo.</div>` : ""}
  <div class="grid g2"><div class="card"><div class="kv">
    <b>Chave de acesso</b><span style="word-break:break-all;font-family:Consolas,monospace;font-size:12.5px">${esc(d.chave)}</span>
    <b>Emissão</b><span>${dt(d.dh_emissao)}</span><b>Autorização</b><span>${dt(d.dh_autorizacao)} ${esc(d.protocolo)}</span>
    <b>Retorno SEFAZ</b><span>${d.c_stat || ""} ${esc(d.x_motivo)}</span><b>Natureza</b><span>${esc(d.natureza)}</span>
    ${d.direcao === "E" && d.modelo === "NFE" ? `<b>Manifestação</b><span>${esc(MANIF[d.manifestacao] || "Pendente")}</span>` : ""}
  </div></div><div class="card"><div class="kv">
    <b>Emitente</b><span>${esc(d.emit_nome)}<br><span class="muted">${docFmt(d.emit_doc)} ${esc(d.emit_uf)}</span></span>
    <b>Destinatário</b><span>${esc(d.dest_nome) || "—"}<br><span class="muted">${docFmt(d.dest_doc)} ${esc(d.dest_uf)}</span></span>
    <b>Valor total</b><span><b>${brl(d.valor_total)}</b></span><b>ICMS / ICMS-ST</b><span>${brl(d.valor_icms)} / ${brl(d.valor_st)}</span>
    <b>IPI</b><span>${brl(d.valor_ipi)}</span><b>PIS / COFINS</b><span>${brl(d.valor_pis)} / ${brl(d.valor_cofins)}</span>
    ${d.valor_iss ? `<b>ISS / Retenções</b><span>${brl(d.valor_iss)} / ${brl(d.valor_retencoes)}</span>` : ""}</div></div></div>
  ${r.itens?.length ? `<h3 style="margin-top:20px">Itens (${r.itens.length})</h3><div class="tbl"><table><thead><tr><th>#</th><th>Produto</th><th>NCM</th><th>CFOP</th><th>CST</th><th class="n">Qtd</th><th class="n">Unitário</th><th class="n">Total</th><th class="n">ICMS</th></tr></thead><tbody>
    ${r.itens.map(i => `<tr><td>${i.n_item}</td><td><div class="doc-nome">${esc(i.descricao)}</div><div class="doc-sub">${esc(i.cod_prod)}</div></td><td>${esc(i.ncm)}</td><td>${esc(i.cfop)}</td><td>${esc(i.cst_icms)}</td><td class="n">${num(i.quantidade)} ${esc(i.unidade)}</td><td class="n">${brl(i.valor_unit)}</td><td class="n">${brl(i.valor_total)}</td><td class="n">${brl(i.v_icms)}</td></tr>`).join("")}</tbody></table></div>` : ""}
  ${ocs.length ? `<h3 style="margin-top:20px">Conferência fiscal (${ocs.length})</h3>${ocs.map(o => `<div class="oc ${o.severidade}"><div>${svTag(o.severidade)} <b>${esc(o.mensagem)}</b></div>
    <div class="meta">${o.item ? "Item " + o.item + " · " : ""}${esc(o.campo)}${o.esperado ? ` · esperado <b>${esc(o.esperado)}</b>` : ""}${o.informado ? ` · informado <b>${esc(o.informado)}</b>` : ""}</div>${o.sugestao ? `<div class="meta">${esc(o.sugestao)}</div>` : ""}</div>`).join("")}` : ""}
  ${r.eventos?.length ? `<h3 style="margin-top:20px">Eventos</h3><div class="tbl"><table><tbody>${r.eventos.map(e => `<tr><td>${esc(e.tp_evento)}</td><td>${esc(e.descricao)}</td><td>${dt(e.dh_evento)}</td><td>${e.c_stat} ${esc(e.x_motivo)}</td><td>${esc(e.protocolo)}</td></tr>`).join("")}</tbody></table></div>` : ""}`;
  $("#drawer").classList.remove("hidden");
}
function fecharDrawer() { $("#drawer").classList.add("hidden"); mostrar(estado.view, true); }
$("#drawer").onclick = e => { if (e.target.id === "drawer") fecharDrawer(); };
async function acao(chave, a) {
  try { const r = await api(`/documentos/${chave}/${a}`, { method: "POST" }); toast(`SEFAZ: ${r.c_stat} ${esc(r.x_motivo)}`); abrirDoc(chave); }
  catch (e) { toast(esc(e.message), "var(--er)"); }
}
function modal(html) { $("#modal-body").innerHTML = html; $("#modal").classList.remove("hidden"); }
function fecharModal() { $("#modal").classList.add("hidden"); }
$("#modal").onclick = e => { if (e.target.id === "modal") fecharModal(); };
document.addEventListener("keydown", e => { if (e.key === "Escape") { if (!$("#modal").classList.contains("hidden")) fecharModal(); else if (!$("#drawer").classList.contains("hidden")) fecharDrawer(); } });
function manifestar(chave) {
  modal(`<h3>Manifestação do destinatário</h3><form id="fm" class="grid"><label>Evento<select name="tipo"><option value="210210">Ciência da Operação (libera o download do XML)</option><option value="210200">Confirmação da Operação</option><option value="210220">Desconhecimento da Operação</option><option value="210240">Operação não Realizada</option></select></label>
  <label>Justificativa (obrigatória para "Operação não Realizada", 15 a 255 caracteres)<textarea name="just" rows="3"></textarea></label><div class="acoes"><button class="btn primary">Enviar à SEFAZ</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`);
  $("#fm").onsubmit = async ev => { ev.preventDefault(); const f = new FormData(ev.target);
    try { const r = await api(`/documentos/${chave}/manifestacao`, { method: "POST", json: { tipo: f.get("tipo"), justificativa: f.get("just") } }); toast(`Evento: ${r.evento_c_stat} ${esc(r.evento_x_motivo)}`); fecharModal(); abrirDoc(chave); }
    catch (e) { toast(esc(e.message), "var(--er)"); } };
}

/* ---------- Empresas ---------- */
VIEWS.empresas = async el => {
  await carregarEmpresas();
  el.innerHTML = `<div class="acoes" style="margin-top:0"><button class="btn primary" onclick="novaEmpresa()">${svg("mais")} Nova empresa</button></div>
  ${estado.empresas.length ? `<div class="grid g2">${estado.empresas.map(e => `<div class="card">
    <div style="display:flex;justify-content:space-between;gap:10px"><div><h3 style="margin:0">${esc(e.razao_social)}</h3><div class="doc-sub">${docFmt(e.cnpj)} · ${esc(e.uf)} · IE ${esc(e.ie || "não informada")}</div></div>
    <span class="st ${e.ambiente === 1 ? "AUTORIZADA" : "PENDENTE"}">${e.ambiente === 1 ? "Produção" : "Homologação"}</span></div>
    <div class="kv" style="margin-top:12px"><b>Regime</b><span>${esc({ LR: "Lucro Real", LP: "Lucro Presumido", SN: "Simples Nacional", MEI: "MEI" }[e.regime] || e.regime)}</span><b>Certificado</b><span id="cert-${e.id}" class="muted">verificando…</span></div>
    <div class="acoes"><button class="btn sm" onclick='editarEmpresa(${JSON.stringify(e).replace(/'/g, "&#39;")})'>Editar</button><button class="btn sm" onclick="certificadoEmpresa(${e.id})">${e.tem_certificado ? "Substituir certificado" : "Incluir certificado"}</button>
    ${e.tem_certificado ? `<button class="btn sm danger" onclick="removerCert(${e.id})">Remover certificado</button>` : ""}</div></div>`).join("")}</div>`
    : `<div class="card vazio">${svg("empresa")}<div>Nenhuma empresa cadastrada.</div><div style="font-size:12.5px">Informe apenas o CNPJ ou use o certificado digital: os dados são importados da Receita Federal e a IE da SEFAZ.</div></div>`}`;
  for (const e of estado.empresas) {
    api(`/empresas/${e.id}/certificado`).then(c => $("#cert-" + e.id).innerHTML = `<span style="color:var(--tx)">${esc(c.titular)}</span><br>válido até ${dia(c.valido_ate)} (${c.dias_restantes} dias)`)
      .catch(() => $("#cert-" + e.id).innerHTML = `<span class="st CONTINGENCIA">Sem certificado</span>`);
  }
};
let novaCert = null; // {arquivo, senha} escolhido no cadastro pelo certificado
function novaEmpresa() {
  novaCert = null;
  modal(`<h3>Nova empresa</h3><div class="passos"><button class="on" data-m="cnpj"><b>Pelo CNPJ</b><span class="muted">Dados da Receita Federal e IE pelo Sintegra/SEFAZ</span></button>
  <button data-m="cert"><b>Pelo certificado digital</b><span class="muted">Lê o CNPJ do e-CNPJ e consulta a IE na SEFAZ</span></button></div><div id="nova-passo"></div><div id="nova-form"></div>`);
  const passo = m => {
    $$(".passos button").forEach(b => b.classList.toggle("on", b.dataset.m === m)); $("#nova-form").innerHTML = "";
    $("#nova-passo").innerHTML = m === "cnpj"
      ? `<form id="fc" class="filtros" style="box-shadow:none"><label class="busca">CNPJ<input name="cnpj" placeholder="00.000.000/0000-00" required autofocus></label><button class="btn primary">${svg("busca")} Consultar</button></form>`
      : `<form id="fc" class="filtros" style="box-shadow:none"><label class="busca">Certificado A1 (.pfx/.p12)<input type="file" name="arquivo" accept=".pfx,.p12" required></label><label>Senha<input type="password" name="senha" required></label><button class="btn primary">${svg("chave")} Ler certificado</button></form>`;
    $("#fc").onsubmit = async ev => {
      ev.preventDefault(); const f = new FormData(ev.target), btn = $("#fc button"); btn.disabled = true; btn.textContent = "Consultando…";
      try {
        let dados, cert;
        if (m === "cnpj") dados = await api("/cadastro/cnpj/" + encodeURIComponent(f.get("cnpj").replace(/[^0-9A-Za-z]/g, "")));
        else { const r = await api("/cadastro/certificado", { method: "POST", body: f }); dados = r.dados; cert = r.certificado; novaCert = { arquivo: f.get("arquivo"), senha: f.get("senha") }; }
        formEmpresa($("#nova-form"), dados, cert);
      } catch (e) { toast(esc(e.message), "var(--er)"); }
      btn.disabled = false; btn.innerHTML = m === "cnpj" ? `${svg("busca")} Consultar` : `${svg("chave")} Ler certificado`;
    };
  };
  $$(".passos button").forEach(b => b.onclick = () => passo(b.dataset.m)); passo("cnpj");
}
function formEmpresa(el, d, cert, existente) {
  const ies = d.inscricoes || [];
  el.innerHTML = `${cert ? `<div class="aviso">${svg("chave")} Certificado de <b>${esc(cert.titular)}</b>, válido até ${dia(cert.valido_ate)}. Será incluído junto com a empresa.</div>` : ""}
  ${(d.fontes || []).length ? `<div class="doc-sub" style="margin:8px 0">Fontes: ${d.fontes.map(esc).join(" · ")}</div>` : ""}
  ${(d.avisos || []).map(a => `<div class="aviso">${esc(a).replace(/(https?:\/\/\S+)/, '<a onclick="window.open(\'$1\',\'_blank\')">$1</a>')}</div>`).join("")}
  <form id="fe"><div class="form" style="margin-top:12px">
    <label>CNPJ<input name="cnpj" value="${esc(d.cnpj)}" readonly></label>
    <label class="w2">Razão social<input name="razao_social" value="${esc(d.razao_social)}" required></label>
    <label>Situação na Receita<input value="${esc(d.situacao || "")}" readonly></label>
    <label>UF<input name="uf" maxlength="2" value="${esc(d.uf)}" required></label>
    <label>Município<input value="${esc(d.municipio || "")}" readonly></label>
    <label>Cód. IBGE<input name="cod_mun" value="${esc(d.cod_mun || "")}"></label>
    <label>Inscrição Estadual ${ies.length ? `<select id="ie-sel"><option value="">Digitar…</option>${ies.map(i => `<option value="${esc(i.ie)}" ${i.ie === d.ie ? "selected" : ""}>${esc(i.ie)} · ${esc(i.uf)} · ${i.ativa ? "ativa" : "inativa"} (${esc(i.fonte)})</option>`).join("")}</select>` : ""}<input name="ie" value="${esc(d.ie || "")}" placeholder="ou ISENTO"></label>
    <label>Regime tributário<select name="regime">${[["LR", "Lucro Real"], ["LP", "Lucro Presumido"], ["SN", "Simples Nacional"], ["MEI", "MEI"]].map(([k, l]) => `<option value="${k}" ${(d.regime || d.regime_sugerido) === k ? "selected" : ""}>${l}</option>`).join("")}</select></label>
    <label>Ambiente SEFAZ<select name="ambiente"><option value="1" ${d.ambiente !== 2 ? "selected" : ""}>Produção</option><option value="2" ${d.ambiente === 2 ? "selected" : ""}>Homologação</option></select></label>
    <label>Buscar notas recebidas automaticamente<select name="captura_automatica"><option value="1">Sim</option><option value="" ${existente && !d.captura_automatica ? "selected" : ""}>Não</option></select></label>
  </div>${d.endereco || d.logradouro ? `<div class="doc-sub">${esc([d.logradouro, d.numero, d.bairro, d.municipio, d.uf, d.cep].filter(Boolean).join(", "))}</div>` : ""}
  <div class="acoes"><button class="btn primary">${existente ? "Salvar alterações" : "Cadastrar empresa"}</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`;
  const sel = $("#ie-sel"); if (sel) sel.onchange = () => { if (sel.value) $("#fe [name=ie]").value = sel.value; };
  $("#fe").onsubmit = async ev => {
    ev.preventDefault(); const f = Object.fromEntries(new FormData(ev.target));
    const corpo = { ...f, ambiente: +f.ambiente, captura_automatica: !!f.captura_automatica, ciencia_automatica: !!existente?.ciencia_automatica, ativo: true, uf: f.uf.toUpperCase() };
    try {
      const e = await api(existente ? `/empresas/${existente.id}` : "/empresas", { method: existente ? "PUT" : "POST", json: corpo });
      if (novaCert && !existente) { const fd = new FormData(); fd.append("arquivo", novaCert.arquivo); fd.append("senha", novaCert.senha); await api(`/empresas/${e.id}/certificado`, { method: "POST", body: fd }); }
      fecharModal(); toast(existente ? "Empresa atualizada" : `Empresa ${esc(e.razao_social)} cadastrada`, "var(--ok)"); estado.view = "empresas"; mostrar("empresas");
      if (!novaCert && !existente) certificadoEmpresa(e.id);
    } catch (er) { toast(esc(er.message), "var(--er)"); }
  };
}
function editarEmpresa(e) {
  modal(`<h3>Editar empresa</h3><div class="acoes" style="margin-top:0"><button class="btn sm" id="atualizar-rf">${svg("busca")} Atualizar pela Receita/SEFAZ</button></div><div id="ed-form"></div>`);
  formEmpresa($("#ed-form"), { ...e, regime: e.regime }, null, e);
  $("#atualizar-rf").onclick = async () => {
    try { const d = await api("/cadastro/cnpj/" + e.cnpj); formEmpresa($("#ed-form"), { ...d, ambiente: e.ambiente, captura_automatica: e.captura_automatica, regime: e.regime }, null, e); toast("Dados atualizados — confira e salve"); }
    catch (er) { toast(esc(er.message), "var(--er)"); }
  };
}
function certificadoEmpresa(id) {
  modal(`<h3>Certificado digital A1</h3><p class="muted">Necessário para consultar a SEFAZ, baixar as notas recebidas e manifestar. O arquivo fica guardado criptografado neste computador.</p>
  <form id="fcert"><div class="form"><label class="w2">Arquivo .pfx / .p12<input type="file" name="arquivo" accept=".pfx,.p12" required></label><label>Senha<input type="password" name="senha" required></label></div>
  <div class="acoes"><button class="btn primary">Salvar certificado</button><button type="button" class="btn" onclick="fecharModal()">Agora não</button></div></form>`);
  $("#fcert").onsubmit = async ev => {
    ev.preventDefault();
    try { const c = await api(`/empresas/${id}/certificado`, { method: "POST", body: new FormData(ev.target) }); toast(`Certificado incluído: ${esc(c.titular)}`, "var(--ok)"); fecharModal(); mostrar("empresas"); }
    catch (e) { toast(esc(e.message), "var(--er)"); }
  };
}
async function removerCert(id) { if (confirm("Remover o certificado desta empresa?")) { await api(`/empresas/${id}/certificado`, { method: "DELETE" }); mostrar("empresas"); } }

/* ---------- Configurações ---------- */
VIEWS.config = async el => {
  const abas = [["usuarios", "Usuários"], ["uf", "Alíquotas por UF"], ["mva", "MVA / ICMS-ST"], ["integracao", "Integração com ERP"], ["sistema", "Sistema"]];
  el.innerHTML = `<div class="tabs">${abas.map(([k, l]) => `<a class="${estado.abaCfg === k ? "on" : ""}" data-a="${k}">${l}</a>`).join("")}</div><div id="cfg"></div>`;
  $$(".tabs a", el).forEach(a => a.onclick = () => { estado.abaCfg = a.dataset.a; mostrar("config"); });
  await CFG[estado.abaCfg]($("#cfg"));
};
const CFG = {};
CFG.usuarios = async el => {
  const us = await api("/usuarios");
  el.innerHTML = `<div class="tbl"><table><thead><tr><th>Login</th><th>Nome</th><th>Perfil</th></tr></thead><tbody>${us.map(u => `<tr><td>${esc(u.login)}</td><td>${esc(u.nome)}</td><td>${esc({ ADMIN: "Administrador", FISCAL: "Fiscal", CONSULTA: "Consulta" }[u.perfil] || u.perfil)}</td></tr>`).join("")}</tbody></table></div>
  <form id="fu" class="filtros" style="margin-top:16px"><label>Login<input name="login" required></label><label>Nome<input name="nome"></label><label>Senha<input type="password" name="senha" minlength="8" required></label><label>Perfil<select name="perfil"><option value="FISCAL">Fiscal</option><option value="CONSULTA">Consulta</option><option value="ADMIN">Administrador</option></select></label><button class="btn primary">Criar usuário</button></form>`;
  $("#fu").onsubmit = async ev => { ev.preventDefault(); try { await api("/usuarios", { method: "POST", json: Object.fromEntries(new FormData(ev.target)) }); mostrar("config"); } catch (e) { toast(esc(e.message), "var(--er)"); } };
};
CFG.uf = async el => {
  const ps = await api("/parametros/uf");
  el.innerHTML = `<p class="muted">Alíquotas internas e FCP de referência usados na conferência fiscal dos documentos. Revise conforme a legislação vigente.</p>
  <div class="tbl"><table><thead><tr><th>UF</th><th>Alíq. interna %</th><th>FCP %</th><th>Base do DIFAL</th></tr></thead><tbody>${ps.map(p => `<tr><td><b>${p.uf}</b></td><td><input data-uf="${p.uf}" data-k="aliq_interna" value="${p.aliq_interna}" size="6"></td><td><input data-uf="${p.uf}" data-k="fcp" value="${p.fcp}" size="6"></td>
  <td><select data-uf="${p.uf}" data-k="difal_metodo"><option value="BASE_DUPLA" ${p.difal_metodo === "BASE_DUPLA" ? "selected" : ""}>Base dupla</option><option value="BASE_UNICA" ${p.difal_metodo === "BASE_UNICA" ? "selected" : ""}>Base única</option></select></td></tr>`).join("")}</tbody></table></div>
  <div class="acoes"><button class="btn primary" id="salvar-uf">Salvar alterações</button></div>`;
  $("#salvar-uf").onclick = async () => { const m = {}; $$("[data-uf]", el).forEach(i => { m[i.dataset.uf] = m[i.dataset.uf] || { uf: i.dataset.uf }; m[i.dataset.uf][i.dataset.k] = i.dataset.k === "difal_metodo" ? i.value : +String(i.value).replace(",", "."); });
    await api("/parametros/uf", { method: "PUT", json: Object.values(m) }); toast("Parâmetros salvos", "var(--ok)"); };
};
CFG.mva = async el => {
  const ms = await api("/parametros/mva");
  el.innerHTML = `<p class="muted">MVA-ST por NCM e UF de destino, usada na conferência do ICMS-ST dos documentos.</p>
  <form id="fmva" class="filtros"><label>NCM/prefixo<input name="ncm" required></label><label>CEST<input name="cest"></label><label>UF destino<input name="uf_destino" maxlength="2" required></label><label>MVA %<input name="mva_original" required></label><label>Início<input type="date" name="vigencia_ini"></label><label>Fim<input type="date" name="vigencia_fim"></label><label>Fundamento<input name="fundamento"></label><button class="btn primary">Adicionar</button></form>
  <div class="tbl"><table><thead><tr><th>NCM</th><th>CEST</th><th>UF</th><th class="n">MVA %</th><th>Vigência</th><th>Fundamento</th><th></th></tr></thead><tbody>${ms.map(m => `<tr><td>${esc(m.ncm)}</td><td>${esc(m.cest)}</td><td>${esc(m.uf_destino)}</td><td class="n">${num(m.mva_original)}</td><td>${esc(m.vigencia_ini)} → ${esc(m.vigencia_fim || "…")}</td><td>${esc(m.fundamento)}</td><td><button class="btn sm danger" onclick="remMVA(${m.id})">Remover</button></td></tr>`).join("") || `<tr><td colspan="7" class="muted">Nenhuma MVA cadastrada.</td></tr>`}</tbody></table></div>`;
  $("#fmva").onsubmit = async ev => { ev.preventDefault(); const f = Object.fromEntries(new FormData(ev.target)); f.mva_original = +String(f.mva_original).replace(",", "."); f.uf_destino = f.uf_destino.toUpperCase();
    try { await api("/parametros/mva", { method: "POST", json: f }); mostrar("config"); } catch (e) { toast(esc(e.message), "var(--er)"); } };
};
async function remMVA(id) { await api(`/parametros/mva/${id}`, { method: "DELETE" }); mostrar("config"); }
CFG.integracao = async el => {
  const [c, wh] = await Promise.all([api("/config"), api("/integracoes/webhooks")]);
  el.innerHTML = `<div class="card"><h3>API local para o ERP</h3><div class="kv"><b>Endereço</b><span><code>http://127.0.0.1:${c.config.porta}/api/v1</code></span><b>Chave (header X-API-Key)</b><span><code>${esc(c.config.api_key)}</code></span>
  <b>Enviar XML</b><span><code>POST /documentos/importar</code> ou gravar na pasta <code>${esc(c.config.pasta_monitorada)}</code></span><b>Consultar</b><span><code>GET /documentos?de=AAAA-MM-DD&ate=AAAA-MM-DD</code></span>
  <b>Exportar</b><span><code>POST /documentos/exportar</code> {"chaves":[…],"formato":"xml|pdf|ambos"}</span></div></div>
  <div class="card" style="margin-top:16px"><h3>Webhooks</h3><p class="muted">Avisa o ERP quando chega documento novo ou muda a situação (documento.novo, documento.status, ou *).</p>
  ${wh.map(w => `<div class="kv"><b>${esc(w.eventos)}</b><span>${esc(w.url)} <button class="btn sm danger" onclick="remWh(${w.id})">Remover</button></span></div>`).join("")}
  <form id="fw" class="filtros" style="box-shadow:none;margin-top:8px"><label class="busca">URL<input name="url" required></label><label>Eventos<input name="eventos" value="*"></label><label>Segredo<input name="segredo"></label><button class="btn primary">Adicionar</button></form></div>`;
  $("#fw").onsubmit = async ev => { ev.preventDefault(); await api("/integracoes/webhooks", { method: "POST", json: Object.fromEntries(new FormData(ev.target)) }); mostrar("config"); };
};
async function remWh(id) { await api(`/integracoes/webhooks/${id}`, { method: "DELETE" }); mostrar("config"); }
CFG.sistema = async el => {
  const c = await api("/config"), cf = c.config;
  const campos = [["intervalo_distdfe_min", "Intervalo da busca de notas recebidas (min, mínimo 60)"], ["pasta_monitorada", "Pasta de importação de XML"], ["cadeia_icp_brasil", "Arquivo PEM da cadeia ICP-Brasil (opcional)"], ["porta", "Porta da API local"]];
  el.innerHTML = `<form id="fs" class="card"><p class="muted">Pasta de dados: <code>${esc(c.diretorio_dados)}</code></p><div class="form">${campos.map(([k, l]) => `<label>${l}<input name="${k}" value="${esc(cf[k] ?? "")}"></label>`).join("")}
  <label>Verificar certificado do servidor da SEFAZ<select name="verificar_tls_sefaz"><option value="1">Sim (recomendado)</option><option value="" ${cf.verificar_tls_sefaz ? "" : "selected"}>Não</option></select></label>
  <label>Atualizar situação das notas emitidas na SEFAZ<select name="consultar_pendentes_sefaz"><option value="1">Sim</option><option value="" ${cf.consultar_pendentes_sefaz ? "" : "selected"}>Não</option></select></label></div>
  <div class="acoes"><button class="btn primary">Salvar</button></div><p class="muted">Alterar a porta exige reiniciar o programa.</p></form>`;
  $("#fs").onsubmit = async ev => { ev.preventDefault(); const o = {};
    for (const [k, v] of new FormData(ev.target)) o[k] = ["verificar_tls_sefaz", "consultar_pendentes_sefaz"].includes(k) ? !!v : (/^(pasta_monitorada|cadeia_icp_brasil)$/.test(k) ? v : +String(v).replace(",", "."));
    await api("/config", { method: "PUT", json: o }); toast("Configuração salva", "var(--ok)"); };
};

iniciar();
