"""Interface gráfica (Tkinter/ttk) do Monitor de Notas."""
from __future__ import annotations

import logging
import os
import queue
import shutil
import subprocess
import sys
import threading
import tkinter as tk
from datetime import datetime, timezone
from pathlib import Path
from tkinter import filedialog, messagebox, ttk

from . import config as cfgmod
from . import segredo, servico
from .caminhos import pasta_base
from .certificado import Certificado

log = logging.getLogger("monitor_notas")

TITULO = "Monitor de Notas"
AVISO_SEFAZ = "A SEFAZ só permite nova consulta após 1h quando não há documentos novos."
ORIGENS = {"Todas": None, "NF-e": "nfe", "NFS-e": "nfse"}
DIRECOES = {"Todas": None, "Recebidas": "recebida", "Emitidas": "emitida"}
TIPOS = {"Todos": None, "Completo": "completo", "Resumo": "resumo", "Evento": "evento"}
ROTULO_ORIGEM = {"nfe": "NF-e", "nfse": "NFS-e"}
ROTULO_DIRECAO = {"recebida": "Recebida", "emitida": "Emitida"}
ROTULO_TIPO = {"completo": "Completo", "resumo": "Resumo", "evento": "Evento"}


# ---------- utilitários ----------

def fmt_valor(v) -> str:
    """1234.5 -> '1.234,50'"""
    if v is None:
        return ""
    return f"{float(v):,.2f}".replace(",", "X").replace(".", ",").replace("X", ".")


def fmt_data(iso: str | None) -> str:
    """AAAA-MM-DD... -> DD/MM/AAAA"""
    d = (iso or "")[:10]
    try:
        return datetime.strptime(d, "%Y-%m-%d").strftime("%d/%m/%Y")
    except ValueError:
        return d


def ler_data(texto: str, rotulo: str) -> str | None:
    """Aceita AAAA-MM-DD ou DD/MM/AAAA; devolve AAAA-MM-DD (ou None se vazio)."""
    texto = texto.strip()
    if not texto:
        return None
    for f in ("%Y-%m-%d", "%d/%m/%Y"):
        try:
            return datetime.strptime(texto, f).strftime("%Y-%m-%d")
        except ValueError:
            pass
    raise ValueError(f"{rotulo} inválida: use AAAA-MM-DD ou DD/MM/AAAA.")


def fmt_cnpj(c: str | None) -> str:
    c = c or ""
    if len(c) == 14 and c.isdigit():
        return f"{c[:2]}.{c[2:5]}.{c[5:8]}/{c[8:12]}-{c[12:]}"
    return c


def abrir_caminho(caminho: Path) -> None:
    """Abre arquivo/pasta no programa padrão do sistema."""
    if sys.platform == "win32":
        os.startfile(str(caminho))  # type: ignore[attr-defined]
    elif sys.platform == "darwin":
        subprocess.Popen(["open", str(caminho)])
    else:
        subprocess.Popen(["xdg-open", str(caminho)])


def caminho_para_config(caminho: str) -> str:
    """Relativo à pasta do exe quando estiver dentro dela (mantém a pasta portátil)."""
    p = Path(caminho).resolve()
    try:
        return p.relative_to(pasta_base().resolve()).as_posix()
    except ValueError:
        return str(p)


def erro(parent, msg: str, exc: Exception | None = None) -> None:
    if exc is not None:
        log.exception(msg)
        msg = f"{msg}\n\n{exc}"
    messagebox.showerror(TITULO, msg, parent=parent)


# ---------- diálogo de empresa ----------

