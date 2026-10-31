/* Vorticis Monitor — painel web (sem dependências externas). */
"use strict";
const $ = (s, el = document) => el.querySelector(s);
const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const brl = v => (v || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
const num = v => (v || 0).toLocaleString("pt-BR", { maximumFractionDigits: 2 });
const dt = s => { if (!s) return ""; const d = new Date(s); return isNaN(d) ? s : d.toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" }); };
const docFmt = d => { d = String(d || ""); return d.length === 14 ? d.replace(/^(..)(...)(...)(....)(..)$/, "$1.$2.$3/$4-$5") : d.length === 11 ? d.replace(/^(...)(...)(...)(..)$/, "$1.$2.$3-$4") : d; };
const ST = { AUTORIZADA: "Autorizada", CANCELADA: "Cancelada", DENEGADA: "Denegada", REJEITADA: "Rejeitada", PENDENTE: "Pendente", CONTINGENCIA: "Contingência", BLOQUEADA: "Bloqueada (pré-validação)", RESUMO: "Resumo (aguarda XML)", INUTILIZADA: "Inutilizada" };
const MOD = { NFE: "NF-e", NFCE: "NFC-e", CTE: "CT-e", MDFE: "MDF-e", NFSE: "NFS-e" };
const stTag = s => `<span class="st ${esc(s)}">${esc(ST[s] || s)}</span>`;
const svTag = s => `<span class="st sv-${esc(s)}">${esc(s)}</span>`;
const scoreTag = n => `<span class="score ${n >= 85 ? "b" : n >= 60 ? "m" : "r"}">${n}</span>`;

const estado = { token: localStorage.getItem("vtx_token") || "", empresa: localStorage.getItem("vtx_emp") || "", view: "painel", empresas: [], filtroStatus: "", docModelo: "", pagina: 1 };
let es = null;

async function api(path, opt = {}) {
  opt.headers = Object.assign({ Authorization: "Bearer " + estado.token }, opt.headers || {});
  if (opt.json !== undefined) { opt.body = JSON.stringify(opt.json); opt.headers["Content-Type"] = "application/json"; }
  const r = await fetch("/api/v1" + path, opt);
  if (r.status === 401 && !path.startsWith("/auth")) { sair(); throw new Error("Sessão expirada"); }
  const ct = r.headers.get("content-type") || "";
  const body = ct.includes("json") ? await r.json() : await r.text();
  if (!r.ok && r.status !== 422) throw new Error(body.erro || body || r.statusText);
  return body;
}
function toast(msg, cor) {
  const t = document.createElement("div"); t.className = "toast"; if (cor) t.style.borderLeftColor = cor;
  t.innerHTML = msg; $("#toasts").appendChild(t); setTimeout(() => t.remove(), 6000);
}
const empQS = () => estado.empresa ? "empresa_id=" + estado.empresa : "";
const link = (p) => "/api/v1" + p + (p.includes("?") ? "&" : "?") + "token=" + encodeURIComponent(estado.token);

/* ---------- login ---------- */
async function iniciar() {
  const e = await fetch("/api/v1/auth/estado").then(r => r.json());
  if (e.precisa_setup) {
    $("#login").classList.remove("hidden"); $("#setup-campos").classList.remove("hidden");
    $("#login-sub").textContent = "Primeiro acesso: crie o usuário administrador"; $("#btn-login").textContent = "Criar administrador";
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
    if (setup) mostrar("config");
  } catch (e) { $("#login-erro").textContent = e.message; }
};
function sair() { localStorage.removeItem("vtx_token"); estado.token = ""; location.reload(); }
$("#sair").onclick = sair;

function entrar() {
  $("#app").classList.remove("hidden"); conectarSSE(); mostrar(estado.view); atualizarBadge();
}
async function carregarEmpresas() {
  estado.empresas = await api("/empresas");
  const sel = $("#empresa-sel");
  sel.innerHTML = `<option value="">Todas as empresas</option>` + estado.empresas.map(e => `<option value="${e.id}">${esc(e.razao_social)} — ${docFmt(e.cnpj)}</option>`).join("");
  sel.value = estado.empresa;
  const sem = estado.empresas.filter(e => !e.tem_certificado);
  const b = $("#banner-cert");
  if (!estado.empresas.length) { b.innerHTML = `Nenhuma empresa cadastrada. <a onclick="mostrar('config')">Cadastre a empresa e inclua o certificado digital</a> para habilitar consultas à SEFAZ.`; b.classList.remove("hidden"); }
  else if (sem.length) { b.innerHTML = `⚠ ${sem.length} empresa(s) sem certificado digital A1: ${sem.map(e => esc(e.razao_social)).join(", ")}. <a onclick="mostrar('config')">Incluir certificado</a> para consultar a SEFAZ, capturar XMLs destinados e manifestar.`; b.classList.remove("hidden"); }
  else b.classList.add("hidden");
}
$("#empresa-sel").onchange = e => { estado.empresa = e.target.value; localStorage.setItem("vtx_emp", estado.empresa); mostrar(estado.view); };
let tBusca; $("#busca").oninput = () => { clearTimeout(tBusca); tBusca = setTimeout(() => { estado.pagina = 1; mostrar("documentos"); }, 350); };
$("#upload").onchange = async ev => {
  const fd = new FormData(); [...ev.target.files].forEach(f => fd.append("arquivo", f, f.name));
  try {
    const r = await api("/documentos/importar", { method: "POST", body: fd });
    const err = r.resultados.filter(x => x.status === "ERRO").length;
    toast(`<b>${r.processados}</b> arquivo(s) processado(s)${err ? `, <b style="color:var(--er)">${err} com erro</b>` : ""}`);
    mostrar(estado.view);
  } catch (e) { toast(esc(e.message), "var(--er)"); }
  ev.target.value = "";
};

/* ---------- tempo real ---------- */
let tRefresh;
function conectarSSE() {
  if (es) es.close();
  es = new EventSource(link("/eventos/stream"));
  es.addEventListener("conectado", () => { $("#sse").className = "dot on"; $("#sse-txt").textContent = "tempo real"; });
  es.onerror = () => { $("#sse").className = "dot off"; $("#sse-txt").textContent = "reconectando…"; };
  const refresh = () => { clearTimeout(tRefresh); tRefresh = setTimeout(() => { if (["painel", "documentos", "rejeicoes", "alertas", "entradas", "servicos"].includes(estado.view) && $("#drawer").classList.contains("hidden")) mostrar(estado.view, true); }, 800); };
  es.addEventListener("documento.novo", e => { const d = JSON.parse(e.data).dados; toast(`Novo documento: ${MOD[d.modelo] || d.modelo} nº ${esc(d.numero)} ${stTag(d.status)}`); refresh(); });
  es.addEventListener("documento.status", refresh);
  es.addEventListener("alerta.novo", e => { const a = JSON.parse(e.data).dados; if (a.titulo) toast(`🔔 ${esc(a.titulo)}`, a.severidade === "ERRO" ? "var(--er)" : "var(--wa)"); atualizarBadge(); });
}
async function atualizarBadge() {
  const a = await api("/alertas?abertos=1").catch(() => []);
  const b = $("#badge-alertas"); b.textContent = a.length; b.classList.toggle("hidden", !a.length);
}

/* ---------- navegação ---------- */
document.querySelectorAll("#menu a").forEach(a => a.onclick = () => { estado.pagina = 1; mostrar(a.dataset.v); });
async function mostrar(v, silencioso) {
  estado.view = v;
  document.querySelectorAll("#menu a").forEach(a => a.classList.toggle("ativo", a.dataset.v === v));
  const el = $("#view");
  if (!silencioso) el.innerHTML = `<p class="muted">Carregando…</p>`;
  try { await (VIEWS[v] || VIEWS.painel)(el); } catch (e) { el.innerHTML = `<div class="card erro">${esc(e.message)}</div>`; }
}

/* ---------- Painel ---------- */
const VIEWS = {};
VIEWS.painel = async el => {
  const r = await api("/dashboard/resumo?" + empQS());
  const s = r.por_status || {};
  const kpi = (st, cor) => `<div class="card kpi click" onclick="estado.filtroStatus='${st}';mostrar('documentos')"><div class="l">${ST[st]}</div><div class="n" style="color:${cor}">${s[st] || 0}</div></div>`;
  const ent = r.valor_por_direcao?.E || 0, sai = r.valor_por_direcao?.S || 0;
  const t = r.tributos || {};
  el.innerHTML = `<h2>Painel fiscal</h2>
  <div class="grid g4">
    ${kpi("AUTORIZADA", "var(--ok)")}${kpi("PENDENTE", "var(--in)")}${kpi("CONTINGENCIA", "var(--wa)")}${kpi("REJEITADA", "var(--er)")}
    ${kpi("DENEGADA", "var(--er)")}${kpi("CANCELADA", "var(--gr)")}${kpi("BLOQUEADA", "var(--er)")}${kpi("RESUMO", "var(--in)")}
  </div>
  <div class="grid g4" style="margin-top:14px">
    <div class="card kpi"><div class="l">Score fiscal médio</div><div class="n">${scoreTag(Math.round(r.score_medio || 0))}</div><div class="bar"><i style="width:${r.score_medio || 0}%"></i></div></div>
    <div class="card kpi click" onclick="mostrar('alertas')"><div class="l">Alertas abertos</div><div class="n" style="color:var(--wa)">${r.alertas_abertos}</div></div>
    <div class="card kpi click" onclick="mostrar('entradas')"><div class="l">NF-e sem manifestação conclusiva</div><div class="n">${r.sem_manifestacao}</div></div>
    <div class="card kpi click" onclick="mostrar('entradas')"><div class="l">Conciliações divergentes</div><div class="n">${r.conciliacao_divergente}</div></div>
  </div>
  <div class="grid g2" style="margin-top:14px">
    <div class="card"><h3>Movimento autorizado</h3><div class="kv"><b>Saídas</b><span>${brl(sai)}</span><b>Entradas</b><span>${brl(ent)}</span>
      ${Object.entries(r.por_modelo || {}).map(([k, v]) => `<b>${MOD[k] || k}</b><span>${v} documento(s)</span>`).join("")}</div></div>
    <div class="card"><h3>Tributos destacados</h3><div class="kv"><b>ICMS</b><span>${brl(t.icms)}</span><b>ICMS-ST</b><span>${brl(t.icms_st)}</span><b>DIFAL (UF destino)</b><span>${brl(t.difal)}</span>
      <b>IPI</b><span>${brl(t.ipi)}</span><b>PIS / COFINS</b><span>${brl(t.pis)} / ${brl(t.cofins)}</span><b>ISS</b><span>${brl(t.iss)}</span><b>Retenções (NFS-e)</b><span>${brl(t.retencoes)}</span></div></div>
    <div class="card"><h3>Inconsistências mais frequentes</h3>${(r.top_regras || []).length ? `<table><tbody>${r.top_regras.map(x => `<tr onclick="mostrar('documentos')"><td>${svTag(x.severidade)}</td><td><b>${esc(x.regra)}</b> ${esc(x.exemplo)}</td><td class="n">${x.quantidade}</td></tr>`).join("")}</tbody></table>` : `<p class="muted">Nenhuma inconsistência.</p>`}</div>
    <div class="card"><h3>Rejeições SEFAZ</h3>${(r.top_rejeicoes || []).length ? `<table><tbody>${r.top_rejeicoes.map(x => `<tr onclick="mostrar('rejeicoes')"><td><b>${x.codigo}</b></td><td>${esc(x.descricao)}</td><td class="n">${x.quantidade}</td></tr>`).join("")}</tbody></table>` : `<p class="muted">Nenhuma rejeição no período.</p>`}</div>
  </div>
  <h3 style="margin-top:20px">Últimos documentos</h3><div id="ultimos"></div>`;
  const d = await api("/documentos?por_pagina=12&" + empQS());
  $("#ultimos").innerHTML = gridDocs(d.itens);
};

function gridDocs(itens) {
  if (!itens.length) return `<div class="card muted">Nenhum documento. Use <b>Importar XML/ZIP</b>, a pasta monitorada ou a API do ERP.</div>`;
  return `<div class="tbl"><table><thead><tr><th>Status</th><th>Modelo</th><th>Mov.</th><th>Nº</th><th>Emissão</th><th>Emitente / Destinatário</th><th class="n">Valor</th><th>SEFAZ</th><th class="n">Score</th></tr></thead><tbody>
  ${itens.map(d => `<tr onclick="abrirDoc('${esc(d.chave)}')"><td>${stTag(d.status)}${d.tp_emis && d.tp_emis !== "1" ? ` <span class="st CONTINGENCIA">tpEmis ${esc(d.tp_emis)}</span>` : ""}</td>
  <td>${MOD[d.modelo] || esc(d.modelo)}</td><td>${d.direcao === "E" ? "Entrada" : "Saída"}</td><td>${esc(d.numero)}</td><td>${dt(d.dh_emissao)}</td>
  <td>${esc(d.direcao === "E" ? d.emit_nome : d.dest_nome)}<br><span class="muted">${docFmt(d.direcao === "E" ? d.emit_doc : d.dest_doc)}</span></td>
  <td class="n">${brl(d.valor_total)}</td><td>${d.c_stat ? `<b>${d.c_stat}</b> ` : ""}<span class="muted">${esc((d.x_motivo || "").slice(0, 50))}</span></td>
  <td class="n">${scoreTag(d.score)}${d.erros ? `<br><span class="muted">${d.erros} erro(s)</span>` : ""}</td></tr>`).join("")}</tbody></table></div>`;
}

/* ---------- Documentos ---------- */
VIEWS.documentos = async el => {
  const chips = ["", "AUTORIZADA", "PENDENTE", "CONTINGENCIA", "REJEITADA", "DENEGADA", "CANCELADA", "BLOQUEADA", "RESUMO"];
  const qs = new URLSearchParams({ por_pagina: 50, pagina: estado.pagina, status: estado.filtroStatus, modelo: estado.docModelo, q: $("#busca").value });
  if (estado.empresa) qs.set("empresa_id", estado.empresa);
  const d = await api("/documentos?" + qs);
  const pags = Math.ceil(d.total / 50) || 1;
  el.innerHTML = `<h2>Documentos fiscais <span class="muted" style="font-size:14px">(${d.total})</span></h2>
  <div class="chips">${chips.map(c => `<span class="chip ${estado.filtroStatus === c ? "on" : ""}" onclick="estado.filtroStatus='${c}';estado.pagina=1;mostrar('documentos')">${c ? ST[c] : "Todos"}</span>`).join("")}
  <select onchange="estado.docModelo=this.value;mostrar('documentos')"><option value="">Todos os modelos</option>${Object.entries(MOD).map(([k, v]) => `<option value="${k}" ${estado.docModelo === k ? "selected" : ""}>${v}</option>`).join("")}</select>
  <a class="btn sm" href="${link("/relatorios/xml-lote?" + qs)}">⬇ ZIP (XML + DANFE)</a></div>
  ${gridDocs(d.itens)}
  <div class="acoes"><button class="btn sm" ${estado.pagina <= 1 ? "disabled" : ""} onclick="estado.pagina--;mostrar('documentos')">‹ Anterior</button><span class="muted">Página ${estado.pagina} de ${pags}</span><button class="btn sm" ${estado.pagina >= pags ? "disabled" : ""} onclick="estado.pagina++;mostrar('documentos')">Próxima ›</button></div>`;
};

async function abrirDoc(chave) {
  const r = await api("/documentos/" + encodeURIComponent(chave));
  const d = r.documento, dr = $("#drawer");
  const ocs = r.ocorrencias || [];
  $("#drawer-body").innerHTML = `<div style="display:flex;justify-content:space-between;align-items:center"><h2>${MOD[d.modelo] || d.modelo} nº ${esc(d.numero)} ${stTag(d.status)}</h2><button class="btn" onclick="fecharDrawer()">✕</button></div>
  <div class="acoes">
    <a class="btn" target="_blank" href="${link("/documentos/" + chave + "/danfe")}">🖨 DANFE</a>
    <a class="btn" href="${link("/documentos/" + chave + "/xml")}">⬇ XML</a>
    ${d.direcao === "S" || d.resumo ? `<button class="btn" onclick="acao('${chave}','consultar-sefaz')">Consultar SEFAZ</button>` : ""}
    <button class="btn" onclick="acao('${chave}','revalidar')">Revalidar regras</button>
    ${d.direcao === "E" && d.modelo === "NFE" ? `<button class="btn" onclick="manifestar('${chave}')">Manifestar</button><button class="btn" onclick="conciliar('${chave}')">Conciliar com pedido</button>` : ""}
    ${ocs.length ? `<button class="btn" onclick="verCorrecoes('${chave}')">Correções sugeridas</button>` : ""}
  </div>
  ${r.rejeicao ? `<div class="card" style="border-color:var(--er);margin-bottom:12px"><h3>Rejeição ${r.rejeicao.codigo} — ${esc(r.rejeicao.descricao_oficial)}</h3><p>${esc(r.rejeicao.explicacao)}</p><p><b>Como corrigir:</b> ${esc(r.rejeicao.sugestao)}</p></div>` : ""}
  <div class="grid g2"><div class="card"><div class="kv">
    <b>Chave</b><span style="word-break:break-all">${esc(d.chave)}</span><b>Movimento</b><span>${d.direcao === "E" ? "Entrada" : "Saída"} — ${esc(d.tipo_operacao)}</span>
    <b>Emissão</b><span>${dt(d.dh_emissao)}</span><b>Autorização</b><span>${dt(d.dh_autorizacao)} ${esc(d.protocolo)}</span>
    <b>Retorno SEFAZ</b><span>${d.c_stat || ""} ${esc(d.x_motivo)}</span><b>Natureza</b><span>${esc(d.natureza)}</span>
    <b>Origem</b><span>${esc(d.origem)}</span><b>Manifestação</b><span>${esc(d.manifestacao || "—")}</span><b>Conciliação</b><span>${esc(d.conciliacao || "—")}</span>
  </div></div><div class="card"><div class="kv">
    <b>Emitente</b><span>${esc(d.emit_nome)}<br>${docFmt(d.emit_doc)} ${esc(d.emit_uf)}</span><b>Destinatário</b><span>${esc(d.dest_nome)}<br>${docFmt(d.dest_doc)} ${esc(d.dest_uf)}</span>
    <b>Valor total</b><span><b>${brl(d.valor_total)}</b></span><b>ICMS / ST</b><span>${brl(d.valor_icms)} / ${brl(d.valor_st)}</span><b>DIFAL</b><span>${brl(d.valor_difal)}</span>
    <b>IPI</b><span>${brl(d.valor_ipi)}</span><b>PIS / COFINS</b><span>${brl(d.valor_pis)} / ${brl(d.valor_cofins)}</span>${d.valor_iss ? `<b>ISS / Retenções</b><span>${brl(d.valor_iss)} / ${brl(d.valor_retencoes)}</span>` : ""}
    <b>Score fiscal</b><span>${scoreTag(d.score)}</span></div></div></div>
  <h3 style="margin-top:16px">Auditoria fiscal (${ocs.length})</h3>
  ${ocs.length ? ocs.map(o => `<div class="oc ${o.severidade}"><div>${svTag(o.severidade)} <b>${esc(o.mensagem)}</b>${o.cod_rejeicao ? ` <span class="st REJEITADA">Rejeição ${o.cod_rejeicao}</span>` : ""}</div>
    <div class="meta">${esc(o.regra)} · ${esc(o.grupo)}${o.item ? " · item " + o.item : ""} · ${esc(o.campo)}${o.esperado ? ` · esperado <b>${esc(o.esperado)}</b>` : ""}${o.informado ? ` · informado <b>${esc(o.informado)}</b>` : ""}</div>
    ${o.sugestao ? `<div class="meta">💡 ${esc(o.sugestao)}</div>` : ""}</div>`).join("") : `<div class="card muted">✔ Nenhuma inconsistência encontrada pelo motor de regras.</div>`}
  ${r.itens?.length ? `<h3 style="margin-top:16px">Itens</h3><div class="tbl"><table><thead><tr><th>#</th><th>Produto</th><th>NCM</th><th>CEST</th><th>CFOP</th><th>CST</th><th class="n">Qtd</th><th class="n">Total</th><th class="n">ICMS</th><th class="n">ST</th><th class="n">IPI</th><th>PIS/COF</th></tr></thead><tbody>
    ${r.itens.map(i => `<tr><td>${i.n_item}</td><td>${esc(i.cod_prod)} ${esc(i.descricao)}</td><td>${esc(i.ncm)}</td><td>${esc(i.cest)}</td><td>${esc(i.cfop)}</td><td>${esc(i.cst_icms)}</td><td class="n">${num(i.quantidade)}</td><td class="n">${brl(i.valor_total)}</td><td class="n">${num(i.v_icms)}</td><td class="n">${num(i.v_icms_st)}</td><td class="n">${num(i.v_ipi)}</td><td>${esc(i.cst_pis)}/${esc(i.cst_cofins)}</td></tr>`).join("")}</tbody></table></div>` : ""}
  ${r.eventos?.length ? `<h3 style="margin-top:16px">Eventos</h3><div class="tbl"><table><tbody>${r.eventos.map(e => `<tr><td>${esc(e.tp_evento)}</td><td>${esc(e.descricao)}</td><td>${dt(e.dh_evento)}</td><td>${e.c_stat} ${esc(e.x_motivo)}</td><td>${esc(e.protocolo)}</td></tr>`).join("")}</tbody></table></div>` : ""}`;
  dr.classList.remove("hidden");
}
function fecharDrawer() { $("#drawer").classList.add("hidden"); mostrar(estado.view, true); }
$("#drawer").onclick = e => { if (e.target.id === "drawer") fecharDrawer(); };
async function acao(chave, a) {
  try { const r = await api(`/documentos/${chave}/${a}`, { method: "POST" }); toast(a === "revalidar" ? `Revalidado: ${r.erros} erro(s), ${r.alertas} alerta(s)` : `SEFAZ: ${r.c_stat} ${esc(r.x_motivo)}`); abrirDoc(chave); }
  catch (e) { toast(esc(e.message), "var(--er)"); }
}
function modal(html) { $("#modal-body").innerHTML = html; $("#modal").classList.remove("hidden"); }
function fecharModal() { $("#modal").classList.add("hidden"); }
$("#modal").onclick = e => { if (e.target.id === "modal") fecharModal(); };
function manifestar(chave) {
  modal(`<h3>Manifestação do destinatário</h3><form id="fm" class="grid"><label>Evento<select name="tipo"><option value="210210">Ciência da Operação (libera download do XML)</option><option value="210200">Confirmação da Operação</option><option value="210220">Desconhecimento da Operação</option><option value="210240">Operação não Realizada</option></select></label>
  <label>Justificativa (obrigatória para "não realizada", 15–255 caracteres)<textarea name="just" rows="3"></textarea></label><div class="acoes"><button class="btn primary">Enviar à SEFAZ</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`);
  $("#fm").onsubmit = async ev => { ev.preventDefault(); const f = new FormData(ev.target);
    try { const r = await api(`/documentos/${chave}/manifestacao`, { method: "POST", json: { tipo: f.get("tipo"), justificativa: f.get("just") } }); toast(`Evento: ${r.evento_c_stat} ${esc(r.evento_x_motivo)}`); fecharModal(); abrirDoc(chave); }
    catch (e) { toast(esc(e.message), "var(--er)"); } };
}
async function conciliar(chave) {
  try {
    const c = await api(`/documentos/${chave}/conciliar`, { method: "POST", json: {} });
    modal(`<h3>Conciliação: ${stTag(c.status === "CONCILIADO" ? "AUTORIZADA" : "REJEITADA").replace(/>.*</, ">" + esc(c.status) + "<")} ${c.pedido_numero ? "Pedido " + esc(c.pedido_numero) : ""}</h3>
    ${(c.divergencias || []).length ? (c.divergencias).map(d => `<div class="oc ${d.bloqueia ? "ERRO" : "ALERTA"}"><b>${esc(d.tipo)}</b> ${esc(d.mensagem)}<div class="meta">${d.n_item ? "Item XML " + d.n_item : ""} ${d.item_pedido ? "· item pedido " + esc(d.item_pedido) : ""} ${esc(d.produto || "")} ${d.esperado ? "· esperado " + num(d.esperado) : ""} ${d.informado ? "· informado " + num(d.informado) : ""}</div></div>`).join("") : `<p>✔ XML confere com o pedido de compra. Recebimento registrado.</p>`}
    <div class="acoes"><button class="btn" onclick="fecharModal();abrirDoc('${chave}')">Fechar</button></div>`);
  } catch (e) { toast(esc(e.message), "var(--er)"); }
}
async function verCorrecoes(chave) {
  const r = await api(`/documentos/${chave}/correcoes`);
  modal(`<h3>Correções sugeridas</h3><p class="muted">${esc(r.observacao)}</p><div class="tbl"><table><thead><tr><th>Campo</th><th>Atual</th><th>Sugerido</th><th>Ação</th></tr></thead><tbody>
  ${r.correcoes.map(c => `<tr><td>${c.item ? "Item " + c.item + " · " : ""}${esc(c.campo)}<div class="muted">${esc(c.motivo)}</div></td><td>${esc(c.valor_atual)}</td><td><b>${esc(c.valor_sugerido)}</b></td><td>${esc(c.acao)}</td></tr>`).join("")}</tbody></table></div>
  <div class="acoes"><button class="btn" onclick="fecharModal()">Fechar</button></div>`);
}

/* ---------- Central de rejeições ---------- */
VIEWS.rejeicoes = async el => {
  const [grupos, cat] = await Promise.all([api("/rejeicoes/central?" + empQS()), api("/rejeicoes")]);
  el.innerHTML = `<h2>Central de Erros e Rejeições</h2>
  ${grupos.length ? grupos.map(g => `<div class="card" style="margin-bottom:12px"><h3><span class="st REJEITADA">${g.rejeicao.codigo || "Pré-validação"}</span> ${esc(g.rejeicao.descricao_oficial)} — ${g.documentos.length} documento(s)</h3>
    <p>${esc(g.rejeicao.explicacao)}</p><p><b>Correção sugerida:</b> ${esc(g.rejeicao.sugestao)} ${g.rejeicao.acao_automatica ? `<span class="st PENDENTE">ação automática: ${esc(g.rejeicao.acao_automatica)}</span>` : ""}</p>
    ${gridDocs(g.documentos)}</div>`).join("") : `<div class="card muted">✔ Nenhum documento rejeitado, denegado ou bloqueado.</div>`}
  <h3 style="margin-top:20px">Catálogo de códigos SEFAZ</h3><input id="filtro-rej" placeholder="Filtrar por código ou texto…" style="width:100%;margin-bottom:8px">
  <div class="tbl"><table><thead><tr><th>Código</th><th>Descrição oficial</th><th>Em linguagem clara</th><th>Como corrigir</th></tr></thead><tbody id="cat-rej"></tbody></table></div>`;
  const render = f => $("#cat-rej").innerHTML = cat.filter(c => !f || (c.codigo + c.descricao_oficial + c.explicacao).toLowerCase().includes(f.toLowerCase()))
    .map(c => `<tr><td><b>${c.codigo}</b></td><td>${esc(c.descricao_oficial)}</td><td>${esc(c.explicacao)}</td><td>${esc(c.sugestao)}</td></tr>`).join("");
  render(""); $("#filtro-rej").oninput = e => render(e.target.value);
};

/* ---------- Alertas ---------- */
VIEWS.alertas = async el => {
  const a = await api("/alertas");
  el.innerHTML = `<h2>Alertas e notificações</h2>${a.length ? `<div class="tbl"><table><thead><tr><th>Severidade</th><th>Tipo</th><th>Alerta</th><th>Vence em</th><th>Criado</th><th></th></tr></thead><tbody>
  ${a.map(x => `<tr style="${x.lido ? "opacity:.55" : ""}" onclick="${x.chave ? `abrirDoc('${esc(x.chave)}')` : ""}"><td>${svTag(x.severidade)}</td><td>${esc(x.tipo.replace(/_/g, " "))}</td><td><b>${esc(x.titulo)}</b><div class="muted">${esc(x.mensagem)}</div></td>
  <td>${dt(x.vence_em)}</td><td>${dt(x.created_at)}</td><td>${x.lido ? "" : `<button class="btn sm" onclick="event.stopPropagation();lerAlerta(${x.id})">Marcar lido</button>`}</td></tr>`).join("")}</tbody></table></div>` : `<div class="card muted">Nenhum alerta.</div>`}`;
};
async function lerAlerta(id) { await api(`/alertas/${id}/lido`, { method: "POST" }); atualizarBadge(); mostrar("alertas", true); }

/* ---------- Entradas e compras ---------- */
VIEWS.entradas = async el => {
  const qs = "direcao=E&modelo=NFE&por_pagina=100&" + empQS();
  const [d, conc, nsu] = await Promise.all([api("/documentos?" + qs), api("/conciliacoes"), api("/captura/status")]);
  el.innerHTML = `<h2>Entradas, manifestação e conciliação</h2>
  <div class="grid g2"><div class="card"><h3>Captura automática (Distribuição DF-e)</h3>
  ${estado.empresas.map(e => { const n = nsu.find(x => x.empresa_id === e.id) || {}; return `<div class="kv" style="margin-bottom:8px"><b>${esc(e.razao_social)}</b><span>${e.tem_certificado ? `último NSU ${esc(n.ult_nsu || "0")} / máx ${esc(n.max_nsu || "-")} · cStat ${n.ultimo_c_stat || "-"} ${esc(n.ultimo_motivo || "")}<br>próxima consulta ${dt(n.proxima_consulta) || "agora"} <button class="btn sm" onclick="capturar(${e.id})">Consultar agora</button>` : `<span class="st CONTINGENCIA">sem certificado</span>`}</span></div>`; }).join("") || `<p class="muted">Cadastre empresas em Configurações.</p>`}
  </div><div class="card"><h3>Pedido de compra (integração ERP)</h3><p class="muted">Os pedidos chegam pelo ERP via <code>POST /api/v1/pedidos-compra</code>. O cruzamento usa xPed/nItemPed do XML, código do fornecedor ou NCM+preço.</p><button class="btn" onclick="novoPedido()">Cadastrar pedido manual</button></div></div>
  <h3 style="margin-top:16px">NF-e recebidas</h3>${gridDocs(d.itens)}
  <h3 style="margin-top:16px">Conciliações recentes</h3>${conc.length ? `<div class="tbl"><table><thead><tr><th>Chave</th><th>Pedido</th><th>Status</th><th>Divergências</th><th>Data</th></tr></thead><tbody>${conc.slice(0, 50).map(c => `<tr onclick="abrirDoc('${c.chave}')"><td>${esc(c.chave)}</td><td>${esc(c.pedido_numero)}</td><td>${c.status === "CONCILIADO" ? stTag("AUTORIZADA").replace("Autorizada", "Conciliado") : `<span class="st REJEITADA">${esc(c.status)}</span>`}</td><td>${(c.divergencias || []).length}</td><td>${dt(c.created_at)}</td></tr>`).join("")}</tbody></table></div>` : `<p class="muted">Nenhuma conciliação.</p>`}`;
};
async function capturar(id) {
  try { const r = await api(`/captura/${id}/executar?forcar=1`, { method: "POST" }); toast(`DF-e: ${r.documentos} documento(s) · cStat ${r.c_stat} ${esc(r.motivo)}`); mostrar("entradas", true); }
  catch (e) { toast(esc(e.message), "var(--er)"); }
}
function novoPedido() {
  modal(`<h3>Pedido de compra</h3><form id="fp"><div class="form"><label>Empresa<select name="empresa_id">${estado.empresas.map(e => `<option value="${e.id}">${esc(e.razao_social)}</option>`).join("")}</select></label>
  <label>Número<input name="numero" required></label><label>CNPJ fornecedor<input name="fornecedor_cnpj" required></label></div>
  <label>Itens (um por linha: item;código fornecedor;descrição;NCM;quantidade;valor unitário;finalidade REVENDA|USO_CONSUMO|ATIVO|INDUSTRIALIZACAO)<textarea name="itens" rows="6" placeholder="1;CX-100;CAIXA PAPELAO;48191000;120;3,50;REVENDA"></textarea></label>
  <div class="acoes"><button class="btn primary">Salvar</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`);
  $("#fp").onsubmit = async ev => { ev.preventDefault(); const f = new FormData(ev.target);
    const itens = f.get("itens").split("\n").filter(l => l.trim()).map(l => { const p = l.split(";"); return { item: p[0], cod_produto_fornecedor: p[1], cod_produto: p[1], descricao: p[2], ncm: p[3], quantidade: +String(p[4]).replace(",", "."), valor_unitario: +String(p[5]).replace(",", "."), finalidade: (p[6] || "REVENDA").trim() }; });
    try { await api("/pedidos-compra", { method: "POST", json: { empresa_id: +f.get("empresa_id"), numero: f.get("numero"), fornecedor_cnpj: f.get("fornecedor_cnpj"), itens } }); toast("Pedido salvo"); fecharModal(); }
    catch (e) { toast(esc(e.message), "var(--er)"); } };
}

/* ---------- Serviços e transporte ---------- */
VIEWS.servicos = async el => {
  const [n, c, m] = await Promise.all(["NFSE", "CTE", "MDFE"].map(x => api(`/documentos?modelo=${x}&por_pagina=50&` + empQS())));
  el.innerHTML = `<h2>Serviços (NFS-e) e Transporte (CT-e / MDF-e)</h2>
  <div class="grid g4"><div class="card kpi"><div class="l">NFS-e</div><div class="n">${n.total}</div></div><div class="card kpi"><div class="l">CT-e</div><div class="n">${c.total}</div></div><div class="card kpi"><div class="l">MDF-e</div><div class="n">${m.total}</div></div></div>
  <h3 style="margin-top:16px">NFS-e (ABRASF e Padrão Nacional) — ISS, local de incidência e retenções</h3>${gridDocs(n.itens)}
  <h3 style="margin-top:16px">CT-e</h3>${gridDocs(c.itens)}<h3 style="margin-top:16px">MDF-e</h3>${gridDocs(m.itens)}`;
};

/* ---------- Apuração e simuladores ---------- */
VIEWS.apuracao = async el => {
  const r = await api("/dashboard/resumo?" + empQS());
  const t = r.tributos || {};
  el.innerHTML = `<h2>Apuração e simuladores</h2>
  <div class="grid g4">${[["ICMS", t.icms], ["ICMS-ST", t.icms_st], ["DIFAL", t.difal], ["IPI", t.ipi], ["PIS", t.pis], ["COFINS", t.cofins], ["ISS", t.iss], ["Retenções", t.retencoes]].map(([l, v]) => `<div class="card kpi"><div class="l">${l}</div><div class="n" style="font-size:20px">${brl(v)}</div></div>`).join("")}</div>
  <div class="grid g2" style="margin-top:14px">
  <div class="card"><h3>Simulador DIFAL (EC 87/2015 · LC 190/2022)</h3><form id="sd"><div class="form"><label>Base (valor da operação)<input name="base" value="1000"></label><label>UF origem<input name="uf_origem" value="SP"></label><label>UF destino<input name="uf_destino" value="BA"></label><label>Origem mercadoria<select name="origem_mercadoria"><option>0</option><option>1</option><option>2</option><option>3</option><option>8</option></select></label>
  <label>Método<select name="metodo"><option value="">Conforme UF</option><option>BASE_DUPLA</option><option>BASE_UNICA</option></select></label></div><button class="btn primary">Calcular</button></form><pre id="sd-r"></pre></div>
  <div class="card"><h3>Simulador ICMS-ST (MVA ajustada)</h3><form id="ss"><div class="form"><label>Valor produtos<input name="valor_produtos" value="500"></label><label>IPI<input name="IPI" value="0"></label><label>MVA original %<input name="mva_original" value="40"></label>
  <label>UF origem<input name="uf_origem" value="SP"></label><label>UF destino<input name="uf_destino" value="PR"></label><label>NCM (busca MVA cadastrada)<input name="ncm"></label><label>Ajustar MVA<select name="ajustar_mva"><option value="1">Sim</option><option value="">Não</option></select></label></div><button class="btn primary">Calcular</button></form><pre id="ss-r"></pre></div>
  <div class="card"><h3>Simulador de retenções (NFS-e)</h3><form id="sr"><div class="form"><label>Valor do serviço<input name="base" value="8000"></label><label>Subitem LC 116<input name="servico" value="7.10"></label><label>Prestador do Simples<select name="prestador_simples"><option value="">Não</option><option value="1">Sim</option></select></label></div><button class="btn primary">Calcular</button></form><pre id="sr-r"></pre></div>
  </div>`;
  const sim = (id, path, conv) => $("#" + id).onsubmit = async ev => { ev.preventDefault(); const o = {}; new FormData(ev.target).forEach((v, k) => o[k] = conv(k, v));
    try { $("#" + id + "-r").textContent = JSON.stringify(await api(path, { method: "POST", json: o }), null, 2); } catch (e) { $("#" + id + "-r").textContent = e.message; } };
  const nums = (k, v) => /^(base|valor_produtos|IPI|mva_original)$/.test(k) ? +String(v).replace(",", ".") : (k === "ajustar_mva" || k === "prestador_simples") ? !!v : v;
  sim("sd", "/simulacao/difal", nums); sim("ss", "/simulacao/icms-st", nums); sim("sr", "/simulacao/retencoes", nums);
};

/* ---------- Relatórios ---------- */
VIEWS.relatorios = async el => {
  const cat = await api("/relatorios");
  const hoje = new Date(), per = `${hoje.getFullYear()}-${String(hoje.getMonth() + 1).padStart(2, "0")}`;
  el.innerHTML = `<h2>Relatórios, SPED e exportação</h2>
  <div class="filtros"><label>Período<input type="month" id="r-per" value="${per}"></label><label>Empresa<select id="r-emp">${estado.empresas.map(e => `<option value="${e.id}" ${String(e.id) === estado.empresa ? "selected" : ""}>${esc(e.razao_social)}</option>`).join("")}</select></label></div>
  <div class="tbl"><table><thead><tr><th>Relatório</th><th>Uso</th><th>Formato</th><th></th></tr></thead><tbody>
  ${cat.map(c => `<tr><td><b>${esc(c.nome)}</b></td><td class="muted">${esc(c.uso)}</td><td>${esc(c.formato.toUpperCase())}</td><td><button class="btn sm" onclick="baixarRel('${c.id}')">Gerar</button></td></tr>`).join("")}</tbody></table></div>
  <p class="muted">Os arquivos SPED são de <b>pré-escrituração/conciliação</b>: importe no sistema de escrituração ou valide no PVA da RFB antes da transmissão.</p>`;
};
function baixarRel(id) {
  const per = $("#r-per").value, emp = $("#r-emp").value;
  window.location = link(`/relatorios/${id}?periodo=${per}&empresa_id=${emp}`);
}

/* ---------- Configurações ---------- */
VIEWS.config = async el => {
  const aba = estado.abaCfg || "empresas";
  el.innerHTML = `<h2>Configurações</h2><div class="tabs">${[["empresas", "Empresas e certificado"], ["uf", "Alíquotas por UF"], ["mva", "MVA / ICMS-ST"], ["usuarios", "Usuários"], ["integracao", "Integração ERP / escrituração"], ["sistema", "Sistema"]]
    .map(([k, l]) => `<a class="${aba === k ? "on" : ""}" onclick="estado.abaCfg='${k}';mostrar('config')">${l}</a>`).join("")}</div><div id="cfg"></div>`;
  await CFG[aba]($("#cfg"));
};
const CFG = {};
CFG.empresas = async el => {
  await carregarEmpresas();
  el.innerHTML = `${estado.empresas.map(e => `<div class="card" style="margin-bottom:12px"><h3>${esc(e.razao_social)} <span class="muted">${docFmt(e.cnpj)} · ${esc(e.uf)} · IE ${esc(e.ie)} · ${esc(e.regime)} · ${e.ambiente === 1 ? "Produção" : "Homologação"}</span></h3>
    <div id="cert-${e.id}" class="muted">Carregando certificado…</div>
    <form class="filtros" onsubmit="enviarCert(event,${e.id})" style="margin-top:8px"><label>Certificado A1 (.pfx/.p12)<input type="file" name="arquivo" accept=".pfx,.p12" required></label><label>Senha<input type="password" name="senha" required></label><button class="btn primary">${e.tem_certificado ? "Substituir" : "Incluir"} certificado</button></form>
    <div class="acoes"><button class="btn sm" onclick='editarEmpresa(${JSON.stringify(e)})'>Editar empresa</button>${e.tem_certificado ? `<button class="btn sm" onclick="statusSefaz(${e.id})">Status SEFAZ</button><button class="btn sm danger" onclick="removerCert(${e.id})">Remover certificado</button>` : ""}</div></div>`).join("")}
  <button class="btn primary" onclick="editarEmpresa({})">+ Cadastrar empresa</button>`;
  for (const e of estado.empresas) {
    api(`/empresas/${e.id}/certificado`).then(c => $("#cert-" + e.id).innerHTML = `🔐 <b>${esc(c.titular)}</b> · emissor ${esc(c.emissor)} · válido até <b>${new Date(c.valido_ate).toLocaleDateString("pt-BR")}</b> (${c.dias_restantes} dias)`)
      .catch(() => $("#cert-" + e.id).innerHTML = `<span class="st CONTINGENCIA">Sem certificado digital</span> Inclua o certificado A1 para consultar a SEFAZ, capturar documentos destinados e manifestar.`);
  }
};
async function enviarCert(ev, id) {
  ev.preventDefault(); const fd = new FormData(ev.target);
  try { const c = await api(`/empresas/${id}/certificado`, { method: "POST", body: fd }); toast(`Certificado incluído: ${esc(c.titular)}`); mostrar("config"); }
  catch (e) { toast(esc(e.message), "var(--er)"); }
}
async function removerCert(id) { if (confirm("Remover o certificado desta empresa?")) { await api(`/empresas/${id}/certificado`, { method: "DELETE" }); mostrar("config"); } }
async function statusSefaz(id) {
  try { const r = await api(`/sefaz/status?empresa_id=${id}`); modal(`<h3>Status dos serviços SEFAZ</h3><pre>${esc(JSON.stringify(r, null, 2))}</pre><button class="btn" onclick="fecharModal()">Fechar</button>`); }
  catch (e) { toast(esc(e.message), "var(--er)"); }
}
function editarEmpresa(e) {
  modal(`<h3>${e.id ? "Editar" : "Cadastrar"} empresa</h3><form id="fe"><div class="form">
  <label>CNPJ<input name="cnpj" value="${esc(e.cnpj || "")}" ${e.id ? "readonly" : ""} required></label><label>Razão social<input name="razao_social" value="${esc(e.razao_social || "")}" required></label>
  <label>UF<input name="uf" maxlength="2" value="${esc(e.uf || "")}" required></label><label>Inscrição Estadual<input name="ie" value="${esc(e.ie || "")}"></label><label>Cód. município IBGE<input name="cod_mun" value="${esc(e.cod_mun || "")}"></label>
  <label>Regime<select name="regime">${[["LR", "Lucro Real"], ["LP", "Lucro Presumido"], ["SN", "Simples Nacional"], ["MEI", "MEI"]].map(([k, l]) => `<option value="${k}" ${e.regime === k ? "selected" : ""}>${l}</option>`).join("")}</select></label>
  <label>Ambiente SEFAZ<select name="ambiente"><option value="2" ${e.ambiente !== 1 ? "selected" : ""}>Homologação</option><option value="1" ${e.ambiente === 1 ? "selected" : ""}>Produção</option></select></label>
  <label>Captura automática DF-e<select name="captura_automatica"><option value="1">Sim</option><option value="" ${e.id && !e.captura_automatica ? "selected" : ""}>Não</option></select></label>
  <label>Ciência automática<select name="ciencia_automatica"><option value="">Não</option><option value="1" ${e.ciencia_automatica ? "selected" : ""}>Sim</option></select></label></div>
  <div class="acoes"><button class="btn primary">Salvar</button><button type="button" class="btn" onclick="fecharModal()">Cancelar</button></div></form>`);
  $("#fe").onsubmit = async ev => { ev.preventDefault(); const f = Object.fromEntries(new FormData(ev.target));
    f.ambiente = +f.ambiente; f.captura_automatica = !!f.captura_automatica; f.ciencia_automatica = !!f.ciencia_automatica; f.ativo = true; f.uf = f.uf.toUpperCase();
    try { await api(e.id ? `/empresas/${e.id}` : "/empresas", { method: e.id ? "PUT" : "POST", json: f }); fecharModal(); toast("Empresa salva. Inclua o certificado digital."); mostrar("config"); }
    catch (er) { toast(esc(er.message), "var(--er)"); } };
}
CFG.uf = async el => {
  const ps = await api("/parametros/uf");
  el.innerHTML = `<p class="muted">Alíquotas internas modais e FCP de referência — <b>revise com a legislação vigente de cada UF</b>. Usadas no cálculo de DIFAL e ST.</p>
  <div class="tbl"><table><thead><tr><th>UF</th><th>Alíq. interna %</th><th>FCP %</th><th>Método DIFAL</th></tr></thead><tbody>${ps.map(p => `<tr><td><b>${p.uf}</b></td><td><input data-uf="${p.uf}" data-k="aliq_interna" value="${p.aliq_interna}" size="6"></td><td><input data-uf="${p.uf}" data-k="fcp" value="${p.fcp}" size="6"></td>
  <td><select data-uf="${p.uf}" data-k="difal_metodo"><option ${p.difal_metodo === "BASE_DUPLA" ? "selected" : ""}>BASE_DUPLA</option><option ${p.difal_metodo === "BASE_UNICA" ? "selected" : ""}>BASE_UNICA</option></select></td></tr>`).join("")}</tbody></table></div>
  <div class="acoes"><button class="btn primary" id="salvar-uf">Salvar alterações</button></div>`;
  $("#salvar-uf").onclick = async () => { const m = {}; el.querySelectorAll("[data-uf]").forEach(i => { m[i.dataset.uf] = m[i.dataset.uf] || { uf: i.dataset.uf }; m[i.dataset.uf][i.dataset.k] = i.dataset.k === "difal_metodo" ? i.value : +String(i.value).replace(",", "."); });
    await api("/parametros/uf", { method: "PUT", json: Object.values(m) }); toast("Parâmetros salvos. Use 'Revalidar' nos documentos para reprocessar."); };
};
CFG.mva = async el => {
  const ms = await api("/parametros/mva");
  el.innerHTML = `<p class="muted">MVA-ST original por NCM (prefixo) e UF de destino, com vigência. Documentos com MVA diferente geram alerta de <b>MVA defasada</b>. A MVA ajustada interestadual é calculada automaticamente.</p>
  <form id="fmva" class="filtros"><label>NCM/prefixo<input name="ncm" required></label><label>CEST<input name="cest"></label><label>UF destino<input name="uf_destino" maxlength="2" required></label><label>MVA %<input name="mva_original" required></label><label>Vigência início<input type="date" name="vigencia_ini"></label><label>Fim<input type="date" name="vigencia_fim"></label><label>Fundamento<input name="fundamento" placeholder="Protocolo ICMS…"></label><button class="btn primary">Adicionar</button></form>
  <div class="tbl"><table><thead><tr><th>NCM</th><th>CEST</th><th>UF</th><th class="n">MVA %</th><th>Vigência</th><th>Fundamento</th><th></th></tr></thead><tbody>${ms.map(m => `<tr><td>${esc(m.ncm)}</td><td>${esc(m.cest)}</td><td>${esc(m.uf_destino)}</td><td class="n">${num(m.mva_original)}</td><td>${esc(m.vigencia_ini)} → ${esc(m.vigencia_fim || "…")}</td><td>${esc(m.fundamento)}</td><td><button class="btn sm danger" onclick="remMVA(${m.id})">Remover</button></td></tr>`).join("") || `<tr><td colspan="7" class="muted">Nenhuma MVA cadastrada.</td></tr>`}</tbody></table></div>
  <h3 style="margin-top:16px">Tabela NCM (TIPI)</h3><p class="muted">Importe CSV <code>ncm;descricao;aliq_ipi</code> para validar existência de NCM (rejeição 778).</p><input type="file" id="tipi" accept=".csv,.txt">`;
  $("#fmva").onsubmit = async ev => { ev.preventDefault(); const f = Object.fromEntries(new FormData(ev.target)); f.mva_original = +String(f.mva_original).replace(",", "."); f.uf_destino = f.uf_destino.toUpperCase();
    try { await api("/parametros/mva", { method: "POST", json: f }); mostrar("config"); } catch (e) { toast(esc(e.message), "var(--er)"); } };
  $("#tipi").onchange = async ev => { const r = await api("/parametros/ncm", { method: "POST", body: ev.target.files[0] }); toast(`${r.ncms} NCMs importados`); };
};
async function remMVA(id) { await api(`/parametros/mva/${id}`, { method: "DELETE" }); mostrar("config"); }
CFG.usuarios = async el => {
  const us = await api("/usuarios");
  el.innerHTML = `<div class="tbl"><table><thead><tr><th>Login</th><th>Nome</th><th>Perfil</th></tr></thead><tbody>${us.map(u => `<tr><td>${esc(u.login)}</td><td>${esc(u.nome)}</td><td>${esc(u.perfil)}</td></tr>`).join("")}</tbody></table></div>
  <form id="fu" class="filtros" style="margin-top:12px"><label>Login<input name="login" required></label><label>Nome<input name="nome"></label><label>Senha<input type="password" name="senha" minlength="8" required></label><label>Perfil<select name="perfil"><option>FISCAL</option><option>CONSULTA</option><option>ADMIN</option></select></label><button class="btn primary">Criar usuário</button></form>`;
  $("#fu").onsubmit = async ev => { ev.preventDefault(); try { await api("/usuarios", { method: "POST", json: Object.fromEntries(new FormData(ev.target)) }); mostrar("config"); } catch (e) { toast(esc(e.message), "var(--er)"); } };
};
CFG.integracao = async el => {
  const [c, wh] = await Promise.all([api("/config"), api("/integracoes/webhooks")]);
  el.innerHTML = `<div class="card"><h3>API para ERP e sistema de escrituração</h3><div class="kv"><b>Endpoint</b><span><code>${location.origin}/api/v1</code></span><b>Header</b><span><code>X-API-Key: ${esc(c.config.api_key)}</code></span>
  <b>Pré-validação</b><span><code>POST /validacao/pre-emissao</code> (XML no corpo) → 200 aprovado / 422 bloqueado</span><b>Envio de XML</b><span><code>POST /documentos/importar</code> ou grave em <code>${esc(c.config.pasta_monitorada)}</code></span>
  <b>Escrituração</b><span><code>GET /escrituracao/lancamentos?empresa_id=1&periodo=AAAA-MM</code> · SPED em <code>/relatorios/efd-icms-ipi</code></span></div></div>
  <div class="card" style="margin-top:12px"><h3>Webhooks</h3><p class="muted">Eventos: documento.novo, documento.status, alerta.novo, captura.ciclo (ou *). Assinatura HMAC-SHA256 no header X-Vorticis-Assinatura.</p>
  ${wh.map(w => `<div class="kv"><b>${esc(w.eventos)}</b><span>${esc(w.url)} <button class="btn sm danger" onclick="remWh(${w.id})">Remover</button></span></div>`).join("")}
  <form id="fw" class="filtros" style="margin-top:8px"><label>URL<input name="url" required size="40"></label><label>Eventos<input name="eventos" value="*"></label><label>Segredo<input name="segredo"></label><button class="btn primary">Adicionar</button></form></div>`;
  $("#fw").onsubmit = async ev => { ev.preventDefault(); await api("/integracoes/webhooks", { method: "POST", json: Object.fromEntries(new FormData(ev.target)) }); mostrar("config"); };
};
async function remWh(id) { await api(`/integracoes/webhooks/${id}`, { method: "DELETE" }); mostrar("config"); }
CFG.sistema = async el => {
  const c = await api("/config"), cf = c.config;
  const campos = [["host", "Endereço de escuta (127.0.0.1 local · 0.0.0.0 rede/web)"], ["porta", "Porta"], ["intervalo_distdfe_min", "Intervalo DF-e (min, mín. 60)"], ["prazo_cancelamento_nfe_horas", "Prazo cancelamento NF-e (h)"], ["prazo_cancelamento_nfce_min", "Prazo cancelamento NFC-e (min)"],
    ["prazo_manifestacao_dias", "Prazo manifestação conclusiva (dias)"], ["prazo_ciencia_dias", "Alerta XML não baixado (dias)"], ["antecedencia_alerta_horas", "Antecedência dos alertas (h)"], ["dias_alerta_certificado", "Alertar vencimento do certificado (dias)"],
    ["tolerancia_preco_pct", "Tolerância de preço na conciliação (%)"], ["tolerancia_qtd_pct", "Tolerância de quantidade (%)"], ["pasta_monitorada", "Pasta monitorada (XML do ERP)"], ["cadeia_icp_brasil", "Arquivo PEM da cadeia ICP-Brasil"]];
  el.innerHTML = `<form id="fs" class="card"><p class="muted">Diretório de dados: <code>${esc(c.diretorio_dados)}</code></p><div class="form">${campos.map(([k, l]) => `<label>${l}<input name="${k}" value="${esc(cf[k] ?? "")}"></label>`).join("")}
  <label>Verificar TLS da SEFAZ<select name="verificar_tls_sefaz"><option value="1">Sim (recomendado)</option><option value="" ${cf.verificar_tls_sefaz ? "" : "selected"}>Não</option></select></label>
  <label>Consultar pendentes automaticamente<select name="consultar_pendentes_sefaz"><option value="1">Sim</option><option value="" ${cf.consultar_pendentes_sefaz ? "" : "selected"}>Não</option></select></label></div>
  <div class="acoes"><button class="btn primary">Salvar</button></div><p class="muted">Alterações de endereço/porta exigem reiniciar o serviço "Vorticis Monitor Fiscal".</p></form>`;
  $("#fs").onsubmit = async ev => { ev.preventDefault(); const f = Object.fromEntries(new FormData(ev.target)); const o = {};
    for (const [k, v] of Object.entries(f)) o[k] = ["verificar_tls_sefaz", "consultar_pendentes_sefaz"].includes(k) ? !!v : (/^(host|pasta_monitorada|cadeia_icp_brasil)$/.test(k) ? v : +String(v).replace(",", "."));
    await api("/config", { method: "PUT", json: o }); toast("Configuração salva"); };
};

iniciar();
