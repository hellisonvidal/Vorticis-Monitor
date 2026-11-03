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
  relatorio: '<path d="M4 20V10M10 20V4M16 20v-7M22 20H2"/>', nf: '<path d="M6 3h9l4 4v14H6z"/><path d="M9 9h6M9 13h6M9 17h4"/>',
  chave: '<circle cx="8" cy="15" r="4"/><path d="M11 12l9-9M16 7l3 3"/>', mais: '<path d="M12 5v14M5 12h14"/>',
};
const svg = n => `<svg viewBox="0 0 24 24">${IC[n] || ""}</svg>`;

const MENU = [
  { grupo: "Visão geral" }, { v: "painel", t: "Painel", i: "painel", sub: "Resumo dos documentos no período" },
  { grupo: "Documentos" },
  { v: "nfe", t: "NF-e / NFC-e", i: "nf", sub: "Notas fiscais de mercadorias — emitidas e recebidas" },
  { v: "servicos", t: "NFS-e", i: "servico", sub: "Notas fiscais de serviço — prestadas (emitidas) e tomadas (recebidas)" },
  { v: "transporte", t: "Transporte", i: "transporte", sub: "CT-e e MDF-e — emitidos e recebidos" },
  { v: "documentos", t: "Todos os documentos", i: "docs", sub: "Consulta geral de documentos fiscais" },
  { grupo: "Análise" },
  { v: "relatorios", t: "Relatórios", i: "relatorio", sub: "Relatórios do período em PDF, Excel (XLSX) e CSV" },
  { grupo: "Cadastros" },
  { v: "empresas", t: "Empresas", i: "empresa", sub: "Empresas e filiais monitoradas e certificados digitais" },
  { v: "config", t: "Configurações", i: "config", sub: "Usuários, parâmetros e integração", admin: true },
];

const hoje = new Date();
const estado = {
  token: localStorage.getItem("vtx_token") || "", empresa: localStorage.getItem("vtx_emp") || "", view: "painel", empresas: [],
  de: iso(new Date(hoje.getFullYear(), hoje.getMonth(), 1)), ate: iso(hoje), preset: "mes",
  busca: "", status: "", modelo: "", direcao: "S", pagina: 1, rel: "documentos", sel: new Set(), abaCfg: "usuarios",
};
let es = null;

async function api(path, opt = {}) {
  opt.headers = Object.assign({ Authorization: "Bearer " + estado.token }, opt.headers || {});
  if (opt.json !== undefined) { opt.body = JSON.stringify(opt.json); opt.headers["Content-Type"] = "application/json"; }
  const ctl = new AbortController(), lim = setTimeout(() => ctl.abort(), opt.tempo || 120000);
  let r;
  try { r = await fetch("/api/v1" + path, { ...opt, signal: ctl.signal }); }
  catch (e) { throw new Error(e.name === "AbortError" ? "A consulta demorou demais e foi interrompida. Verifique a internet e tente novamente." : "Não foi possível falar com o serviço do monitor: " + e.message); }
  finally { clearTimeout(lim); }
  if (r.status === 401 && !path.startsWith("/auth")) { expirou(); throw new Error("Sessão expirada"); }
  if (opt.blob) { if (!r.ok) throw new Error((await r.json().catch(() => ({}))).erro || r.statusText); return r; }
  const ct = r.headers.get("content-type") || "";
  const body = ct.includes("json") ? await r.json() : await r.text();
  if (!r.ok) throw new Error(body.erro || body || r.statusText);
  return body;
}
window.addEventListener("error", e => toast("Erro na tela: " + esc(e.message), "var(--er)"));
window.addEventListener("unhandledrejection", e => toast(esc(e.reason?.message || e.reason || "Falha inesperada"), "var(--er)"));
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
// "Salvar senha": o servidor entrega uma credencial assinada (180 dias, invalidada
// se a senha mudar); a senha em si nunca fica guardada no computador.
const LS = { get: k => { try { return localStorage.getItem(k) || ""; } catch { return ""; } }, set: (k, v) => { try { v ? localStorage.setItem(k, v) : localStorage.removeItem(k); } catch { } } };
function prepararLogin() {
  const cred = LS.get("vtx_cred"), f = $("#form-login");
  $("#ck-lembrar").checked = !!cred; $("#ck-auto").checked = !!LS.get("vtx_auto");
  $("#ck-auto").onchange = () => { if ($("#ck-auto").checked) $("#ck-lembrar").checked = true; };
  $("#ck-lembrar").onchange = () => { if (!$("#ck-lembrar").checked) $("#ck-auto").checked = false; };
  if (cred) {
    f.login.value = LS.get("vtx_login"); f.senha.required = false; f.senha.placeholder = "•••••• senha salva";
    $("#esquecer").classList.remove("hidden");
  }
  $("#esquecer").onclick = () => { ["vtx_cred", "vtx_auto", "vtx_login"].forEach(k => LS.set(k, "")); f.senha.required = true; f.senha.placeholder = ""; $("#esquecer").classList.add("hidden"); $("#ck-lembrar").checked = $("#ck-auto").checked = false; };
}
async function iniciar() {
  const e = await fetch("/api/v1/auth/estado").then(r => r.json());
  if (e.precisa_setup) {
    $("#login").classList.remove("hidden"); $("#setup-campos").classList.remove("hidden");
    $("#login-sub").textContent = "Primeiro acesso: crie o usuário administrador do programa"; $("#btn-login").textContent = "Criar administrador";
    $("#form-login").dataset.setup = "1"; $("#lembrar-campos").classList.add("hidden"); return;
  }
  if (estado.token) { try { await carregarEmpresas(); await entrar(); return; } catch { estado.token = ""; } }
  prepararLogin();
  if (LS.get("vtx_cred") && LS.get("vtx_auto") && !sessionStorage.getItem("vtx_saiu")) {
    try { await autenticar({ credencial: LS.get("vtx_cred") }, true, true); return; } catch (er) { $("#login-erro").textContent = er.message; }
  }
  $("#login").classList.remove("hidden");
}
async function autenticar(corpo, lembrar, auto, setup) {
  const r = await api(setup ? "/auth/setup" : "/auth/login", { method: "POST", json: { ...corpo, lembrar } });
  estado.token = r.token; LS.set("vtx_token", r.token); sessionStorage.removeItem("vtx_saiu");
  if (lembrar && r.credencial) { LS.set("vtx_cred", r.credencial); LS.set("vtx_login", r.usuario?.login || corpo.login); LS.set("vtx_auto", auto ? "1" : ""); }
  else if (!setup) ["vtx_cred", "vtx_auto", "vtx_login"].forEach(k => LS.set(k, ""));
  $("#login").classList.add("hidden"); await carregarEmpresas(); await entrar();
  if (!estado.empresas.length && ehAdmin()) { mostrar("empresas"); novaEmpresa(); }
}
$("#form-login").onsubmit = async ev => {
  ev.preventDefault();
  const f = new FormData(ev.target), setup = ev.target.dataset.setup === "1", cred = LS.get("vtx_cred");
  const usaCred = !setup && cred && !f.get("senha") && f.get("login").toLowerCase() === LS.get("vtx_login").toLowerCase();
  const corpo = usaCred ? { credencial: cred } : { login: f.get("login"), senha: f.get("senha"), nome: f.get("nome") };
  try { await autenticar(corpo, !!f.get("lembrar"), !!f.get("auto"), setup); }
  catch (e) { $("#login-erro").textContent = e.message; if (usaCred) $("#esquecer").click(); }
};
function sair() { LS.set("vtx_token", ""); estado.token = ""; sessionStorage.setItem("vtx_saiu", "1"); location.reload(); }
function expirou() { LS.set("vtx_token", ""); estado.token = ""; location.reload(); }
$("#sair").onclick = sair;
$("#recolher").onclick = () => { $("#side").classList.toggle("mini"); localStorage.setItem("vtx_mini", $("#side").classList.contains("mini") ? "1" : ""); };
if (localStorage.getItem("vtx_mini")) $("#side").classList.add("mini");

const ehAdmin = () => estado.eu?.perfil === "ADMIN";
const podeAlterar = () => ["ADMIN", "FISCAL"].includes(estado.eu?.perfil);
async function entrar() {
  estado.eu = await api("/auth/eu").catch(() => ({ perfil: "ADMIN" }));
  $("#app").classList.remove("hidden");
  $("#eu-nome").textContent = estado.eu.nome || estado.eu.login || "";
  if (MENU.find(m => m.v === estado.view)?.admin && !ehAdmin()) estado.view = "painel";
  $("#menu").innerHTML = MENU.filter(m => !m.admin || ehAdmin()).map(m => m.grupo ? `<div class="grupo">${m.grupo}</div>` : `<a data-v="${m.v}" title="${m.t}">${svg(m.i)}<span>${m.t}</span></a>`).join("");
  $$("#menu a").forEach(a => a.onclick = () => { estado.pagina = 1; estado.status = ""; estado.modelo = ""; estado.busca = ""; mostrar(a.dataset.v); });
  conectarSSE(); mostrar(estado.view);
}
async function carregarEmpresas() {
  estado.empresas = await api("/empresas");
  if (estado.empresa && !estado.empresas.some(e => String(e.id) === estado.empresa)) { estado.empresa = ""; localStorage.setItem("vtx_emp", ""); }
  rotuloEmpresa();
  const sem = estado.empresas.filter(e => !e.tem_certificado), b = $("#banner");
  if (!estado.empresas.length) { b.innerHTML = `${svg("empresa")} Nenhuma empresa cadastrada. <a onclick="mostrar('empresas');novaEmpresa()">Cadastrar empresa pelo CNPJ ou certificado</a>`; b.classList.remove("hidden"); }
  else if (sem.length) { b.innerHTML = `${svg("chave")} ${sem.length} empresa(s) sem certificado digital — necessário para consultar a SEFAZ e baixar os XMLs recebidos. <a onclick="mostrar('empresas')">Incluir certificado</a>`; b.classList.remove("hidden"); }
  else b.classList.add("hidden");
}
// Seletor de empresa: CNPJ antes do nome e pesquisa por CNPJ (com ou sem pontuação) ou nome.
const semAcento = s => String(s || "").normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase();
const empRaiz = e => (e.cnpj || "").slice(0, 8);
const empOrdem = () => [...estado.empresas].sort((a, b) => empRaiz(a).localeCompare(empRaiz(b)) || (b.matriz - a.matriz) || a.cnpj.localeCompare(b.cnpj));
function rotuloEmpresa() {
  const e = estado.empresas.find(x => String(x.id) === estado.empresa);
  $("#emp-rot").innerHTML = e ? `<b>${docFmt(e.cnpj)}</b> ${esc(e.razao_social)}` : "Todas as empresas";
}
function listaEmpresas() {
  const t = $("#emp-busca").value.trim(), dig = t.replace(/\D/g, ""), txt = semAcento(t);
  const achou = empOrdem().filter(e => !t || (dig && e.cnpj.includes(dig)) || semAcento(e.razao_social).includes(txt));
  const it = (id, html, on) => `<a data-emp="${id}" class="${on ? "on" : ""}">${html}</a>`;
  $("#emp-lista").innerHTML = (!t ? it("", "<span>Todas as empresas</span>", !estado.empresa) : "") +
    achou.map(e => it(e.id, `<b>${docFmt(e.cnpj)}</b><span>${esc(e.razao_social)}</span>${e.cnpj.length === 14 && !e.matriz ? `<i>filial</i>` : ""}`, String(e.id) === estado.empresa)).join("") +
    (t && !achou.length ? `<div class="muted" style="padding:10px 12px">Nenhuma empresa com "${esc(t)}"</div>` : "");
  $$("#emp-lista a").forEach(a => a.onclick = () => escolherEmpresa(a.dataset.emp));
}
function escolherEmpresa(id) {
  $("#emp-pop").classList.add("hidden");
  estado.empresa = String(id); localStorage.setItem("vtx_emp", estado.empresa); rotuloEmpresa(); estado.pagina = 1; mostrar(estado.view);
}
$("#emp-btn").onclick = ev => {
  ev.stopPropagation(); const p = $("#emp-pop"), abrir = p.classList.contains("hidden");
  p.classList.toggle("hidden", !abrir);
  if (abrir) { $("#emp-busca").value = ""; listaEmpresas(); $("#emp-busca").focus(); }
};
$("#emp-pop").onclick = ev => ev.stopPropagation();
document.addEventListener("click", () => $("#emp-pop").classList.add("hidden"));
$("#emp-busca").oninput = listaEmpresas;
$("#emp-busca").onkeydown = ev => {
  if (ev.key === "Escape") { $("#emp-pop").classList.add("hidden"); ev.stopPropagation(); }
  if (ev.key === "Enter") { const a = $("#emp-lista a"); if (a) escolherEmpresa(a.dataset.emp); }
};
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
  es.addEventListener("captura.ciclo", () => { if (["nfe", "servicos"].includes(estado.view)) refresh(); else if (estado.view === "config" && estado.abaCfg === "monitoramento") mostrar("config", true); });
}