class DialogoEmpresa(tk.Toplevel):
    def __init__(self, parent, empresa: cfgmod.Empresa | None):
        super().__init__(parent)
        self.empresa = empresa
        self.resultado: cfgmod.Empresa | None = None
        self.title("Editar empresa" if empresa else "Adicionar empresa")
        self.resizable(False, False)
        self.transient(parent)

        self.v_nome = tk.StringVar(value=empresa.nome if empresa else "")
        self.v_cnpj = tk.StringVar(value=empresa.cnpj if empresa else "")
        self.v_uf = tk.StringVar(value=empresa.uf.upper() if empresa else "")
        self.v_cert = tk.StringVar(value=empresa.certificado if empresa else "")
        self.v_senha = tk.StringVar()
        self.v_nfe = tk.BooleanVar(value=empresa.consultar_nfe if empresa else True)
        self.v_nfse = tk.BooleanVar(value=empresa.consultar_nfse if empresa else True)

        f = ttk.Frame(self, padding=12)
        f.pack(fill="both", expand=True)
        f.columnconfigure(1, weight=1)
        ttk.Label(f, text="Nome:").grid(row=0, column=0, sticky="w", pady=3)
        ttk.Entry(f, textvariable=self.v_nome, width=45).grid(row=0, column=1, columnspan=2, sticky="ew")
        ttk.Label(f, text="CNPJ:").grid(row=1, column=0, sticky="w", pady=3)
        ttk.Entry(f, textvariable=self.v_cnpj, width=24).grid(row=1, column=1, sticky="w")
        ttk.Label(f, text="UF:").grid(row=2, column=0, sticky="w", pady=3)
        ttk.Combobox(f, textvariable=self.v_uf, values=list(cfgmod.CODIGOS_UF), width=6,
                     state="readonly").grid(row=2, column=1, sticky="w")
        ttk.Label(f, text="Certificado (.pfx/.p12):").grid(row=3, column=0, sticky="w", pady=3)
        ttk.Entry(f, textvariable=self.v_cert, width=45).grid(row=3, column=1, sticky="ew")
        ttk.Button(f, text="Procurar...", command=self._procurar).grid(row=3, column=2, padx=(6, 0))
        ttk.Label(f, text="Senha do certificado:").grid(row=4, column=0, sticky="w", pady=3)
        ttk.Entry(f, textvariable=self.v_senha, show="*", width=30).grid(row=4, column=1, sticky="w")
        if empresa:
            ttk.Label(f, text="(deixe em branco para manter a senha atual)",
                      foreground="gray").grid(row=5, column=1, sticky="w")
        ttk.Checkbutton(f, text="Consultar NF-e", variable=self.v_nfe).grid(row=6, column=1, sticky="w")
        ttk.Checkbutton(f, text="Consultar NFS-e", variable=self.v_nfse).grid(row=7, column=1, sticky="w")
        bt = ttk.Frame(f)
        bt.grid(row=8, column=0, columnspan=3, sticky="e", pady=(12, 0))
        ttk.Button(bt, text="Salvar", command=self._salvar).pack(side="left", padx=4)
        ttk.Button(bt, text="Cancelar", command=self.destroy).pack(side="left")

        self.bind("<Escape>", lambda _e: self.destroy())
        self.grab_set()
        self.wait_visibility()
        self.focus_set()

    def _procurar(self) -> None:
        arq = filedialog.askopenfilename(
            parent=self, title="Selecionar certificado",
            filetypes=[("Certificado digital", "*.pfx *.p12"), ("Todos os arquivos", "*.*")])
        if not arq:
            return
        origem = Path(arq).resolve()
        pasta_cert = pasta_base() / "certificados"
        dentro = pasta_base().resolve() in origem.parents
        if not dentro and messagebox.askyesno(
                TITULO, f"Copiar o certificado para a pasta\n{pasta_cert}\n"
                        "ao lado do programa? Assim a pasta inteira pode ser movida de computador.",
                parent=self):
            try:
                pasta_cert.mkdir(parents=True, exist_ok=True)
                destino = pasta_cert / origem.name
                shutil.copy2(origem, destino)
                origem = destino.resolve()
            except OSError as e:
                erro(self, "Não foi possível copiar o certificado.", e)
        self.v_cert.set(caminho_para_config(str(origem)))

    def _salvar(self) -> None:
        nome = self.v_nome.get().strip()
        cnpj = cfgmod.so_digitos(self.v_cnpj.get())
        uf = self.v_uf.get().strip().upper()
        cert = self.v_cert.get().strip()
        senha = self.v_senha.get()
        if len(cnpj) != 14:
            return messagebox.showwarning(TITULO, "O CNPJ deve ter 14 dígitos.", parent=self)
        if uf not in cfgmod.CODIGOS_UF:
            return messagebox.showwarning(TITULO, "Selecione a UF.", parent=self)
        if not cert:
            return messagebox.showwarning(TITULO, "Selecione o arquivo do certificado.", parent=self)
        if not self.empresa and not senha:
            return messagebox.showwarning(TITULO, "Informe a senha do certificado.", parent=self)
        if not (self.v_nfe.get() or self.v_nfse.get()):
            return messagebox.showwarning(TITULO, "Marque NF-e e/ou NFS-e.", parent=self)
        try:
            armazenada = segredo.proteger(senha) if senha else self.empresa.senha_armazenada
        except Exception as e:
            return erro(self, "Não foi possível proteger a senha.", e)
        self.resultado = cfgmod.Empresa(
            nome=nome or cnpj, cnpj=cnpj, uf=uf, certificado=cert, senha_armazenada=armazenada,
            consultar_nfe=self.v_nfe.get(), consultar_nfse=self.v_nfse.get())
        self.destroy()