/* ---------- navegação ---------- */
const VIEWS = {};
const VIEWS_DOC = ["painel", "documentos", "nfe", "servicos", "transporte"];
async function mostrar(v, silencioso) {
  estado.view = v;
  const m = MENU.find(x => x.v === v) || MENU[1];
  $("#titulo").textContent = m.t; $("#subtitulo").textContent = m.sub;
  $$("#menu a").forEach(a => a.classList.toggle("ativo", a.dataset.v === v));
  const el = $("#view");
  if (!silencioso) el.innerHTML = `<p class="muted">Carregando…</p>`;
  else el.classList.add("carregando");
  try { await (VIEWS[v] || VIEWS.painel)(el); } catch (e) { el.innerHTML = `<div class="card erro">${esc(e.message)}</div>`; toast(esc(e.message), "var(--er)"); }
  el.classList.remove("carregando");
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
function filtroHTML(extras = "", compacto) {
  return `<div class="filtros${compacto ? " compacto" : ""}">
    <label>De<input type="date" id="f-de" value="${estado.de}"></label>
    <label>Até<input type="date" id="f-ate" value="${estado.ate}"></label>
    <div class="presets">${Object.keys(PRESETS).map(k => `<button type="button" data-p="${k}" class="${estado.preset === k ? "on" : ""}">${ROT_PRESET[k]}</button>`).join("")}</div>
    ${extras}
    <button class="btn primary" id="f-ok">${svg("busca")} Consultar</button>
  </div>`;
}
// O botão azul "Consultar" concentra todas as ações da tela: busca as notas novas
// na SEFAZ / Ambiente Nacional (conforme a tela) e depois consulta o período.
const SINCRONIZA = { painel: ["nfe", "nfse"], documentos: ["nfe", "nfse"], nfe: ["nfe"], servicos: ["nfse"] };
function ligarFiltro(el, extrasLer) {
  const aplicar = async (comBusca) => {
    estado.de = $("#f-de", el).value; estado.ate = $("#f-ate", el).value;
    if (!estado.de || !estado.ate) { toast("Informe a data inicial e a final", "var(--er)"); return; }
    if (estado.de > estado.ate) { toast("A data inicial é posterior à final", "var(--er)"); return; }
    if (extrasLer) extrasLer();
    const b = $("#f-ok", el); b.disabled = true;
    if (comBusca === true && SINCRONIZA[estado.view] && podeAlterar()) { b.textContent = estado.view === "servicos" ? "Buscando no Ambiente Nacional…" : "Buscando na SEFAZ…"; await sincronizar(SINCRONIZA[estado.view]); }
    b.textContent = "Consultando…";
    estado.pagina = 1; estado.total = undefined;
    await mostrar(estado.view, true);
    if (estado.total !== undefined) toast(`Consulta concluída: <b>${estado.total}</b> ${estado.view === "relatorios" ? "linha(s)" : "documento(s)"} de ${dia(estado.de + "T12:00")} a ${dia(estado.ate + "T12:00")}`, "var(--ok)");
  };
  $$(".presets button", el).forEach(b => b.onclick = () => { [estado.de, estado.ate] = PRESETS[b.dataset.p](); estado.preset = b.dataset.p; $("#f-de", el).value = estado.de; $("#f-ate", el).value = estado.ate; aplicar(); });
  $("#f-de", el).onchange = $("#f-ate", el).onchange = () => { estado.preset = ""; };
  $("#f-ok", el).onclick = () => aplicar(true);
  $$(".filtros input:not([type=date])", el).forEach(i => i.onkeydown = e => { if (e.key === "Enter") aplicar(true); });
}
// Busca as notas novas de cada empresa do filtro: NF-e (Distribuição DF-e, inclusive
// pelo certificado do contador — autXML) e NFS-e (ADN). Respeita a regra de 1 hora da SEFAZ.
async function sincronizar(tipos) {
  const emps = estado.empresas.filter(e => e.ativo !== false && (!estado.empresa || String(e.id) === estado.empresa));
  const tarefas = [];
  const pedir = (rot, nome, url) => tarefas.push(api(url, { method: "POST", tempo: 240000 }).then(r => ({ rot, nome, r })).catch(er => ({ rot, nome, er })));
  if (tipos.includes("nfe")) {
    emps.filter(e => e.tem_certificado).forEach(e => pedir("NF-e", e.razao_social, `/captura/${e.id}/executar?forcar=1`));
    const procs = await api("/procuradores").catch(() => []);
    procs.filter(p => p.nfe_autxml && p.uf).forEach(p => pedir("NF-e (autXML)", p.nome, `/procuradores/${p.id}/nfe?forcar=1`));
  }
  if (tipos.includes("nfse"))
    emps.filter(e => e.tem_certificado || (e.nfse_modo === "PROCURACAO" && e.nfse_procurador_id)).forEach(e => pedir("NFS-e", e.razao_social, `/captura/${e.id}/nfse?forcar=1`));
  if (!tarefas.length) return;
  const res = await Promise.all(tarefas);
  let novos = 0; const erros = [], avisos = [];
  for (const x of res) {
    if (x.er) { erros.push(`<b>${esc(x.nome)}</b> · ${x.rot}: ${esc(x.er.message)}`); continue; }
    novos += x.r.documentos || 0;
    if (x.r.ignorado || x.r.c_stat === 656 || x.r.c_stat === 429 || [401, 403].includes(x.r.c_stat) || (x.rot === "NFS-e" && x.r.c_stat >= 400 && x.r.c_stat !== 404))
      avisos.push(`<b>${esc(x.nome)}</b> · ${x.rot}: ${esc(x.r.motivo)}`);
  }
  toast(novos ? `<b>${novos}</b> documento(s) novo(s) baixado(s)` : "Nenhum documento novo na SEFAZ / Ambiente Nacional", novos ? "var(--ok)" : "");
  if (avisos.length) toast(avisos.join("<br>"), "var(--wa)");
  if (erros.length) toast(erros.join("<br>"), "var(--er)");
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
  const s = r.por_status || {}; estado.total = r.total;
  const kpi = (st, ic, cor) => `<div class="card kpi" onclick="estado.status='${st}';estado.direcao='';mostrar('documentos')"><div class="ic ${cor}">${svg(ic)}</div><div><div class="n">${s[st] || 0}</div><div class="l">${ST[st]}</div></div></div>`;
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
      <b>Sem manifestação conclusiva</b><span><a onclick="estado.direcao='E';mostrar('nfe')">${r.sem_manifestacao} NF-e recebida(s)</a></span></div></div>
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
  const lr = $("#lote-rot"); if (lr) lr.textContent = n ? `Lote: ${n} selecionado(s)` : `Lote: todos os ${estado.total ?? ""} do filtro`;
}
$("#sel-limpar").onclick = () => { estado.sel.clear(); mostrar(estado.view, true); };
$$("#selecao [data-exp]").forEach(b => b.onclick = () => exportar([...estado.sel], b.dataset.exp));
async function exportar(chaves, formato) {
  if (!chaves.length) return;
  toast(`Gerando arquivo com ${chaves.length} documento(s)…`);
  try { await baixar(await api("/documentos/exportar", { method: "POST", json: { chaves, formato }, blob: true, tempo: 30 * 60000 }), "documentos.zip"); toast("Exportação concluída — arquivo salvo em Downloads", "var(--ok)"); }
  catch (e) { toast(esc(e.message), "var(--er)"); }
}

/* ---------- consultas de documentos ---------- */
// Seletor destacado Emitidos | Recebidos | Todos, com a quantidade do período em cada um.
async function segDirecao(fixo, rot) {
  const base = qsBase({ por_pagina: 1, q: estado.busca, status: estado.status, ...fixo });
  const total = async d => { const q = new URLSearchParams(base); d ? q.set("direcao", d) : q.delete("direcao"); return (await api("/documentos?" + q)).total; };
  const [ns, ne, nt] = await Promise.all([total("S"), total("E"), total("")]);
  return `<div class="seg">${[["S", rot[0], ns, "saida"], ["E", rot[1], ne, "entrada"], ["", "Todos", nt, "docs"]].map(([k, l, n, ic]) =>
    `<button type="button" data-d="${k}" class="${estado.direcao === k ? "on" : ""} d-${k || "T"}">${svg(ic)}<span>${l}</span><b>${n}</b></button>`).join("")}</div>`;
}
async function consultaDocs(el, fixo, opts = {}) {
  const mods = opts.modelos || [];
  if (estado.modelo && !mods.includes(estado.modelo)) estado.modelo = "";
  const filtroModelo = { ...fixo }; if (estado.modelo) filtroModelo.modelo = estado.modelo;
  const extras = `
    <label class="busca">Pesquisar<input id="f-q" value="${esc(estado.busca)}" placeholder="Número, chave, CNPJ ou nome"></label>
    ${mods.length > 1 ? `<label>Modelo<select id="f-mod"><option value="">Todos</option>${mods.map(m => `<option value="${m}" ${estado.modelo === m ? "selected" : ""}>${MOD[m]}</option>`).join("")}</select></label>` : ""}
    <label>Situação<select id="f-st"><option value="">Todas</option>${Object.entries(ST).map(([k, v]) => `<option value="${k}" ${estado.status === k ? "selected" : ""}>${v}</option>`).join("")}</select></label>`;
  const q = qsBase({ por_pagina: 50, pagina: estado.pagina, q: estado.busca, status: estado.status, direcao: estado.direcao, ...filtroModelo });
  const [seg, d] = await Promise.all([segDirecao(filtroModelo, opts.rotulos || ["Emitidos", "Recebidos"]), api("/documentos?" + q)]);
  estado.total = d.total;
  const recebidos = estado.direcao === "E", emitidos = estado.direcao === "S";
  const gopts = { manif: recebidos && opts.manif, parte: recebidos ? opts.parteE || "Emitente" : emitidos ? opts.parteS || "Destinatário" : undefined };
  const antes = opts.antesSempre ? await opts.antesSempre() : recebidos && opts.antesRecebidos ? await opts.antesRecebidos() : "";
  const pags = Math.max(1, Math.ceil(d.total / 50));
  el.innerHTML = seg + antes + filtroHTML(extras) + `
  <div class="lote"><span class="muted">${d.total} documento(s) entre ${dia(estado.de + "T12:00")} e ${dia(estado.ate + "T12:00")}</span>
  ${d.total ? `<div class="lote-acoes"><button class="btn sm" id="sel-filtro">Selecionar todos os ${d.total}</button>
    <span class="lote-rot" id="lote-rot">${estado.sel.size ? `Lote: ${estado.sel.size} selecionado(s)` : `Lote: todos os ${d.total} do filtro`}</span>
    <button class="btn sm" data-lote="xml">${svg("xml")} XML em lote</button><button class="btn sm" data-lote="pdf">${svg("pdf")} PDF em lote</button><button class="btn sm primary" data-lote="ambos">XML + PDF</button></div>` : ""}</div>
  ${gridDocs(d.itens, true, gopts)}
  <div class="paginacao"><button class="btn sm" ${estado.pagina <= 1 ? "disabled" : ""} id="pg-ant">‹ Anterior</button><span class="muted">Página ${estado.pagina} de ${pags}</span><button class="btn sm" ${estado.pagina >= pags ? "disabled" : ""} id="pg-prox">Próxima ›</button></div>`;
  $$(".seg button", el).forEach(b => b.onclick = () => { estado.direcao = b.dataset.d; estado.pagina = 1; mostrar(estado.view, true); });
  ligarFiltro(el, () => { estado.busca = $("#f-q").value.trim(); estado.status = $("#f-st").value; if ($("#f-mod")) estado.modelo = $("#f-mod").value; });
  ligarGrid(el);
  $("#pg-ant").onclick = () => { estado.pagina--; mostrar(estado.view, true); };
  $("#pg-prox").onclick = () => { estado.pagina++; mostrar(estado.view, true); };
  const sf = $("#sel-filtro");
  if (sf) sf.onclick = async () => { const qa = new URLSearchParams(q); qa.set("por_pagina", 5000); qa.set("pagina", 1); const all = await api("/documentos?" + qa); all.itens.forEach(x => estado.sel.add(x.chave)); mostrar(estado.view, true); };
  $$("[data-lote]", el).forEach(b => b.onclick = () => exportarLote(b.dataset.lote, q, d.total, b));
}
// Gera XML e/ou PDF em lote (ZIP): os documentos selecionados ou, sem seleção, todos os do filtro.
async function exportarLote(formato, q, total, btn) {
  if (estado.sel.size) return exportar([...estado.sel], formato);
  if (!total) return;
  if (total > 1000 && formato !== "xml" && !confirm(`Gerar o PDF de ${total} documentos pode levar alguns minutos. Continuar?`)) return;
  const qf = new URLSearchParams(q); qf.delete("por_pagina"); qf.delete("pagina");
  const txt = btn?.innerHTML; if (btn) { btn.disabled = true; btn.textContent = "Gerando…"; }
  toast(`Gerando arquivo com ${total} documento(s)…`);
  try { await baixar(await api("/documentos/exportar?" + qf, { method: "POST", json: { formato }, blob: true, tempo: 30 * 60000 }), "documentos.zip"); toast("Exportação concluída — arquivo salvo em Downloads", "var(--ok)"); }
  catch (e) { toast(esc(e.message), "var(--er)"); }
  if (btn) { btn.disabled = false; btn.innerHTML = txt; }
}
VIEWS.nfe = el => consultaDocs(el, { modelo: "NFE,NFCE" }, { modelos: ["NFE", "NFCE"], rotulos: ["Emitidas", "Recebidas"], manif: true, antesRecebidos: cardsCaptura });
VIEWS.servicos = el => consultaDocs(el, { modelo: "NFSE" }, { rotulos: ["Prestadas (emitidas)", "Tomadas (recebidas)"], parteS: "Tomador", parteE: "Prestador", antesSempre: cardsNFSe });

// Busca de NFS-e no Ambiente de Dados Nacional (ADN), por empresa: certificado próprio ou procuração.
async function cardsNFSe() {
  const [nsu, procs] = await Promise.all([api("/captura/status").catch(() => []), api("/procuradores").catch(() => [])]);
  const emps = estado.empresas.filter(e => !estado.empresa || String(e.id) === estado.empresa);
  if (!emps.length) return "";
  return `<div class="grid g2" style="margin-bottom:16px">${emps.map(e => {
    const n = nsu.find(x => x.empresa_id === e.id && x.servico === "NFSE_ADN") || {};
    const pr = e.nfse_modo === "PROCURACAO" ? procs.find(p => p.id === e.nfse_procurador_id) : null;
    const pode = pr || e.tem_certificado;
    const via = pr ? `Procuração eletrônica · certificado de <b>${esc(pr.nome)}</b>` : e.nfse_modo === "PROCURACAO" ? `<span style="color:var(--er)">Procuração sem procurador escolhido</span>` : e.cert_proprio ? "Certificado da própria empresa" : e.tem_certificado ? `Certificado da matriz ${esc(e.cert_empresa)}` : "Sem certificado — inclua o certificado ou use procuração do contador";
    return `<div class="card" style="display:flex;gap:14px;align-items:center">
      <div class="kpi"><div class="ic ${!pode ? "c-wa" : [401, 403].includes(n.ultimo_c_stat) ? "c-er" : "c-pr"}">${svg(pr ? "chave" : "servico")}</div></div>
      <div style="flex:1"><b>${esc(e.razao_social)}</b><div class="doc-sub">NFS-e Nacional (ADN) · ${via}<br>Última consulta: ${dt(n.ultima_consulta) || "ainda não realizada"}${n.proxima_consulta ? ` · próxima: ${dt(n.proxima_consulta)}` : ""}${n.ultimo_motivo ? `<br>${esc(n.ultimo_motivo)}` : ""}</div></div>
      ${pode || !ehAdmin() ? "" : `<button class="btn sm" onclick='editarEmpresa(${JSON.stringify(e).replace(/'/g, "&#39;")})'>Configurar</button>`}</div>`;
  }).join("")}</div>`;
}
VIEWS.transporte = el => consultaDocs(el, { modelo: "CTE,MDFE" }, { modelos: ["CTE", "MDFE"] });
VIEWS.documentos = el => consultaDocs(el, {}, { modelos: ["NFE", "NFCE", "CTE", "MDFE", "NFSE"] });

// Situação da busca automática de NF-e recebidas (Distribuição DF-e), por empresa.
async function cardsCaptura() {
  const nsu = await api("/captura/status").catch(() => []);
  const emps = estado.empresas.filter(e => !estado.empresa || String(e.id) === estado.empresa);
  if (!emps.length) return "";
  const agora = new Date();
  return `<div class="grid g2" style="margin-bottom:16px">${emps.map(e => {
    const n = nsu.find(x => x.empresa_id === e.id && x.servico === "NFE_DISTDFE") || {};
    const prox = n.proxima_consulta ? new Date(n.proxima_consulta) : null;
    const aguarda = prox && prox > agora && [137, 656].includes(n.ultimo_c_stat);
    const info = !e.tem_certificado ? "Inclua o certificado digital para baixar as notas destinadas à empresa"
      : `${e.cert_proprio ? "" : `Certificado da matriz ${esc(e.cert_empresa)}<br>`}Última consulta: ${dt(n.ultima_consulta || n.updated_at) || "ainda não realizada"}${prox ? ` · próxima: ${dt(n.proxima_consulta)}` : ""}${n.ultimo_motivo ? `<br>${n.ultimo_c_stat ? n.ultimo_c_stat + " · " : ""}${esc(n.ultimo_motivo)}` : ""}`;
    return `<div class="card" style="display:flex;gap:14px;align-items:center">
    <div class="kpi"><div class="ic ${!e.tem_certificado ? "c-wa" : n.ultimo_c_stat === 656 ? "c-er" : "c-pr"}">${svg(e.tem_certificado ? "entrada" : "chave")}</div></div>
    <div style="flex:1"><b>${esc(e.razao_social)}</b><div class="doc-sub">${info}</div>
    ${n.concorrente ? `<div class="aviso" style="margin:8px 0 0">Outro sistema também consulta este CNPJ na SEFAZ (ex.: outro monitor de notas). O intervalo foi ampliado para no mínimo 2 horas para evitar bloqueio por consumo indevido (656).</div>` : ""}</div>
    ${aguarda ? `<span class="st PENDENTE" title="A SEFAZ só libera nova consulta após ${dt(n.proxima_consulta)}">aguardando SEFAZ</span>` : ""}
    ${!e.tem_certificado && ehAdmin() ? `<button class="btn sm" onclick="mostrar('empresas')">Incluir certificado</button>` : ""}</div>`;
  }).join("")}</div>`;
}

/* ---------- Relatórios ---------- */
VIEWS.relatorios = async el => {
  const cat = estado.catRel || (estado.catRel = await api("/relatorios"));
  const rf = estado.relFiltro || (estado.relFiltro = { direcao: "", modelo: "", status: "" });
  const sel = (id, rot, val, ops) => `<label>${rot}<select id="${id}">${ops.map(([k, l]) => `<option value="${k}" ${val === k ? "selected" : ""}>${l}</option>`).join("")}</select></label>`;
  const extras = sel("r-dir", "Movimento", rf.direcao, [["", "Emitidos e recebidos"], ["S", "Emitidos"], ["E", "Recebidos"]]) +
    sel("r-mod", "Documento", rf.modelo, [["", "Todos"], ["NFE", "NF-e"], ["NFCE", "NFC-e"], ["NFSE", "NFS-e"], ["CTE", "CT-e"], ["MDFE", "MDF-e"]]) +
    sel("r-st", "Situação", rf.status, [["", "Todas"], ...Object.entries(ST)]);
  const q = qsBase({ direcao: rf.direcao, modelo: rf.modelo, status: rf.status });
  const camposSalvos = LS.get("vtx_rel_" + estado.rel);
  if (camposSalvos) q.set("colunas", camposSalvos);
  const resp = await api(`/relatorios/${estado.rel}?` + q), rel = resp.relatorio;
  estado.total = rel.linhas.length;
  const fmtCel = (v, t) => t === "M" ? brl(v) : t === "D" ? dia(v) : esc(v);
  const lim = rel.linhas.slice(0, 300);
  const selec = new Set(resp.selecionadas);
  el.innerHTML = filtroHTML(extras, true) + `<div class="rel-tipos">${cat.map(c => `<a data-r="${c.id}" class="${estado.rel === c.id ? "on" : ""}" title="${esc(c.descricao)}">${esc(c.nome)}</a>`).join("")}</div>
    <div class="card rel-prev"><div class="rel-topo">
      <div><h3 style="margin:0">${esc(rel.titulo)}</h3><div class="doc-sub">${esc(rel.filtros)} · ${rel.linhas.length} linha(s) · ${rel.colunas.length} de ${resp.disponiveis.length} campos</div></div>
      <div class="acoes" style="margin:0"><button class="btn sm ${estado.camposAbertos ? "on" : ""}" id="bt-campos">${svg("config")} Campos (${rel.colunas.length}/${resp.disponiveis.length})</button>
      ${[["pdf", "PDF"], ["xlsx", "Excel (XLSX)"], ["csv", "CSV"]].map(([f, l]) => `<button class="btn sm ${f === "pdf" ? "primary" : ""}" data-f="${f}">${svg(f === "pdf" ? "pdf" : "docs")} ${l}</button>`).join("")}</div></div>
      <div class="campos-painel ${estado.camposAbertos ? "" : "hidden"}" id="campos">
        <div class="campos-cab"><div><b>Campos do relatório</b><div class="doc-sub">Desmarque os campos que não quer no relatório (vale para a tela, PDF, Excel e CSV). A escolha fica salva para este relatório.</div></div>
        <span><a data-c="todos">Marcar todos</a> · <a data-c="padrao">Padrão</a></span></div>
        <div class="campos-lista">${resp.disponiveis.map((c, i) => `<label class="campo ${selec.has(i) ? "on" : ""}"><input type="checkbox" value="${i}" ${selec.has(i) ? "checked" : ""}> ${esc(c.titulo)}</label>`).join("")}</div>
        <div class="acoes" style="margin:10px 0 0"><button class="btn sm primary" id="campos-ok">Aplicar campos</button><button class="btn sm" id="campos-fechar">Fechar</button></div></div>
      <div class="tbl" style="margin-top:14px"><table><thead><tr>${rel.colunas.map((c, j) => `<th class="${"MN".includes(c.tipo) ? "n" : ""}">${esc(c.titulo)}${rel.colunas.length > 1 ? `<a class="tira" data-tira="${resp.selecionadas[j]}" title="Tirar o campo ${esc(c.titulo)} do relatório">×</a>` : ""}</th>`).join("")}</tr></thead><tbody>
      ${lim.map(l => `<tr>${l.map((v, i) => `<td class="${"MN".includes(rel.colunas[i].tipo) ? "n" : ""}">${fmtCel(v, rel.colunas[i].tipo)}</td>`).join("")}</tr>`).join("") || `<tr><td colspan="${rel.colunas.length}" class="muted">Nenhum registro no período e filtros informados.</td></tr>`}
      ${rel.total ? `<tr class="total">${rel.total.map((v, i) => `<td class="${"MN".includes(rel.colunas[i].tipo) ? "n" : ""}">${fmtCel(v, rel.colunas[i].tipo)}</td>`).join("")}</tr>` : ""}</tbody></table></div>
      ${rel.linhas.length > lim.length ? `<p class="muted">Pré-visualização das primeiras ${lim.length} linhas — o arquivo exportado contém todas.</p>` : ""}</div>`;
  const ajustar = () => { const t = $(".rel-prev .tbl", el); if (!t || !t.isConnected) { window.removeEventListener("resize", ajustar); return; } t.style.maxHeight = Math.max(220, window.innerHeight - t.getBoundingClientRect().top - (rel.linhas.length > lim.length ? 48 : 22)) + "px"; };
  ajustar(); window.addEventListener("resize", ajustar);
  const cx = $("#campos", el);
  $("#bt-campos", el).onclick = () => { estado.camposAbertos = cx.classList.toggle("hidden") ? false : true; $("#bt-campos", el).classList.toggle("on", estado.camposAbertos); ajustar(); };
  $("#campos-fechar", cx).onclick = () => { estado.camposAbertos = false; cx.classList.add("hidden"); $("#bt-campos", el).classList.remove("on"); ajustar(); };
  $$("input", cx).forEach(i => i.onchange = () => i.parentElement.classList.toggle("on", i.checked));
  $$("[data-c]", cx).forEach(a => a.onclick = () => $$("input", cx).forEach((i, n) => { i.checked = a.dataset.c === "todos" || resp.disponiveis[n].padrao; i.parentElement.classList.toggle("on", i.checked); }));
  const gravarCampos = v => {
    if (!v.length) { toast("Escolha pelo menos um campo", "var(--er)"); return; }
    const padrao = resp.disponiveis.map((c, i) => c.padrao ? String(i) : null).filter(x => x !== null).join(",");
    LS.set("vtx_rel_" + estado.rel, v.join(",") === padrao ? "" : v.join(",")); mostrar("relatorios", true);
  };
  $("#campos-ok", cx).onclick = () => gravarCampos($$("input:checked", cx).map(i => i.value));
  $$("[data-tira]", el).forEach(a => a.onclick = () => gravarCampos(resp.selecionadas.map(String).filter(i => i !== a.dataset.tira)));
  ligarFiltro(el, () => { rf.direcao = $("#r-dir").value; rf.modelo = $("#r-mod").value; rf.status = $("#r-st").value; });
  $$(".rel-tipos a", el).forEach(a => a.onclick = () => { estado.rel = a.dataset.r; mostrar("relatorios", true); });
  $$("[data-f]", el).forEach(b => b.onclick = async () => {
    const txt = b.innerHTML; b.disabled = true; b.textContent = "Gerando…";
    try { q.set("formato", b.dataset.f); await baixar(await api(`/relatorios/${estado.rel}?` + q, { blob: true }), `${estado.rel}.${b.dataset.f}`); toast("Relatório salvo em Downloads", "var(--ok)"); }
    catch (e) { toast(esc(e.message), "var(--er)"); }
    b.disabled = false; b.innerHTML = txt;
  });
};

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
function modal(html) { $("#modal-body").innerHTML = `<button type="button" class="btn icone modal-x" onclick="fecharModal()" title="Fechar (Esc)">${svg("x")}</button>` + html; $("#modal").classList.remove("hidden"); }
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
  const adm = ehAdmin(), filiais = r => estado.empresas.filter(x => empRaiz(x) === r && !x.matriz).length;
  const regime = { LR: "Lucro Real", LP: "Lucro Presumido", SN: "Simples Nacional", MEI: "MEI" };
  el.innerHTML = `<div class="filtros compacto"><label class="busca">Pesquisar<input id="emp-q" placeholder="CNPJ ou nome da empresa" value="${esc(estado.empQ || "")}"></label>
    <span class="muted" id="emp-qtd" style="align-self:center"></span>${adm ? `<button class="btn primary" onclick="novaEmpresa()">${svg("mais")} Nova empresa</button>` : ""}</div>
  ${estado.empresas.length ? `<div class="tbl"><table class="tab-emp"><thead><tr><th>CNPJ</th><th>Razão social</th><th>UF</th><th>Tipo</th><th>Regime</th><th>Ambiente</th><th>Certificado</th>${adm ? "<th></th>" : ""}</tr></thead><tbody>
  ${empOrdem().map(e => `<tr data-emp="${e.id}" class="${e.cnpj.length === 14 && !e.matriz ? "filial" : ""}">
    <td class="cnpj">${docFmt(e.cnpj)}</td>
    <td><b>${esc(e.razao_social)}</b><div class="doc-sub">IE ${esc(e.ie || "não informada")}${e.ativo === false ? " · inativa" : ""}</div></td>
    <td>${esc(e.uf)}</td>
    <td>${e.cnpj.length !== 14 ? "—" : e.matriz ? `<span class="st PENDENTE">Matriz${filiais(empRaiz(e)) ? ` · ${filiais(empRaiz(e))}` : ""}</span>` : `<span class="st CANCELADA">Filial</span>`}</td>
    <td>${esc(regime[e.regime] || e.regime)}</td>
    <td><span class="st ${e.ambiente === 1 ? "AUTORIZADA" : "PENDENTE"}">${e.ambiente === 1 ? "Produção" : "Homologação"}</span></td>
    <td id="cert-${e.id}" class="doc-sub">verificando…</td>
    ${adm ? `<td><div class="emp-acoes"><button class="btn sm" data-a="editar">Editar</button><button class="btn sm" data-a="cert">Certificado</button>${e.cnpj.length === 14 ? `<button class="btn sm" data-a="filial" title="Incluir filial desta empresa">${svg("mais")} Filial</button>` : ""}<button class="btn sm danger" data-a="excluir" title="Excluir empresa">${svg("x")}</button></div></td>` : ""}</tr>`).join("")}</tbody></table></div>`
    : `<div class="card vazio">${svg("empresa")}<div>Nenhuma empresa cadastrada.</div><div style="font-size:12.5px">Informe apenas o CNPJ ou use o certificado digital: os dados são importados da Receita Federal e a IE da SEFAZ.</div></div>`}`;
  const filtrar = () => {
    const t = $("#emp-q").value.trim(), dig = t.replace(/\D/g, ""), txt = semAcento(t); estado.empQ = t; let n = 0;
    $$("tr[data-emp]", el).forEach(tr => { const e = estado.empresas.find(x => String(x.id) === tr.dataset.emp), ok = !t || (dig && e.cnpj.includes(dig)) || semAcento(e.razao_social).includes(txt); tr.classList.toggle("hidden", !ok); n += ok; });
    $("#emp-qtd").textContent = `${n} de ${estado.empresas.length} empresa(s)`;
  };
  $("#emp-q").oninput = filtrar; filtrar();
  $$("tr[data-emp]", el).forEach(tr => {
    const e = estado.empresas.find(x => String(x.id) === tr.dataset.emp);
    tr.onclick = ev => {
      const a = ev.target.closest("[data-a]")?.dataset.a;
      if (a === "editar") editarEmpresa(e); else if (a === "cert") certificadoEmpresa(e.id); else if (a === "filial") novaEmpresa(empRaiz(e)); else if (a === "excluir") excluirEmpresa(e);
    };
  });
  for (const e of estado.empresas) {
    api(`/empresas/${e.id}/certificado`).then(c => $("#cert-" + e.id).innerHTML = `<span style="color:var(--tx)">${esc(String(c.titular).split(":")[0])}</span><br>até ${dia(c.valido_ate)} (${c.dias_restantes} dias)${c.herdado_de ? ` · <span class="st PENDENTE">da matriz</span>` : ""}${adm && e.cert_proprio ? ` · <a onclick="event.stopPropagation();removerCert(${e.id})" style="color:var(--er)">remover</a>` : ""}`)
      .catch(() => { const c = $("#cert-" + e.id); if (c) c.innerHTML = `<span class="st CONTINGENCIA">Sem certificado</span>`; });
  }
};
// Exclusão de empresa/filial: as notas podem ficar no monitor (sem empresa) ou ser excluídas.
async function excluirEmpresa(e) {
  const { documentos: n } = await api(`/empresas/${e.id}/documentos`);
  const dependentes = e.cert_proprio ? estado.empresas.filter(x => x.id !== e.id && x.cert_empresa_id === e.id) : [];
  modal(`<h3>Excluir ${e.cnpj.length === 14 && !e.matriz ? "filial" : "empresa"}</h3>
  <p><b>${docFmt(e.cnpj)}</b> ${esc(e.razao_social)}</p>
  <p class="muted">Serão removidos o cadastro, o certificado, o controle de busca na SEFAZ / Ambiente Nacional e os pedidos de compra desta empresa.</p>
  ${dependentes.length ? `<div class="aviso">${svg("alerta")} ${dependentes.length} filial(is) usam o certificado desta empresa (${dependentes.map(x => esc(x.razao_social)).join(", ")}) e ficarão sem certificado.</div>` : ""}
  <form id="fx">${n ? `<div class="opcoes"><label class="chk-inline"><input type="radio" name="docs" value="manter" checked> Manter as ${n} nota(s) no monitor (ficam em "Todas as empresas", sem vínculo)</label>
    <label class="chk-inline"><input type="radio" name="docs" value="excluir"> Excluir também as ${n} nota(s) desta empresa</label></div>` : `<p class="muted">Nenhuma nota vinculada a esta empresa.</p>`}
  <label style="margin-top:12px">Para confirmar, digite EXCLUIR<input name="conf" required autocomplete="off"></label>
  <div class="acoes"><button class="btn primary danger-bg">${svg("x")} Excluir</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`);
  $("#fx").onsubmit = async ev => {
    ev.preventDefault(); const f = new FormData(ev.target);
    if (f.get("conf").trim().toUpperCase() !== "EXCLUIR") { toast('Digite EXCLUIR para confirmar', "var(--er)"); return; }
    try {
      const r = await api(`/empresas/${e.id}?documentos=${f.get("docs") === "excluir" ? "excluir" : "manter"}`, { method: "DELETE" });
      fecharModal(); if (estado.empresa === String(e.id)) { estado.empresa = ""; localStorage.setItem("vtx_emp", ""); }
      toast(`${esc(e.razao_social)} excluída${r.documentos ? ` · ${r.documentos} nota(s) ${r.documentos_excluidos ? "excluída(s)" : "mantida(s) no monitor"}` : ""}`, "var(--ok)"); mostrar("empresas");
    } catch (er) { toast(esc(er.message), "var(--er)"); }
  };
}
// Escolha do certificado: instalado no Windows (repositório do usuário) ou arquivo .pfx.
async function blocoCertificado(el, cnpjBase) {
  el.innerHTML = `<div class="tabs" style="margin-bottom:12px"><a data-c="win" class="on">Instalado no Windows</a><a data-c="arq">Arquivo .pfx / .p12</a></div><div class="cert-orig"></div>`;
  const aba = async c => {
    $$("[data-c]", el).forEach(a => a.classList.toggle("on", a.dataset.c === c));
    const box = $(".cert-orig", el);
    if (c === "arq") { box.innerHTML = `<div class="form"><label class="w2">Arquivo .pfx / .p12<input type="file" name="arquivo" accept=".pfx,.p12" required></label><label>Senha<input type="password" name="senha" required></label></div>`; return; }
    box.innerHTML = `<p class="muted">Procurando certificados instalados neste computador…</p>`;
    try {
      const l = await api("/certificados/instalados");
      if (!l.length) { box.innerHTML = `<div class="aviso">Nenhum certificado e-CNPJ instalado no Windows deste usuário. Use a aba <b>Arquivo .pfx / .p12</b>.</div>`; return; }
      const pref = l.findIndex(c => c.valido && (!cnpjBase || c.cnpj.startsWith(cnpjBase)));
      box.innerHTML = `<div class="certs">${l.map((c, i) => `<label class="cert-item ${c.valido ? "" : "off"}"><input type="radio" name="impressao" value="${esc(c.impressao)}" ${i === pref ? "checked" : ""} ${c.valido ? "" : "disabled"} required>
        <div><b>${esc(c.titular)}</b><div class="doc-sub">${docFmt(c.cnpj)} · ${esc(c.emissor)} · ${c.valido ? "válido até " + dia(c.valido_ate) : "vencido em " + dia(c.valido_ate)}</div></div></label>`).join("")}</div>
        <p class="muted" style="font-size:12px">Se a chave puder ser exportada, uma cópia criptografada fica no cofre do monitor. Caso contrário, o monitor usa o certificado direto do Windows (que pode pedir confirmação de uso).</p>`;
    } catch (e) { box.innerHTML = `<div class="aviso">${esc(e.message)}. Use a aba <b>Arquivo .pfx / .p12</b>.</div>`; }
  };
  $$("[data-c]", el).forEach(a => a.onclick = () => aba(a.dataset.c));
  await aba("win");
}
const certDoForm = f => f.get("impressao") ? { impressao: f.get("impressao") } : { arquivo: f.get("arquivo"), senha: f.get("senha") };
function formCert(c) { const fd = new FormData(); for (const [k, v] of Object.entries(c)) fd.append(k, v); return fd; }
let novaCert = null; // {arquivo, senha} escolhido no cadastro pelo certificado
const cnpjRaizFmt = r => r.replace(/^(..)(...)(...)$/, "$1.$2.$3/");
function novaEmpresa(raizFilial) {
  novaCert = null;
  const mat = raizFilial ? estado.empresas.find(e => e.cnpj.startsWith(raizFilial) && e.matriz) || estado.empresas.find(e => e.cnpj.startsWith(raizFilial)) : null;
  modal(`<h3>${raizFilial ? "Incluir filial" : "Nova empresa"}</h3>${raizFilial ? `<p class="muted" style="margin-top:-6px">Filial de <b>${esc(mat?.razao_social || "")}</b>. Complete o CNPJ da filial: os dados vêm da Receita e as consultas na SEFAZ e no Ambiente Nacional usam o certificado da matriz (mesma raiz de CNPJ) — não é preciso outro certificado.</p>` : ""}<div class="passos ${raizFilial ? "hidden" : ""}"><button class="on" data-m="cnpj"><b>Pelo CNPJ</b><span class="muted">Dados da Receita Federal e IE pelo Sintegra/SEFAZ</span></button>
  <button data-m="cert"><b>Pelo certificado digital</b><span class="muted">Lê o CNPJ do e-CNPJ e consulta a IE na SEFAZ</span></button></div><div id="nova-passo"></div><div id="nova-form"></div>`);
  const passo = m => {
    $$(".passos button").forEach(b => b.classList.toggle("on", b.dataset.m === m)); $("#nova-form").innerHTML = "";
    $("#nova-passo").innerHTML = m === "cnpj"
      ? `<form id="fc" class="filtros" style="box-shadow:none"><label class="busca">CNPJ${raizFilial ? " da filial" : ""}<input name="cnpj" placeholder="00.000.000/0000-00" value="${raizFilial ? cnpjRaizFmt(raizFilial) : ""}" required autofocus></label><button class="btn primary">${svg("busca")} Consultar</button></form>`
      : `<form id="fc"><div id="cert-bloco"></div><div class="acoes"><button class="btn primary">${svg("chave")} Ler certificado</button></div></form>`;
    if (m === "cert") blocoCertificado($("#cert-bloco"));
    $("#fc").onsubmit = async ev => {
      ev.preventDefault(); const f = new FormData(ev.target), btn = $("#fc button.primary"); btn.disabled = true; btn.textContent = "Consultando…";
      try {
        let dados, cert;
        if (m === "cnpj") dados = await api("/cadastro/cnpj/" + encodeURIComponent(f.get("cnpj").replace(/[^0-9A-Za-z]/g, "")));
        else { const r = await api("/cadastro/certificado", { method: "POST", body: f }); dados = r.dados; cert = r.certificado; novaCert = certDoForm(f); }
        formEmpresa($("#nova-form"), dados, cert);
      } catch (e) { toast(esc(e.message), "var(--er)"); }
      btn.disabled = false; btn.innerHTML = m === "cnpj" ? `${svg("busca")} Consultar` : `${svg("chave")} Ler certificado`;
    };
  };
  $$(".passos button").forEach(b => b.onclick = () => passo(b.dataset.m)); passo("cnpj");
  if (raizFilial) { const i = $("#fc [name=cnpj]"); i.focus(); i.setSelectionRange(i.value.length, i.value.length); }
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
    <label>Consulta de NFS-e Nacional<select name="nfse_modo" id="nfse-modo"><option value="PROPRIO">Certificado da própria empresa</option><option value="PROCURACAO" ${d.nfse_modo === "PROCURACAO" ? "selected" : ""}>Procuração eletrônica (certificado do contador)</option></select></label>
    <label id="nfse-proc-l" class="${d.nfse_modo === "PROCURACAO" ? "" : "hidden"}">Procurador<select name="nfse_procurador_id" id="nfse-proc"><option value="">Carregando…</option></select></label>
    <label>Buscar notas recebidas automaticamente<select name="captura_automatica"><option value="1">Sim</option><option value="" ${existente && !d.captura_automatica ? "selected" : ""}>Não</option></select></label>
    ${String(d.cnpj || "").length === 14 && String(d.cnpj).slice(8, 12) !== "0001" ? `<label class="w2">Certificado da filial<select name="usar_cert_matriz"><option value="1">Usar o certificado da matriz (mesma raiz de CNPJ) quando a filial não tiver certificado próprio</option><option value="" ${existente && d.usar_cert_matriz === false ? "selected" : ""}>Somente certificado próprio da filial</option></select></label>` : ""}
  </div>${d.endereco || d.logradouro ? `<div class="doc-sub">${esc([d.logradouro, d.numero, d.bairro, d.municipio, d.uf, d.cep].filter(Boolean).join(", "))}</div>` : ""}
  <div class="acoes"><button class="btn primary">${existente ? "Salvar alterações" : "Cadastrar empresa"}</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`;
  const sel = $("#ie-sel"); if (sel) sel.onchange = () => { if (sel.value) $("#fe [name=ie]").value = sel.value; };
  $("#nfse-modo").onchange = () => $("#nfse-proc-l").classList.toggle("hidden", $("#nfse-modo").value !== "PROCURACAO");
  api("/procuradores").then(ps => { $("#nfse-proc").innerHTML = ps.length ? ps.map(p => `<option value="${p.id}" ${p.id === d.nfse_procurador_id ? "selected" : ""}>${esc(p.nome)} (${docFmt(p.documento)})</option>`).join("")
    : `<option value="">Nenhum procurador — cadastre em Configurações › Procuradores (contador)</option>`; }).catch(() => { });
  $("#fe").onsubmit = async ev => {
    ev.preventDefault(); const f = Object.fromEntries(new FormData(ev.target));
    const corpo = { ...f, nfse_procurador_id: +f.nfse_procurador_id || 0, ambiente: +f.ambiente, captura_automatica: !!f.captura_automatica, ciencia_automatica: !!existente?.ciencia_automatica, ativo: true, uf: f.uf.toUpperCase(), usar_cert_matriz: "usar_cert_matriz" in f ? !!f.usar_cert_matriz : true };
    try {
      const e = await api(existente ? `/empresas/${existente.id}` : "/empresas", { method: existente ? "PUT" : "POST", json: corpo });
      if (novaCert && !existente) await api(`/empresas/${e.id}/certificado`, { method: "POST", body: formCert(novaCert) });
      fecharModal(); estado.view = "empresas"; await mostrar("empresas");
      const nova = estado.empresas.find(x => x.id === e.id);
      toast(existente ? "Empresa atualizada" : `Empresa ${esc(e.razao_social)} cadastrada${nova && nova.tem_certificado && !nova.cert_proprio ? " · usará o certificado da matriz" : ""}`, "var(--ok)");
      if (!novaCert && !existente && !nova?.tem_certificado) certificadoEmpresa(e.id);
    } catch (er) { toast(esc(er.message), "var(--er)"); }
  };
}
function editarEmpresa(e) {
  modal(`<h3>Editar empresa</h3><div class="acoes" style="margin-top:0"><button class="btn sm" id="atualizar-rf">${svg("busca")} Atualizar pela Receita/SEFAZ</button></div><div id="ed-form"></div>`);
  formEmpresa($("#ed-form"), { ...e, regime: e.regime }, null, e);
  $("#atualizar-rf").onclick = async () => {
    try { const d = await api("/cadastro/cnpj/" + e.cnpj); formEmpresa($("#ed-form"), { ...d, ambiente: e.ambiente, captura_automatica: e.captura_automatica, regime: e.regime, nfse_modo: e.nfse_modo, nfse_procurador_id: e.nfse_procurador_id, usar_cert_matriz: e.usar_cert_matriz }, null, e); toast("Dados atualizados — confira e salve"); }
    catch (er) { toast(esc(er.message), "var(--er)"); }
  };
}
function certificadoEmpresa(id) {
  const emp = estado.empresas.find(e => e.id === id) || {};
  modal(`<h3>Certificado digital</h3><p class="muted">Necessário para consultar a SEFAZ, baixar as notas recebidas e manifestar. Escolha um certificado já instalado neste computador ou o arquivo .pfx.</p>
  <form id="fcert"><div id="cert-bloco"></div><div class="acoes"><button class="btn primary">Salvar certificado</button><button type="button" class="btn" onclick="fecharModal()">Agora não</button></div></form>`);
  blocoCertificado($("#cert-bloco"), (emp.cnpj || "").slice(0, 8));
  $("#fcert").onsubmit = async ev => {
    ev.preventDefault(); const b = $("#fcert button.primary"); b.disabled = true; b.textContent = "Salvando…";
    try { const c = await api(`/empresas/${id}/certificado`, { method: "POST", body: formCert(certDoForm(new FormData(ev.target))) }); toast(`Certificado incluído: ${esc(c.titular)}${c.tipo === "REPOSITORIO" ? " (usado direto do Windows)" : ""}`, "var(--ok)"); fecharModal(); mostrar("empresas"); }
    catch (e) { toast(esc(e.message), "var(--er)"); b.disabled = false; b.textContent = "Salvar certificado"; }
  };
}
async function removerCert(id) { if (confirm("Remover o certificado desta empresa?")) { await api(`/empresas/${id}/certificado`, { method: "DELETE" }); mostrar("empresas"); } }

/* ---------- Configurações ---------- */
VIEWS.config = async el => {
  const abas = [["usuarios", "Usuários"], ["monitoramento", "Monitoramento"], ["uf", "Alíquotas por UF"], ["mva", "MVA / ICMS-ST"], ["procuradores", "Procuradores (contador)"], ["integracao", "Integração com ERP"], ["backup", "Backup"], ["sistema", "Sistema"]];
  el.innerHTML = `<div class="tabs">${abas.map(([k, l]) => `<a class="${estado.abaCfg === k ? "on" : ""}" data-a="${k}">${l}</a>`).join("")}</div><div id="cfg"></div>`;
  $$(".tabs a", el).forEach(a => a.onclick = () => { estado.abaCfg = a.dataset.a; mostrar("config"); });
  await CFG[estado.abaCfg]($("#cfg"));
};
const CFG = {};
const PERFIS = { ADMIN: ["Administrador", "Tudo: empresas, certificados, usuários e configurações"], FISCAL: ["Fiscal", "Consulta, importa, busca na SEFAZ, manifesta e exporta"], CONSULTA: ["Consulta", "Somente visualiza, exporta e gera relatórios"] };
const perfilSel = (v, attr = "") => `<select ${attr}>${Object.entries(PERFIS).map(([k, [l]]) => `<option value="${k}" ${v === k ? "selected" : ""}>${l}</option>`).join("")}</select>`;
CFG.usuarios = async el => {
  const us = await api("/usuarios");
  el.innerHTML = `<div class="card"><h3 style="margin-top:0">Usuários do programa</h3>
  <p class="muted">${Object.values(PERFIS).map(([l, d]) => `<b>${l}</b> — ${d}`).join("<br>")}</p>
  <div class="tbl"><table><thead><tr><th>Usuário</th><th>Perfil</th><th>Situação</th><th>Último acesso</th><th></th></tr></thead><tbody>${us.map(u => `<tr data-u="${u.id}" class="${u.ativo ? "" : "inativo"}">
    <td><b>${esc(u.nome || u.login)}</b><div class="doc-sub">${esc(u.login)}${u.login === estado.eu?.login ? " · você" : ""}</div></td>
    <td>${perfilSel(u.perfil, `class="u-perfil" ${u.login === estado.eu?.login ? "disabled" : ""}`)}</td>
    <td><label class="chk-inline"><input type="checkbox" class="u-ativo" ${u.ativo ? "checked" : ""} ${u.login === estado.eu?.login ? "disabled" : ""}> Ativo</label></td>
    <td class="doc-sub">${u.ultimo_acesso ? dt(u.ultimo_acesso) : "nunca acessou"}</td>
    <td><div style="display:flex;gap:6px;justify-content:flex-end"><button class="btn sm u-editar">Editar</button><button class="btn sm u-senha">Redefinir senha</button>${u.login === estado.eu?.login ? "" : `<button class="btn sm danger u-excluir">Excluir</button>`}</div></td></tr>`).join("")}</tbody></table></div></div>
  <form id="fu" class="card" style="margin-top:16px"><h3 style="margin-top:0">Novo usuário</h3><div class="form">
    <label>Login<input name="login" required autocomplete="off"></label><label>Nome<input name="nome"></label>
    <label>Senha (mín. 8)<input type="password" name="senha" minlength="8" required autocomplete="new-password"></label><label>Perfil${perfilSel("FISCAL", `name="perfil"`)}</label></div>
    <div class="acoes"><button class="btn primary">${svg("mais")} Criar usuário</button></div></form>`;
  const salvar = async (id, dados, msg) => { try { await api(`/usuarios/${id}`, { method: "PUT", json: dados }); toast(msg || "Usuário atualizado", "var(--ok)"); } catch (e) { toast(esc(e.message), "var(--er)"); } mostrar("config", true); };
  $$("tr[data-u]", el).forEach(tr => {
    const u = us.find(x => String(x.id) === tr.dataset.u);
    $(".u-perfil", tr).onchange = ev => salvar(u.id, { perfil: ev.target.value });
    $(".u-ativo", tr).onchange = ev => salvar(u.id, { ativo: ev.target.checked }, ev.target.checked ? "Usuário reativado" : "Usuário desativado — não consegue mais entrar");
    $(".u-editar", tr).onclick = () => editarUsuario(u, salvar);
    $(".u-senha", tr).onclick = () => redefinirSenha(u, salvar);
    const ex = $(".u-excluir", tr);
    if (ex) ex.onclick = async () => { if (!confirm(`Excluir o usuário ${u.login}?`)) return; try { await api(`/usuarios/${u.id}`, { method: "DELETE" }); toast("Usuário excluído", "var(--ok)"); } catch (e) { toast(esc(e.message), "var(--er)"); } mostrar("config", true); };
  });
  $("#fu").onsubmit = async ev => { ev.preventDefault(); try { await api("/usuarios", { method: "POST", json: Object.fromEntries(new FormData(ev.target)) }); toast("Usuário criado", "var(--ok)"); mostrar("config"); } catch (e) { toast(esc(e.message), "var(--er)"); } };
};
function editarUsuario(u, salvar) {
  modal(`<h3>Editar usuário</h3><form id="feu"><div class="form"><label>Login<input value="${esc(u.login)}" readonly></label><label>Nome<input name="nome" value="${esc(u.nome)}"></label></div>
  <div class="acoes"><button class="btn primary">Salvar</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`);
  $("#feu").onsubmit = ev => { ev.preventDefault(); fecharModal(); salvar(u.id, { nome: new FormData(ev.target).get("nome") }); };
}
function redefinirSenha(u, salvar) {
  modal(`<h3>Redefinir senha de ${esc(u.nome || u.login)}</h3><p class="muted">As senhas salvas nos computadores deste usuário deixam de valer.</p><form id="frs"><div class="form">
  <label>Nova senha (mín. 8)<input type="password" name="senha" minlength="8" required autocomplete="new-password"></label><label>Repita a nova senha<input type="password" name="senha2" minlength="8" required autocomplete="new-password"></label></div>
  <div class="acoes"><button class="btn primary">Redefinir senha</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`);
  $("#frs").onsubmit = ev => { ev.preventDefault(); const f = new FormData(ev.target);
    if (f.get("senha") !== f.get("senha2")) { toast("As senhas não conferem", "var(--er)"); return; }
    fecharModal(); salvar(u.id, { senha: f.get("senha") }, "Senha redefinida"); };
}
function minhaSenha() {
  modal(`<h3>Minha senha</h3><p class="muted">Usuário <b>${esc(estado.eu?.login)}</b>. Depois da troca, a senha salva neste e em outros computadores deixa de valer.</p><form id="fms"><div class="form">
  <label class="w2">Senha atual<input type="password" name="atual" required autocomplete="current-password"></label>
  <label>Nova senha (mín. 8)<input type="password" name="nova" minlength="8" required autocomplete="new-password"></label><label>Repita a nova senha<input type="password" name="nova2" minlength="8" required autocomplete="new-password"></label></div>
  <div class="acoes"><button class="btn primary">Trocar senha</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`);
  $("#fms").onsubmit = async ev => { ev.preventDefault(); const f = new FormData(ev.target);
    if (f.get("nova") !== f.get("nova2")) { toast("As senhas novas não conferem", "var(--er)"); return; }
    try { await api("/auth/senha", { method: "POST", json: { atual: f.get("atual"), nova: f.get("nova") } }); ["vtx_cred", "vtx_auto"].forEach(k => LS.set(k, "")); fecharModal(); toast("Senha alterada", "var(--ok)"); }
    catch (e) { toast(esc(e.message), "var(--er)"); } };
}
$("#minha-senha").onclick = minhaSenha;
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
const UFS = "AC AL AM AP BA CE DF ES GO MA MG MS MT PA PB PE PI PR RJ RN RO RR RS SC SE SP TO".split(" ");
CFG.procuradores = async el => {
  const [ps, nsu] = await Promise.all([api("/procuradores"), api("/captura/status").catch(() => [])]);
  const ufSel = (v, attr) => `<select ${attr}><option value="">UF…</option>${UFS.map(u => `<option ${u === v ? "selected" : ""}>${u}</option>`).join("")}</select>`;
  el.innerHTML = `<div class="card"><h3 style="margin-top:0">Procuradores (contador)</h3>
    <p class="muted">O certificado do contador é usado de duas formas:<br>
    <b>NFS-e Nacional</b> — por <b>procuração eletrônica NFS-e</b>: em Empresas › Editar, escolha "Procuração eletrônica" e o procurador.<br>
    <b>NF-e</b> — a SEFAZ não aceita procuração; com o certificado do contador são baixadas as NF-e em que ele consta como <b>autorizado a obter o XML</b> (campo autXML da nota). Peça aos clientes que incluam o CPF/CNPJ do contador nesse campo no emissor. Só entram no monitor as notas das empresas cadastradas; as notas recebidas continuam exigindo o certificado da empresa. A busca é feita automaticamente e ao clicar em <b>Consultar</b> na tela NF-e / NFC-e.</p>
    <div class="tbl"><table><thead><tr><th>Procurador</th><th>Certificado</th><th>UF</th><th>Buscar NF-e (autXML)</th><th>Última busca de NF-e</th><th class="n">Empresas NFS-e</th><th></th></tr></thead><tbody>
    ${ps.map(p => { const n = nsu.find(x => x.empresa_id === -p.id && x.servico === "NFE_AUTXML") || {};
      return `<tr data-p="${p.id}"><td><b>${esc(p.nome)}</b><div class="doc-sub">${docFmt(p.documento)}</div></td><td class="doc-sub">${esc(p.titular)}${p.tipo === "REPOSITORIO" ? "<br>instalado no Windows" : ""}<br>válido até ${dia(p.valido_ate)}</td>
      <td>${ufSel(p.uf, `class="p-uf" style="width:76px"`)}</td><td><label class="chk-inline"><input type="checkbox" class="p-nfe" ${p.nfe_autxml ? "checked" : ""}> Ativa</label></td>
      <td class="doc-sub"><div style="max-width:300px;overflow-wrap:anywhere">${n.ultima_consulta ? `${dt(n.ultima_consulta)}${n.proxima_consulta ? ` · próxima ${dt(n.proxima_consulta)}` : ""}<br>${n.ultimo_c_stat ? n.ultimo_c_stat + " · " : ""}${esc(n.ultimo_motivo)}` : "ainda não realizada"}</div></td>
      <td class="n">${p.empresas}</td><td><button class="btn sm danger" onclick="remProcurador(${p.id})">Remover</button></td></tr>`; }).join("")
      || `<tr><td colspan="7" class="muted">Nenhum procurador cadastrado.</td></tr>`}</tbody></table></div></div>
    <form id="fproc" class="card" style="margin-top:16px"><h3 style="margin-top:0">Incluir procurador</h3>
    <div class="form"><label class="w2">Nome (opcional — usa o titular do certificado)<input name="nome"></label><label>UF do procurador${ufSel(estado.empresas[0]?.uf || "", `name="uf"`)}</label>
    <label class="chk-inline" style="align-self:end;white-space:normal"><input type="checkbox" name="nfe_autxml" checked> Buscar NF-e em que consta como autorizado (autXML)</label></div><div id="proc-cert" style="margin-top:10px"></div>
    <div class="acoes"><button class="btn primary">Incluir procurador</button></div></form>`;
  blocoCertificado($("#proc-cert"));
  $$("tr[data-p]", el).forEach(tr => {
    const id = tr.dataset.p, salvar = async () => {
      try { await api(`/procuradores/${id}`, { method: "PUT", json: { uf: $(".p-uf", tr).value, nfe_autxml: $(".p-nfe", tr).checked } }); toast("Procurador atualizado", "var(--ok)"); mostrar("config", true); }
      catch (e) { toast(esc(e.message), "var(--er)"); mostrar("config", true); } };
    $(".p-uf", tr).onchange = salvar; $(".p-nfe", tr).onchange = salvar;
  });
  $("#fproc").onsubmit = async ev => {
    ev.preventDefault(); const f = new FormData(ev.target), fd = formCert(certDoForm(f));
    fd.append("nome", f.get("nome") || ""); fd.append("uf", f.get("uf") || ""); if (f.get("nfe_autxml")) fd.append("nfe_autxml", "1");
    if (f.get("nfe_autxml") && !f.get("uf")) { toast("Informe a UF do procurador para buscar as NF-e", "var(--er)"); return; }
    try { const p = await api("/procuradores", { method: "POST", body: fd }); toast(`Procurador ${esc(p.nome)} incluído`, "var(--ok)"); mostrar("config"); } catch (e) { toast(esc(e.message), "var(--er)"); }
  };
};
async function remProcurador(id) { if (confirm("Remover este procurador? As empresas que o usam voltam a consultar NFS-e com o próprio certificado.")) { await api(`/procuradores/${id}`, { method: "DELETE" }); mostrar("config"); } }
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
  const campoPasta = (k, l, pem) => `<label class="w2">${l}<div class="com-botao"><input name="${k}" value="${esc(cf[k] ?? "")}" ${pem ? `placeholder="opcional"` : ""}><button type="button" class="btn sm" data-nav="${k}" data-pem="${pem ? 1 : ""}">${svg("docs")} Procurar…</button></div></label>`;
  el.innerHTML = `<form id="fs" class="card"><p class="muted">Pasta de dados: <code>${esc(c.diretorio_dados)}</code></p><div class="form">
  ${campoPasta("pasta_monitorada", "Pasta de importação de XML", false)}
  ${campoPasta("cadeia_icp_brasil", "Arquivo PEM da cadeia ICP-Brasil", true)}
  <label>Porta da API local<input name="porta" value="${esc(cf.porta ?? "")}"></label>
  <label>Verificar certificado do servidor da SEFAZ<select name="verificar_tls_sefaz"><option value="1">Sim (recomendado)</option><option value="" ${cf.verificar_tls_sefaz ? "" : "selected"}>Não</option></select></label>
  <label>Atualizar situação das notas emitidas na SEFAZ<select name="consultar_pendentes_sefaz"><option value="1">Sim</option><option value="" ${cf.consultar_pendentes_sefaz ? "" : "selected"}>Não</option></select></label></div>
  <div class="acoes"><button class="btn primary">Salvar</button></div><p class="muted">Alterar a porta exige reiniciar o programa.</p></form>`;
  $$("[data-nav]", el).forEach(b => b.onclick = () => {
    const inp = $(`[name=${b.dataset.nav}]`, el);
    navegador({ inicial: inp.value, arquivo: !!b.dataset.pem, ext: ".pem,.crt,.cer", titulo: b.dataset.pem ? "Escolher arquivo PEM da cadeia ICP-Brasil" : "Escolher a pasta de importação de XML", ok: v => { inp.value = v; } });
  });
  $("#fs").onsubmit = async ev => { ev.preventDefault(); const o = {};
    for (const [k, v] of new FormData(ev.target)) o[k] = ["verificar_tls_sefaz", "consultar_pendentes_sefaz"].includes(k) ? !!v : (/^(pasta_monitorada|cadeia_icp_brasil)$/.test(k) ? v : +String(v).replace(",", "."));
    try { await api("/config", { method: "PUT", json: o }); toast("Configuração salva", "var(--ok)"); } catch (e) { toast(esc(e.message), "var(--er)"); } };
};

// Monitoramento automático: o programa busca sozinho as notas novas de cada empresa
// no intervalo escolhido (NF-e na SEFAZ e NFS-e no Ambiente Nacional).
const ROT_MIN = m => m >= 60 && m % 60 === 0 ? `${m / 60} hora${m > 60 ? "s" : ""}` : `${m} minutos`;
CFG.monitoramento = async el => {
  const [c, nsu, procs] = await Promise.all([api("/config"), api("/captura/status").catch(() => []), api("/procuradores").catch(() => [])]), cf = c.config;
  const opNFe = [60, 65, 80, 120, 180, 240], opNFSe = [10, 15, 30, 60, 120, 240];
  const ivNFe = opNFe.includes(cf.intervalo_distdfe_min) ? cf.intervalo_distdfe_min : 65, ivNFSe = opNFSe.includes(cf.intervalo_nfse_min) ? cf.intervalo_nfse_min : 30;
  const ligado = cf.monitoramento_automatico !== false;
  const cel = (id, serv, ativo) => {
    if (!ativo) return `<td class="doc-sub">—</td>`;
    const n = nsu.find(x => x.empresa_id === id && x.servico === serv);
    if (!n) return `<td class="doc-sub">${ligado ? "na próxima verificação" : "aguardando"}</td>`;
    const erro = !n.ultimo_c_stat || [401, 403, 429, 656].includes(n.ultimo_c_stat);
    return `<td class="doc-sub"><div>Última: ${dt(n.ultima_consulta) || "—"}</div><div>Próxima: <b style="color:var(--tx)">${ligado ? dt(n.proxima_consulta) || "agora" : "desligado"}</b></div>
      <div class="motivo" style="${erro ? "color:var(--wa)" : ""}" title="${esc(n.ultimo_motivo)}">${n.ultimo_c_stat ? n.ultimo_c_stat + " · " : ""}${esc(n.ultimo_motivo)}</div></td>`;
  };
  const emps = empOrdem();
  el.innerHTML = `<form id="fmon" class="card"><h3 style="margin-top:0">Monitoramento automático</h3>
    <p class="muted">Com o monitoramento ligado, o programa verifica a cada minuto e busca as notas novas de cada empresa quando chega o horário definido abaixo — sem precisar clicar em Consultar. As notas novas aparecem nas telas em tempo real. O programa precisa estar aberto (a janela pode ficar minimizada); para monitorar com o computador sem ninguém logado, instale como serviço do Windows.</p>
    <div class="form">
      <label>Monitoramento automático<select name="monitoramento_automatico"><option value="1">Ligado</option><option value="" ${ligado ? "" : "selected"}>Desligado</option></select></label>
      <label>NF-e / NFC-e recebidas (SEFAZ)<select name="intervalo_distdfe_min">${opNFe.map(m => `<option value="${m}" ${m === ivNFe ? "selected" : ""}>A cada ${ROT_MIN(m)}${m === 65 ? " (recomendado)" : m === 60 ? " (mínimo da SEFAZ)" : m === 120 ? " (outro sistema usa o CNPJ)" : ""}</option>`).join("")}</select></label>
      <label>NFS-e (Ambiente Nacional)<select name="intervalo_nfse_min">${opNFSe.map(m => `<option value="${m}" ${m === ivNFSe ? "selected" : ""}>A cada ${ROT_MIN(m)}${m === 30 ? " (recomendado)" : ""}</option>`).join("")}</select></label>
    </div>
    <p class="muted" style="font-size:12.5px;margin:0">A SEFAZ só permite uma nova consulta do mesmo CNPJ 1 hora depois de responder "nenhum documento novo"; consultas antes disso geram bloqueio (656). Empresas com "Buscar notas recebidas automaticamente = Não" (Empresas › Editar) ficam fora do monitoramento.</p>
    <div class="acoes"><button class="btn primary">Salvar</button></div></form>
  <div class="card" style="margin-top:16px"><h3 style="margin-top:0">Situação por empresa</h3>
  ${emps.length ? `<div class="tbl"><table class="tab-mon"><thead><tr><th>Empresa</th><th>Automático</th><th>NF-e (SEFAZ)</th><th>NFS-e (Ambiente Nacional)</th></tr></thead><tbody>
  ${emps.map(e => { const nfse = e.tem_certificado || (e.nfse_modo === "PROCURACAO" && e.nfse_procurador_id);
    return `<tr><td><b>${docFmt(e.cnpj)}</b><div class="doc-sub">${esc(e.razao_social)}</div>${e.tem_certificado ? "" : `<span class="st CONTINGENCIA">sem certificado</span>`}</td>
    <td>${e.captura_automatica && ligado ? `<span class="st AUTORIZADA">Sim</span>` : `<span class="st CANCELADA">Não</span>`}</td>
    ${cel(e.id, "NFE_DISTDFE", e.tem_certificado)}${cel(e.id, "NFSE_ADN", nfse)}</tr>`; }).join("")}
  ${procs.filter(p => p.nfe_autxml && p.uf).map(p => `<tr><td><b>${esc(p.nome)}</b><div class="doc-sub">Procurador · NF-e em que consta como autorizado (autXML)</div></td><td>${ligado ? `<span class="st AUTORIZADA">Sim</span>` : `<span class="st CANCELADA">Não</span>`}</td>${cel(-p.id, "NFE_AUTXML", true)}<td class="doc-sub">—</td></tr>`).join("")}
  </tbody></table></div>` : `<p class="muted">Nenhuma empresa cadastrada.</p>`}</div>`;
  $("#fmon").onsubmit = async ev => { ev.preventDefault(); const f = Object.fromEntries(new FormData(ev.target));
    try { await api("/config", { method: "PUT", json: { monitoramento_automatico: !!f.monitoramento_automatico, intervalo_distdfe_min: +f.intervalo_distdfe_min, intervalo_nfse_min: +f.intervalo_nfse_min } });
      toast(f.monitoramento_automatico ? `Monitoramento ligado · NF-e a cada ${ROT_MIN(+f.intervalo_distdfe_min)}, NFS-e a cada ${ROT_MIN(+f.intervalo_nfse_min)}` : "Monitoramento automático desligado", "var(--ok)"); mostrar("config", true); }
    catch (e) { toast(esc(e.message), "var(--er)"); } };
};

CFG.backup = async el => {
  const [c, b] = await Promise.all([api("/config"), api("/backup")]), cf = c.config;
  const tam = n => n > 1048576 ? (n / 1048576).toFixed(1) + " MB" : Math.ceil((n || 0) / 1024) + " KB";
  el.innerHTML = `<div class="grid g2">
  <form id="fb" class="card"><h3 style="margin-top:0">Backup automático</h3>
    <p class="muted">Copia o banco (notas, eventos, cadastros e configurações) e todos os XMLs para outra pasta — pendrive, disco externo, rede ou uma pasta sincronizada com a nuvem (OneDrive, Google Drive). Os XMLs são copiados de forma incremental (só os novos) e nunca são apagados do destino.</p>
    <div class="form">
      <label class="w2">Pasta de destino<div class="com-botao"><input name="backup_pasta" value="${esc(cf.backup_pasta || "")}" placeholder="ex.: E:\\ ou C:\\Users\\voce\\OneDrive"><button type="button" class="btn sm" id="bk-proc">${svg("docs")} Procurar…</button></div></label>
      <label>Frequência<select name="backup_frequencia">${[["", "Desativado"], ["diario", "Diário"], ["semanal", "Semanal"]].map(([k, l]) => `<option value="${k}" ${(cf.backup_frequencia || "") === k ? "selected" : ""}>${l}</option>`).join("")}</select></label>
      <label>Cópias do banco mantidas<select name="backup_manter">${[3, 7, 14, 30, 60].map(n => `<option ${(cf.backup_manter || 7) === n ? "selected" : ""}>${n}</option>`).join("")}</select></label>
    </div>
    <div class="acoes"><button class="btn primary">Salvar</button><button type="button" class="btn" id="bk-agora" ${b.em_andamento ? "disabled" : ""}>${b.em_andamento ? "Backup em andamento…" : "Fazer backup agora"}</button></div></form>
  <div class="card"><h3 style="margin-top:0">Situação</h3><div class="kv">
    <b>Último backup</b><span>${b.ultimo ? dt(b.ultimo) + (b.duracao_seg ? ` (${b.duracao_seg}s)` : "") : "nenhum realizado"}</span>
    <b>Próximo</b><span>${b.proximo ? dt(b.proximo) : cf.backup_frequencia ? "assim que a pasta for escolhida" : "backup automático desativado"}</span>
    ${b.arquivo ? `<b>Cópia do banco</b><span style="word-break:break-all">${esc(b.arquivo)} · ${tam(b.tamanho_banco)}</span><b>XMLs</b><span>${b.xml_total} no backup · ${b.xml_copiados} novos na última cópia</span>` : ""}
    ${b.erro ? `<b>Erro</b><span style="color:var(--er)">${esc(b.erro)}</span>` : ""}</div>
    <p class="muted" style="font-size:12.5px;margin-top:14px">Restaurar: feche o programa e copie <code>vorticis.db</code>, <code>config.json</code> e <code>chave.mestra</code> do ZIP mais recente (pasta <code>banco</code>) e a pasta <code>xml</code> para a pasta de dados <code>${esc(c.diretorio_dados)}</code>. O LEIA-ME dentro de cada ZIP explica o passo a passo. Em outro computador, inclua os certificados digitais novamente.</p></div></div>`;
  $("#bk-proc").onclick = () => navegador({ inicial: $("[name=backup_pasta]", el).value, titulo: "Escolher a pasta de destino do backup", ok: v => { $("[name=backup_pasta]", el).value = v; } });
  $("#fb").onsubmit = async ev => { ev.preventDefault(); const f = Object.fromEntries(new FormData(ev.target));
    if (f.backup_frequencia && !f.backup_pasta.trim()) { toast("Escolha a pasta de destino do backup", "var(--er)"); return; }
    try { await api("/config", { method: "PUT", json: { backup_pasta: f.backup_pasta.trim(), backup_frequencia: f.backup_frequencia, backup_manter: +f.backup_manter } }); toast("Backup configurado", "var(--ok)"); mostrar("config", true); }
    catch (e) { toast(esc(e.message), "var(--er)"); } };
  $("#bk-agora").onclick = async ev => { const bt = ev.target; bt.disabled = true; bt.textContent = "Copiando…";
    try { await api("/config", { method: "PUT", json: { backup_pasta: $("[name=backup_pasta]", el).value.trim() } }); const r = await api("/backup/executar", { method: "POST", tempo: 30 * 60000 }); toast(`Backup concluído · ${r.xml_copiados} XML novo(s)`, "var(--ok)"); }
    catch (e) { toast(esc(e.message), "var(--er)"); }
    mostrar("config", true); };
};

// Navegador de pastas/arquivos do computador (o caminho é lido pelo próprio programa).
function navegador({ inicial, arquivo, ext, titulo, ok }) {
  modal(`<h3>${esc(titulo)}</h3><div class="nav-cab"><button class="btn sm" id="nv-sobe" title="Pasta acima">↑</button><input id="nv-cam" spellcheck="false"><button class="btn sm" id="nv-ir">Ir</button></div>
    <div class="nav-corpo"><div class="nav-atalhos" id="nv-at"></div><div class="nav-lista" id="nv-lista"></div></div>
    <div class="acoes"><span class="muted" id="nv-sel" style="flex:1"></span>${arquivo ? "" : `<button type="button" class="btn" id="nv-nova">${svg("mais")} Nova pasta</button>`}
    <button class="btn primary" id="nv-ok" ${arquivo ? "disabled" : ""}>${arquivo ? "Selecionar arquivo" : "Selecionar esta pasta"}</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div>`);
  let atual = "", escolhido = "";
  const abrir = async c => {
    try {
      const r = await api(`/sistema/pastas?caminho=${encodeURIComponent(c)}${arquivo ? "&ext=" + ext : ""}`);
      atual = r.caminho; $("#nv-cam").value = r.caminho === "*" ? "Este computador" : r.caminho; $("#nv-sobe").disabled = !r.pai;
      $("#nv-sobe").onclick = () => r.pai && abrir(r.pai);
      $("#nv-at").innerHTML = r.atalhos.map(a => `<a data-p="${esc(a.caminho)}">${esc(a.nome)}</a>`).join("");
      $$("#nv-at a").forEach(a => a.onclick = () => abrir(a.dataset.p));
      $("#nv-lista").innerHTML = [...r.pastas.map(p => `<a class="pasta" data-p="${esc(p.caminho)}">📁 ${esc(p.nome)}</a>`), ...r.arquivos.map(p => `<a class="arq" data-f="${esc(p.caminho)}">📄 ${esc(p.nome)}</a>`)].join("")
        || `<div class="muted" style="padding:12px">${arquivo ? "Nenhuma subpasta nem arquivo .pem/.crt/.cer aqui." : "Nenhuma subpasta."}</div>`;
      $$("#nv-lista a.pasta").forEach(a => a.ondblclick = a.onclick = () => abrir(a.dataset.p));
      $$("#nv-lista a.arq").forEach(a => { a.onclick = () => { $$("#nv-lista a.arq").forEach(x => x.classList.remove("on")); a.classList.add("on"); escolhido = a.dataset.f; $("#nv-ok").disabled = false; $("#nv-sel").textContent = escolhido; }; a.ondblclick = () => { a.onclick(); $("#nv-ok").click(); }; });
      if (!arquivo) { $("#nv-ok").disabled = atual === "*"; $("#nv-sel").textContent = atual === "*" ? "" : atual; }
    } catch (e) { toast(esc(e.message), "var(--er)"); if (c !== "") abrir(""); }
  };
  $("#nv-ir").onclick = () => abrir($("#nv-cam").value);
  $("#nv-cam").onkeydown = e => { if (e.key === "Enter") abrir($("#nv-cam").value); };
  $("#nv-ok").onclick = () => { ok(arquivo ? escolhido : atual); fecharModal(); };
  const nova = $("#nv-nova");
  if (nova) nova.onclick = async () => { const n = prompt("Nome da nova pasta:"); if (!n) return; try { const r = await api("/sistema/pastas", { method: "POST", json: { caminho: atual, nome: n } }); abrir(r.caminho); } catch (e) { toast(esc(e.message), "var(--er)"); } };
  const ini = (inicial || "").trim();
  abrir(arquivo && ini ? ini.replace(/[\\/][^\\/]*$/, "") : ini);
}

iniciar();