# ---------- aplicação ----------

class App:
    def __init__(self, root: tk.Tk):
        self.root = root
        self.fila: queue.Queue = queue.Queue()
        self.consultando = False
        self.importando = False
        self.silencioso = False
        self.auto_atual = False
        self.id_auto: str | None = None
        self.linhas = []  # linhas atualmente exibidas (sqlite3.Row)
        self.caminho_log = servico.configurar_log(console=False)

        root.title(TITULO)
        root.geometry("1100x650")
        estilo = ttk.Style()
        temas = estilo.theme_names()
        estilo.theme_use("vista" if "vista" in temas else "clam")

        try:
            self.cfg = cfgmod.carregar()
        except Exception as e:
            erro(root, "Não foi possível ler o config.toml. Será usada uma configuração vazia.", e)
            self.cfg = cfgmod.Config()

        self._barra_superior()
        self.abas = ttk.Notebook(root)
        self.abas.pack(fill="both", expand=True, padx=8, pady=(0, 8))
        self._aba_documentos()
        self._aba_empresas()
        self._aba_config()

        self._atualizar_empresas()
        self._atualizar_filtro_empresas()
        self.carregar_documentos()
        self._agendar_auto()
        root.after(200, self._ler_fila)

        if not self.cfg.empresas:
            self.abas.select(self.f_emp)
            messagebox.showinfo(
                TITULO, "Bem-vindo! Para começar, cadastre ao menos uma empresa com o "
                        "certificado digital (A1) na aba Empresas.", parent=root)

    # --- barra superior ---
    def _barra_superior(self) -> None:
        f = ttk.Frame(self.root, padding=8)
        f.pack(fill="x")
        self.btn_consultar = ttk.Button(f, text="Consultar agora", command=self.consultar_agora)
        self.btn_consultar.pack(side="left")
        self.v_status = tk.StringVar(value="Pronto.")
        ttk.Label(f, textvariable=self.v_status).pack(side="left", padx=12)
        ttk.Label(f, text=AVISO_SEFAZ, foreground="gray").pack(side="right")

    def status(self, msg: str) -> None:
        self.v_status.set(msg)

    # --- aba Documentos ---
    def _aba_documentos(self) -> None:
        self.f_doc = f = ttk.Frame(self.abas, padding=8)
        self.abas.add(f, text="Documentos")

        flt = ttk.Frame(f)
        flt.pack(fill="x")
        self.v_emp = tk.StringVar(value="Todas")
        self.v_ori = tk.StringVar(value="Todas")
        self.v_dir = tk.StringVar(value="Todas")
        self.v_tipo = tk.StringVar(value="Todos")
        self.v_ini = tk.StringVar()
        self.v_fim = tk.StringVar()
        self.v_txt = tk.StringVar()

        def combo(rot, var, valores, larg, col):
            ttk.Label(flt, text=rot).grid(row=0, column=col, sticky="w", padx=(0, 4))
            c = ttk.Combobox(flt, textvariable=var, values=valores, width=larg, state="readonly")
            c.grid(row=1, column=col, padx=(0, 8), sticky="w")
            return c

        self.cb_emp = combo("Empresa", self.v_emp, ["Todas"], 26, 0)
        combo("Origem", self.v_ori, list(ORIGENS), 8, 1)
        combo("Direção", self.v_dir, list(DIRECOES), 10, 2)
        combo("Tipo", self.v_tipo, list(TIPOS), 10, 3)
        for col, (rot, var) in enumerate((("Data inicial", self.v_ini), ("Data final", self.v_fim)), 4):
            ttk.Label(flt, text=rot).grid(row=0, column=col, sticky="w")
            ttk.Entry(flt, textvariable=var, width=12).grid(row=1, column=col, padx=(0, 8))
        ttk.Label(flt, text="Texto (chave, emitente, CNPJ)").grid(row=0, column=6, sticky="w")
        e = ttk.Entry(flt, textvariable=self.v_txt, width=28)
        e.grid(row=1, column=6, padx=(0, 8))
        e.bind("<Return>", lambda _e: self.carregar_documentos())
        ttk.Button(flt, text="Filtrar", command=self.carregar_documentos).grid(row=1, column=7)

        cols = [("origem", "Origem", 60), ("direcao", "Direção", 80), ("tipo", "Tipo", 80),
                ("emissao", "Emissão", 90), ("emitente", "Emitente", 240),
                ("cnpj", "CNPJ emitente", 130), ("valor", "Valor", 100), ("chave", "Chave", 340)]
        meio = ttk.Frame(f)
        meio.pack(fill="both", expand=True, pady=6)
        self.tv = ttk.Treeview(meio, columns=[c[0] for c in cols], show="headings")
        for cid, titulo, larg in cols:
            self.tv.heading(cid, text=titulo)
            self.tv.column(cid, width=larg, anchor="e" if cid == "valor" else "w")
        sb = ttk.Scrollbar(meio, orient="vertical", command=self.tv.yview)
        sbh = ttk.Scrollbar(meio, orient="horizontal", command=self.tv.xview)
        self.tv.configure(yscrollcommand=sb.set, xscrollcommand=sbh.set)
        meio.rowconfigure(0, weight=1)
        meio.columnconfigure(0, weight=1)
        self.tv.grid(row=0, column=0, sticky="nsew")
        sb.grid(row=0, column=1, sticky="ns")
        sbh.grid(row=1, column=0, sticky="ew")
        self.tv.bind("<Double-1>", self._copiar_chave)

        rod = ttk.Frame(f)
        rod.pack(fill="x")
        self.v_total = tk.StringVar()
        ttk.Label(rod, textvariable=self.v_total).pack(side="left")
        ttk.Button(rod, text="Abrir pasta de XMLs", command=self.abrir_pasta_xml).pack(side="right")
        ttk.Button(rod, text="Exportar XMLs (ZIP)...", command=self.exportar_zip).pack(side="right", padx=6)
        ttk.Button(rod, text="Exportar planilha...", command=self.exportar_planilha).pack(side="right")

    def _atualizar_filtro_empresas(self) -> None:
        self.mapa_emp = {"Todas": None}
        for e in self.cfg.empresas:
            self.mapa_emp[f"{e.nome} ({fmt_cnpj(e.cnpj)})"] = e.cnpj
        self.cb_emp["values"] = list(self.mapa_emp)
        if self.v_emp.get() not in self.mapa_emp:
            self.v_emp.set("Todas")

    def carregar_documentos(self) -> None:
        try:
            ini = ler_data(self.v_ini.get(), "Data inicial")
            fim = ler_data(self.v_fim.get(), "Data final")
        except ValueError as e:
            return messagebox.showwarning(TITULO, str(e), parent=self.root)
        db = None
        try:
            db = servico.abrir_banco(self.cfg)
            self.linhas = db.consultar(
                cnpj=self.mapa_emp.get(self.v_emp.get()), origem=ORIGENS[self.v_ori.get()],
                direcao=DIRECOES[self.v_dir.get()], tipo=TIPOS[self.v_tipo.get()],
                data_ini=ini, data_fim=fim, texto=self.v_txt.get().strip() or None)
        except Exception as e:
            return erro(self.root, "Não foi possível consultar o banco de dados.", e)
        finally:
            if db is not None:
                db.con.close()
        self.tv.delete(*self.tv.get_children())
        soma = 0.0
        for i, r in enumerate(self.linhas):
            self.tv.insert("", "end", iid=str(i), values=(
                ROTULO_ORIGEM.get(r["origem"], r["origem"]),
                ROTULO_DIRECAO.get(r["direcao"], r["direcao"]),
                ROTULO_TIPO.get(r["tipo"], r["tipo"]),
                fmt_data(r["data_emissao"]), r["emitente_nome"] or "",
                fmt_cnpj(r["emitente_cnpj"]), fmt_valor(r["valor"]), r["chave"]))
            if r["tipo"] != "evento" and r["valor"]:
                soma += r["valor"]
        self.v_total.set(f"{len(self.linhas)} documento(s)  |  Soma dos valores "
                         f"(completos e resumos): R$ {fmt_valor(soma)}")

    def _copiar_chave(self, _ev=None) -> None:
        sel = self.tv.selection()
        if not sel:
            return
        chave = self.linhas[int(sel[0])]["chave"]
        self.root.clipboard_clear()
        self.root.clipboard_append(chave)
        self.status(f"Chave copiada para a área de transferência: {chave}")

    def _exportar(self, tipo: str) -> None:
        if not self.linhas:
            return messagebox.showinfo(TITULO, "Não há documentos na lista para exportar.",
                                       parent=self.root)
        hoje = datetime.now().strftime("%Y%m%d")
        if tipo == "planilha":
            caminho = filedialog.asksaveasfilename(
                parent=self.root, title="Exportar planilha", defaultextension=".xlsx",
                initialfile=f"documentos_{hoje}.xlsx",
                filetypes=[("Excel", "*.xlsx"), ("CSV", "*.csv")])
        else:
            caminho = filedialog.asksaveasfilename(
                parent=self.root, title="Exportar XMLs", defaultextension=".zip",
                initialfile=f"xmls_{hoje}.zip", filetypes=[("ZIP", "*.zip")])
        if not caminho:
            return
        try:
            from . import exportar  # import tardio
            if tipo == "planilha":
                n = exportar.planilha(self.linhas, caminho)
                self.status(f"Planilha gerada com {n} linha(s): {caminho}")
            else:
                n = exportar.zip_xml(self.linhas, caminho)
                self.status(f"ZIP gerado com {n} arquivo(s): {caminho}")
        except Exception as e:
            erro(self.root, "Não foi possível exportar.", e)

    def exportar_planilha(self) -> None:
        self._exportar("planilha")

    def exportar_zip(self) -> None:
        self._exportar("zip")

    def abrir_pasta_xml(self) -> None:
        try:
            pasta = self.cfg.caminho_xml
            pasta.mkdir(parents=True, exist_ok=True)
            abrir_caminho(pasta)
        except Exception as e:
            erro(self.root, "Não foi possível abrir a pasta de XMLs.", e)

    # --- aba Empresas ---
    def _aba_empresas(self) -> None:
        self.f_emp = f = ttk.Frame(self.abas, padding=8)
        self.abas.add(f, text="Empresas")
        cols = [("nome", "Nome", 260), ("cnpj", "CNPJ", 140), ("uf", "UF", 40),
                ("cert", "Certificado", 320), ("nfe", "NF-e", 50), ("nfse", "NFS-e", 50)]
        self.tv_emp = ttk.Treeview(f, columns=[c[0] for c in cols], show="headings",
                                   selectmode="browse")
        for cid, titulo, larg in cols:
            self.tv_emp.heading(cid, text=titulo)
            self.tv_emp.column(cid, width=larg, anchor="w")
        self.tv_emp.pack(fill="both", expand=True)
        self.tv_emp.bind("<Double-1>", lambda _e: self.editar_empresa())
        bt = ttk.Frame(f)
        bt.pack(fill="x", pady=(8, 0))
        for txt, cmd in (("Adicionar", self.adicionar_empresa), ("Editar", self.editar_empresa),
                         ("Remover", self.remover_empresa),
                         ("Testar certificado", self.testar_certificado)):
            ttk.Button(bt, text=txt, command=cmd).pack(side="left", padx=(0, 6))

    def _atualizar_empresas(self) -> None:
        self.tv_emp.delete(*self.tv_emp.get_children())
        for i, e in enumerate(self.cfg.empresas):
            self.tv_emp.insert("", "end", iid=str(i), values=(
                e.nome, fmt_cnpj(e.cnpj), e.uf, e.certificado,
                "Sim" if e.consultar_nfe else "Não", "Sim" if e.consultar_nfse else "Não"))

    def _empresa_selecionada(self) -> int | None:
        sel = self.tv_emp.selection()
        if not sel:
            messagebox.showinfo(TITULO, "Selecione uma empresa na lista.", parent=self.root)
            return None
        return int(sel[0])

    def _gravar_config(self) -> bool:
        try:
            cfgmod.salvar(self.cfg)
        except Exception as e:
            erro(self.root, "Não foi possível salvar o config.toml.", e)
            return False
        self._atualizar_empresas()
        self._atualizar_filtro_empresas()
        return True

    def _dialogo_empresa(self, idx: int | None) -> None:
        emp = self.cfg.empresas[idx] if idx is not None else None
        dlg = DialogoEmpresa(self.root, emp)
        self.root.wait_window(dlg)
        if dlg.resultado is None:
            return
        outro = [e for i, e in enumerate(self.cfg.empresas) if i != idx]
        if any(e.cnpj == dlg.resultado.cnpj for e in outro):
            return messagebox.showwarning(TITULO, "Já existe uma empresa com esse CNPJ.",
                                          parent=self.root)
        if idx is None:
            self.cfg.empresas.append(dlg.resultado)
        else:
            self.cfg.empresas[idx] = dlg.resultado
        self._gravar_config()

    def adicionar_empresa(self) -> None:
        self._dialogo_empresa(None)

    def editar_empresa(self) -> None:
        idx = self._empresa_selecionada()
        if idx is not None:
            self._dialogo_empresa(idx)

    def remover_empresa(self) -> None:
        idx = self._empresa_selecionada()
        if idx is None:
            return
        emp = self.cfg.empresas[idx]
        if messagebox.askyesno(TITULO, f"Remover a empresa '{emp.nome}'?\n"
                                       "Os documentos já baixados serão mantidos.",
                               parent=self.root):
            del self.cfg.empresas[idx]
            self._gravar_config()

    def testar_certificado(self) -> None:
        idx = self._empresa_selecionada()
        if idx is None:
            return
        emp = self.cfg.empresas[idx]
        try:
            cert = Certificado.de_pfx(emp.caminho_certificado, emp.senha)
        except Exception as e:
            return erro(self.root, f"Falha ao abrir o certificado de '{emp.nome}'. "
                                   "Verifique o arquivo e a senha.", e)
        validade = cert.validade
        dias = (validade - datetime.now(timezone.utc)).days
        msg = f"Titular: {cert.titular}\nVálido até: {validade:%d/%m/%Y}  ({dias} dia(s))"
        if dias < 0:
            messagebox.showerror(TITULO, msg + "\n\nCERTIFICADO VENCIDO!", parent=self.root)
        elif dias < 30:
            messagebox.showwarning(TITULO, msg + "\n\nAtenção: vence em menos de 30 dias.",
                                   parent=self.root)
        else:
            messagebox.showinfo(TITULO, msg, parent=self.root)

    # --- aba Configurações ---
    def _aba_config(self) -> None:
        f = ttk.Frame(self.abas, padding=16)
        self.abas.add(f, text="Configurações")
        self.v_amb = tk.StringVar(value="Produção" if self.cfg.ambiente == "producao"
                                  else "Homologação")
        self.v_int = tk.StringVar(value=str(self.cfg.intervalo_minutos))
        self.v_auto = tk.BooleanVar(value=False)
        ttk.Label(f, text="Ambiente:").grid(row=0, column=0, sticky="w", pady=4)
        ttk.Combobox(f, textvariable=self.v_amb, values=["Produção", "Homologação"],
                     state="readonly", width=16).grid(row=0, column=1, sticky="w")
        ttk.Label(f, text="Intervalo de consulta (minutos, mín. 60):").grid(row=1, column=0,
                                                                          sticky="w", pady=4)
        ttk.Spinbox(f, from_=60, to=1440, textvariable=self.v_int, width=8).grid(
            row=1, column=1, sticky="w")
        ttk.Checkbutton(f, text="Consultar automaticamente enquanto o programa estiver aberto",
                        variable=self.v_auto, command=self._agendar_auto).grid(
            row=2, column=0, columnspan=2, sticky="w", pady=4)
        ttk.Button(f, text="Salvar configurações", command=self.salvar_config).grid(
            row=3, column=0, sticky="w", pady=(10, 20))
        ttk.Button(f, text="Abrir log", command=self.abrir_log).grid(row=4, column=0, sticky="w", pady=4)
        ttk.Button(f, text="Importar XMLs de uma pasta...", command=self.importar_xmls).grid(
            row=5, column=0, sticky="w", pady=4)

    def salvar_config(self) -> None:
        try:
            minutos = int(self.v_int.get())
        except ValueError:
            return messagebox.showwarning(TITULO, "Informe o intervalo em minutos (número inteiro).",
                                          parent=self.root)
        if minutos < 60:
            minutos = 60
            self.v_int.set("60")
            messagebox.showinfo(TITULO, "O intervalo mínimo é de 60 minutos.", parent=self.root)
        self.cfg.intervalo_minutos = minutos
        self.cfg.ambiente = "producao" if self.v_amb.get() == "Produção" else "homologacao"
        if self._gravar_config():
            self._agendar_auto()
            self.status("Configurações salvas.")

    def abrir_log(self) -> None:
        try:
            abrir_caminho(self.caminho_log)
        except Exception as e:
            erro(self.root, f"Não foi possível abrir o log:\n{self.caminho_log}", e)

    def importar_xmls(self) -> None:
        if self.importando or self.consultando:
            return messagebox.showinfo(TITULO, "Aguarde a operação em andamento.", parent=self.root)
        if not self.cfg.empresas:
            return messagebox.showinfo(TITULO, "Cadastre uma empresa antes de importar.",
                                       parent=self.root)
        pasta = filedialog.askdirectory(parent=self.root, title="Pasta com os XMLs")
        if not pasta:
            return
        self.importando = True
        self.status("Importando XMLs...")

        def tarefa():
            try:
                db = servico.abrir_banco(self.cfg)  # conexão própria da thread
                try:
                    self.fila.put(("importado", servico.importar_pasta(self.cfg, db, pasta)))
                finally:
                    db.con.close()
            except Exception as e:
                self.fila.put(("erro", "Falha ao importar os XMLs.", e))

        threading.Thread(target=tarefa, daemon=True).start()

    # --- consulta (thread) ---
    def consultar_agora(self) -> None:
        if self.consultando or self.importando:
            return
        auto, self.silencioso = self.silencioso, False
        self.auto_atual = auto
        if not self.cfg.empresas:
            self.abas.select(self.f_emp)
            return messagebox.showinfo(TITULO, "Cadastre uma empresa antes de consultar.",
                                       parent=self.root)
        self.consultando = True
        self.btn_consultar.state(["disabled"])
        self.status("Consultando...")

        def tarefa():
            try:
                db = servico.abrir_banco(self.cfg)  # conexão própria da thread
                try:
                    res = servico.executar_ciclo(self.cfg, db, lambda m: self.fila.put(("progresso", m)))
                finally:
                    db.con.close()
                self.fila.put(("fim", res))
            except Exception as e:
                self.fila.put(("erro", "Falha na consulta.", e))

        threading.Thread(target=tarefa, daemon=True).start()

    def _ler_fila(self) -> None:
        try:
            while True:
                msg = self.fila.get_nowait()
                tipo = msg[0]
                if tipo == "progresso":
                    self.status(msg[1])
                elif tipo == "fim":
                    self._fim_consulta(msg[1])
                elif tipo == "importado":
                    self.importando = False
                    self.status(f"Importação concluída: {msg[1]} documento(s) novo(s).")
                    self.carregar_documentos()
                elif tipo == "erro":
                    self.consultando = self.importando = False
                    self.btn_consultar.state(["!disabled"])
                    self.status("Erro. Veja o log para detalhes.")
                    log.error("%s", msg[1], exc_info=msg[2])
                    erro(self.root, msg[1], msg[2])
        except queue.Empty:
            pass
        self.root.after(200, self._ler_fila)

    def _fim_consulta(self, resultados) -> None:
        self.consultando = False
        self.btn_consultar.state(["!disabled"])
        linhas = []
        for r in resultados:
            partes = []
            if r.nfe is not None:
                partes.append(f"NF-e: {r.nfe} novo(s)")
            if r.nfse is not None:
                partes.append(f"NFS-e: {r.nfse} novo(s)")
            if r.erro:
                partes.append(f"ERRO: {r.erro}")
            linhas.append(f"{r.empresa} - " + ("; ".join(partes) or "nada consultado"))
        com_erro = any(r.erro for r in resultados)
        self.status("Consulta concluída" + (" com erros." if com_erro else "."))
        self.carregar_documentos()
        if self.auto_atual and not com_erro:
            return
        texto = "\n".join(linhas) + "\n\n" + AVISO_SEFAZ
        (messagebox.showwarning if com_erro else messagebox.showinfo)(
            TITULO, texto, parent=self.root)

    # --- consulta automática ---
    def _agendar_auto(self) -> None:
        if self.id_auto:
            self.root.after_cancel(self.id_auto)
            self.id_auto = None
        if self.v_auto.get():
            ms = max(60, self.cfg.intervalo_minutos) * 60 * 1000
            self.id_auto = self.root.after(ms, self._tick_auto)

    def _tick_auto(self) -> None:
        self.id_auto = None
        if self.cfg.empresas and not (self.consultando or self.importando):
            self.silencioso = True  # consulta automática não abre pop-up de sucesso
            self.consultar_agora()
        self._agendar_auto()


def iniciar() -> None:
    servico.configurar_log(console=False)
    root = tk.Tk()
    try:
        App(root)
    except Exception as e:
        log.exception("Falha ao iniciar a interface")
        messagebox.showerror(TITULO, f"Falha ao iniciar: {e}")
        raise
    root.mainloop()
