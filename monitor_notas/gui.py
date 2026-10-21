"""Interface gráfica (Tkinter/ttk) do Vorticis Monitor."""
from __future__ import annotations

import logging
import os
import queue
import shutil
import subprocess
import sys
import threading
import tkinter as tk
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from tkinter import filedialog, messagebox, ttk

from . import caminhos
from . import config as cfgmod
from . import segredo, servico
from .certificado import Certificado, DadosCertificado

log = logging.getLogger("monitor_notas")

TITULO = "Vorticis Monitor"
AZUL = "#14284B"
AZUL_CLARO = "#2D4A82"
LARANJA = "#F28C28"
LARANJA_CLARO = "#FFA44D"
CINZA_CLARO = "#E8ECF4"

TODAS_EMPRESAS = "Todas as empresas"
AVISO_SEFAZ = "A SEFAZ só permite nova consulta após 1h quando não há documentos novos."
TIPOS_CONSULTA = (("nfe", "NF-e"), ("nfse", "NFS-e"))
DIRECOES = {"Todas": None, "Recebidas": "recebida", "Emitidas": "emitida"}
SITUACOES = {"Todos": None, "Completo": "completo", "Resumo": "resumo", "Evento": "evento"}
ROTULO_DIRECAO = {"recebida": "Recebida", "emitida": "Emitida"}
ROTULO_TIPO = {"completo": "Completo", "resumo": "Resumo", "evento": "Evento"}
MAX_FALHAS = 10


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
    """Aceita DD/MM/AAAA ou AAAA-MM-DD; devolve AAAA-MM-DD (ou None se vazio)."""
    texto = texto.strip()
    if not texto:
        return None
    for f in ("%d/%m/%Y", "%Y-%m-%d"):
        try:
            return datetime.strptime(texto, f).strftime("%Y-%m-%d")
        except ValueError:
            pass
    raise ValueError(f"{rotulo} inválida: use DD/MM/AAAA.")


def fmt_cnpj(c: str | None) -> str:
    c = c or ""
    if len(c) == 14 and c.isdigit():
        return f"{c[:2]}.{c[2:5]}.{c[5:8]}/{c[8:12]}-{c[12:]}"
    return c


def fmt_validade(v) -> str:
    return v.strftime("%d/%m/%Y") if v else ""


def dias_para(validade) -> int:
    """Dias até a validade (negativo se vencido)."""
    if validade.tzinfo is None:
        validade = validade.replace(tzinfo=timezone.utc)
    return (validade - datetime.now(timezone.utc)).days


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
        return p.relative_to(caminhos.pasta_base().resolve()).as_posix()
    except ValueError:
        return str(p)


def pasta_recursos() -> Path:
    if getattr(sys, "frozen", False):
        return Path(sys._MEIPASS) / "monitor_notas" / "recursos"  # type: ignore[attr-defined]
    return Path(__file__).parent / "recursos"


def carregar_imagem(nome: str, altura: int | None = None) -> tk.PhotoImage | None:
    """Carrega uma imagem dos recursos; devolve None se faltar ou falhar."""
    try:
        img = tk.PhotoImage(file=str(pasta_recursos() / nome))
        if altura and img.height() > altura * 1.5:
            img = img.subsample(max(1, round(img.height() / altura)))
        return img
    except Exception:
        return None


def erro(parent, msg: str, exc: Exception | None = None) -> None:
    if exc is not None:
        log.error("%s", msg, exc_info=exc)
        msg = f"{msg}\n\n{exc}"
    messagebox.showerror(TITULO, msg, parent=parent)


def texto_falhas(falhas: list[str], maximo: int = MAX_FALHAS) -> str:
    texto = "\n".join(falhas[:maximo])
    if len(falhas) > maximo:
        texto += f"\n... e mais {len(falhas) - maximo} falha(s) (veja o log)."
    return texto


def nome_certificado(d: DadosCertificado) -> str:
    return d.razao_social or d.titular


# ---------- diálogos ----------

class DialogoBase(tk.Toplevel):
    """Janela modal centralizada sobre a janela principal."""

    def __init__(self, parent, titulo: str):
        super().__init__(parent)
        self.title(titulo)
        self.resizable(False, False)
        self.transient(parent)
        self.bind("<Escape>", lambda _e: self.destroy())

    def abrir(self) -> None:
        self.update_idletasks()
        p = self.master.winfo_toplevel()
        x = p.winfo_rootx() + max(0, (p.winfo_width() - self.winfo_width()) // 2)
        y = p.winfo_rooty() + max(0, (p.winfo_height() - self.winfo_height()) // 3)
        self.geometry(f"+{x}+{y}")
        try:
            self.grab_set()
        except tk.TclError:
            pass
        self.focus_set()


class DialogoUF(DialogoBase):
    """Pergunta a UF de um certificado que não a traz."""

    def __init__(self, parent, nome: str):
        super().__init__(parent, "Informe a UF")
        self.resultado: str | None = None
        self.v_uf = tk.StringVar()
        f = ttk.Frame(self, padding=12)
        f.pack()
        ttk.Label(f, text=f"O certificado de\n{nome}\nnão informa a UF. Selecione:").pack(anchor="w")
        ttk.Combobox(f, textvariable=self.v_uf, values=list(cfgmod.CODIGOS_UF), width=6,
                     state="readonly").pack(anchor="w", pady=8)
        bt = ttk.Frame(f)
        bt.pack(anchor="e")
        ttk.Button(bt, text="OK", style="Principal.TButton", command=self._ok).pack(side="left", padx=4)
        ttk.Button(bt, text="Ignorar esta empresa", command=self.destroy).pack(side="left")
        self.abrir()

    def _ok(self) -> None:
        if self.v_uf.get() in cfgmod.CODIGOS_UF:
            self.resultado = self.v_uf.get()
            self.destroy()


class DialogoEmpresa(DialogoBase):
    """Cadastro/edição de empresa. Modos: certificado do Windows (thumbprint) ou arquivo .pfx."""

    def __init__(self, parent, empresa: cfgmod.Empresa | None = None,
                 dados: DadosCertificado | None = None, cnpjs_existentes=()):
        super().__init__(parent, "Editar empresa" if empresa else "Adicionar empresa")
        self.empresa = empresa
        self.cnpjs_existentes = set(cnpjs_existentes)
        self.resultado: cfgmod.Empresa | None = None
        self.thumb = (empresa.thumbprint if empresa else dados.thumbprint if dados else "").upper()
        self.dados = dados

        self.v_nome = tk.StringVar(value=empresa.nome if empresa else
                                   nome_certificado(dados) if dados else "")
        self.v_cnpj = tk.StringVar(value=empresa.cnpj if empresa else
                                   (dados.cnpj or "") if dados else "")
        self.v_uf = tk.StringVar(value=empresa.uf.upper() if empresa else
                                 (dados.uf or "") if dados else "")
        self.v_cert = tk.StringVar(value=empresa.certificado if empresa else "")
        self.v_senha = tk.StringVar()
        self.v_nfe = tk.BooleanVar(value=empresa.consultar_nfe if empresa else True)
        self.v_nfse = tk.BooleanVar(value=empresa.consultar_nfse if empresa else True)

        f = ttk.Frame(self, padding=12)
        f.pack(fill="both", expand=True)
        f.columnconfigure(1, weight=1)
        linha = 0
        if self.thumb:
            linha = self._bloco_windows(f)
        else:
            linha = self._bloco_pfx(f)

        ttk.Label(f, text="Nome:").grid(row=linha, column=0, sticky="w", pady=3)
        ttk.Entry(f, textvariable=self.v_nome, width=48).grid(row=linha, column=1, columnspan=2,
                                                              sticky="ew")
        ttk.Label(f, text="CNPJ:").grid(row=linha + 1, column=0, sticky="w", pady=3)
        ttk.Entry(f, textvariable=self.v_cnpj, width=24).grid(row=linha + 1, column=1, sticky="w")
        ttk.Label(f, text="UF:").grid(row=linha + 2, column=0, sticky="w", pady=3)
        ttk.Combobox(f, textvariable=self.v_uf, values=list(cfgmod.CODIGOS_UF), width=6,
                     state="readonly").grid(row=linha + 2, column=1, sticky="w")
        ttk.Checkbutton(f, text="Consultar NF-e", variable=self.v_nfe).grid(
            row=linha + 3, column=1, sticky="w")
        ttk.Checkbutton(f, text="Consultar NFS-e", variable=self.v_nfse).grid(
            row=linha + 4, column=1, sticky="w")
        bt = ttk.Frame(f)
        bt.grid(row=linha + 5, column=0, columnspan=3, sticky="e", pady=(12, 0))
        ttk.Button(bt, text="Salvar", style="Principal.TButton", command=self._salvar).pack(
            side="left", padx=4)
        ttk.Button(bt, text="Cancelar", command=self.destroy).pack(side="left")
        self.abrir()

    # certificado instalado no Windows: informações somente leitura
    def _bloco_windows(self, f) -> int:
        d = self.dados
        if d is None and self.empresa:  # edição: tenta ler o certificado instalado
            try:
                d = Certificado.do_windows(self.thumb).dados
            except Exception as e:
                log.warning("Não foi possível ler o certificado %s: %s", self.thumb, e)
        box = ttk.LabelFrame(f, text="Certificado instalado no Windows", padding=8)
        box.grid(row=0, column=0, columnspan=3, sticky="ew", pady=(0, 8))
        box.columnconfigure(1, weight=1)
        itens = [("Titular:", d.titular if d else "(não foi possível ler o certificado)"),
                 ("Emissor:", d.emissor if d else ""),
                 ("Validade:", fmt_validade(d.validade) if d else ""),
                 ("Impressão digital:", self.thumb)]
        for i, (rot, val) in enumerate(itens):
            ttk.Label(box, text=rot).grid(row=i, column=0, sticky="nw", padx=(0, 8))
            ttk.Label(box, text=val, wraplength=380, justify="left").grid(row=i, column=1, sticky="w")
        return 1

    # arquivo .pfx + senha
    def _bloco_pfx(self, f) -> int:
        ttk.Label(f, text="Certificado (.pfx/.p12):").grid(row=0, column=0, sticky="w", pady=3)
        ttk.Entry(f, textvariable=self.v_cert, width=48).grid(row=0, column=1, sticky="ew")
        ttk.Button(f, text="Procurar...", command=self._procurar).grid(row=0, column=2, padx=(6, 0))
        ttk.Label(f, text="Senha do certificado:").grid(row=1, column=0, sticky="w", pady=3)
        ttk.Entry(f, textvariable=self.v_senha, show="*", width=30).grid(row=1, column=1, sticky="w")
        ttk.Button(f, text="Ler dados do certificado", command=self._ler_dados).grid(
            row=1, column=2, padx=(6, 0))
        if self.empresa:
            ttk.Label(f, text="(deixe a senha em branco para manter a atual)",
                      foreground="gray").grid(row=2, column=1, sticky="w")
        return 3

    def _procurar(self) -> None:
        arq = filedialog.askopenfilename(
            parent=self, title="Selecionar certificado",
            filetypes=[("Certificado digital", "*.pfx *.p12"), ("Todos os arquivos", "*.*")])
        if not arq:
            return
        origem = Path(arq).resolve()
        base = caminhos.pasta_base().resolve()
        pasta_cert = base / "certificados"
        if base not in origem.parents and messagebox.askyesno(
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

    def _senha_efetiva(self) -> str:
        if self.v_senha.get():
            return self.v_senha.get()
        mesmo_arquivo = self.empresa and self.v_cert.get().strip() == self.empresa.certificado
        return self.empresa.senha if mesmo_arquivo else ""

    def _ler_dados(self) -> None:
        cert = self.v_cert.get().strip()
        if not cert:
            return messagebox.showwarning(TITULO, "Selecione o arquivo do certificado.", parent=self)
        try:
            d = Certificado.de_pfx(caminhos.resolver(cert), self._senha_efetiva()).dados
        except Exception as e:
            return erro(self, "Não foi possível ler o certificado. Verifique o arquivo e a senha.", e)
        if not self.v_nome.get().strip():
            self.v_nome.set(nome_certificado(d))
        if d.cnpj:
            self.v_cnpj.set(d.cnpj)
        if d.uf in cfgmod.CODIGOS_UF:
            self.v_uf.set(d.uf)
        self.dados = d
        messagebox.showinfo(TITULO, f"Certificado lido.\nTitular: {d.titular}\n"
                                    f"Validade: {fmt_validade(d.validade)}", parent=self)

    def _salvar(self) -> None:
        aviso = lambda m: messagebox.showwarning(TITULO, m, parent=self)  # noqa: E731
        nome = self.v_nome.get().strip()
        cnpj = cfgmod.so_digitos(self.v_cnpj.get())
        uf = self.v_uf.get().strip().upper()
        cert, senha = self.v_cert.get().strip(), self.v_senha.get()
        if len(cnpj) != 14:
            return aviso("O CNPJ deve ter 14 dígitos.")
        if cnpj in self.cnpjs_existentes:
            return aviso("Já existe uma empresa com esse CNPJ.")
        if uf not in cfgmod.CODIGOS_UF:
            return aviso("Selecione a UF.")
        if not (self.v_nfe.get() or self.v_nfse.get()):
            return aviso("Marque NF-e e/ou NFS-e.")
        armazenada = ""
        if not self.thumb:
            if not cert:
                return aviso("Selecione o arquivo do certificado.")
            mudou = not self.empresa or cert != self.empresa.certificado
            if not senha and mudou:
                return aviso("Informe a senha do certificado.")
            try:
                armazenada = segredo.proteger(senha) if senha else self.empresa.senha_armazenada
            except Exception as e:
                return erro(self, "Não foi possível proteger a senha.", e)
        self.resultado = cfgmod.Empresa(
            nome=nome or cnpj, cnpj=cnpj, uf=uf, certificado="" if self.thumb else cert,
            senha_armazenada=armazenada, thumbprint=self.thumb,
            consultar_nfe=self.v_nfe.get(), consultar_nfse=self.v_nfse.get())
        self.destroy()


class DialogoImportarCertificados(DialogoBase):
    """Lista os certificados instalados e cria empresas para os marcados."""

    def __init__(self, parent, certs: list[DadosCertificado], cnpjs_cadastrados=()):
        super().__init__(parent, "Importar certificados instalados")
        self.certs = certs
        self.cadastrados = set(cnpjs_cadastrados)
        self.resultado: list[cfgmod.Empresa] = []
        self.marcados: set[int] = set()

        f = ttk.Frame(self, padding=12)
        f.pack(fill="both", expand=True)
        ttk.Label(f, text="Clique na primeira coluna para marcar ou desmarcar. Já cadastrados e "
                          "certificados sem CNPJ não podem ser importados.").pack(anchor="w")
        cols = [("chk", "", 36), ("razao", "Razão social", 260), ("cnpj", "CNPJ", 150),
                ("uf", "UF", 40), ("validade", "Validade", 90), ("emissor", "Emissor", 220),
                ("sit", "Situação", 90)]
        meio = ttk.Frame(f)
        meio.pack(fill="both", expand=True, pady=8)
        self.tv = ttk.Treeview(meio, columns=[c[0] for c in cols], show="headings", height=10,
                               selectmode="browse")
        for cid, titulo, larg in cols:
            self.tv.heading(cid, text=titulo)
            self.tv.column(cid, width=larg, anchor="center" if cid == "chk" else "w", stretch=cid != "chk")
        sb = ttk.Scrollbar(meio, orient="vertical", command=self.tv.yview)
        self.tv.configure(yscrollcommand=sb.set)
        self.tv.pack(side="left", fill="both", expand=True)
        sb.pack(side="left", fill="y")
        for i, c in enumerate(certs):
            self.tv.insert("", "end", iid=str(i))
            if self._importavel(c) and self._valido(c):
                self.marcados.add(i)
            self._desenhar(i)
        self.tv.bind("<Button-1>", self._clique)
        self.tv.bind("<space>", lambda _e: self._alternar(self.tv.focus()))

        bt = ttk.Frame(f)
        bt.pack(fill="x")
        ttk.Button(bt, text="Marcar válidos", command=self._marcar_validos).pack(side="left")
        ttk.Button(bt, text="Desmarcar todos", command=self._desmarcar).pack(side="left", padx=6)
        ttk.Button(bt, text="Cancelar", command=self.destroy).pack(side="right")
        ttk.Button(bt, text="Importar", style="Principal.TButton", command=self._importar).pack(
            side="right", padx=6)
        self.abrir()

    @staticmethod
    def _valido(c: DadosCertificado) -> bool:
        return bool(c.validade) and dias_para(c.validade) >= 0

    def _importavel(self, c: DadosCertificado) -> bool:
        return bool(c.cnpj) and c.cnpj not in self.cadastrados

    def _situacao(self, c: DadosCertificado) -> str:
        if not c.cnpj:
            return "Sem CNPJ"
        if c.cnpj in self.cadastrados:
            return "Cadastrada"
        return "OK" if self._valido(c) else "Vencido"

    def _desenhar(self, i: int) -> None:
        c = self.certs[i]
        self.tv.item(str(i), values=(
            "☑" if i in self.marcados else "☐", nome_certificado(c), fmt_cnpj(c.cnpj),
            c.uf or "", fmt_validade(c.validade), c.emissor, self._situacao(c)))

    def _alternar(self, iid: str) -> None:
        if not iid:
            return
        i = int(iid)
        if not self._importavel(self.certs[i]):
            return
        self.marcados ^= {i}
        self._desenhar(i)

    def _clique(self, ev) -> None:
        if self.tv.identify_region(ev.x, ev.y) == "cell" and self.tv.identify_column(ev.x) == "#1":
            self._alternar(self.tv.identify_row(ev.y))

    def _marcar_validos(self) -> None:
        self.marcados = {i for i, c in enumerate(self.certs)
                         if self._importavel(c) and self._valido(c)}
        for i in range(len(self.certs)):
            self._desenhar(i)

    def _desmarcar(self) -> None:
        self.marcados.clear()
        for i in range(len(self.certs)):
            self._desenhar(i)

    def _importar(self) -> None:
        if not self.marcados:
            return messagebox.showinfo(TITULO, "Marque ao menos um certificado.", parent=self)
        novas = []
        for i in sorted(self.marcados):
            c = self.certs[i]
            uf = c.uf if c.uf in cfgmod.CODIGOS_UF else None
            if uf is None:  # passo extra: pergunta a UF
                dlg = DialogoUF(self, nome_certificado(c))
                self.wait_window(dlg)
                uf = dlg.resultado
                if uf is None:
                    continue
            novas.append(cfgmod.Empresa(nome=nome_certificado(c), cnpj=c.cnpj, uf=uf,
                                        thumbprint=c.thumbprint.upper()))
        self.resultado = novas
        self.destroy()


class DialogoConfig(DialogoBase):
    """Configurações gerais (ambiente, pasta de exportação, consulta automática)."""

    def __init__(self, parent, cfg: cfgmod.Config, auto: bool):
        super().__init__(parent, "Configurações")
        self.resultado: dict | None = None
        self.v_amb = tk.StringVar(value="Produção" if cfg.ambiente == "producao" else "Homologação")
        self.v_pasta = tk.StringVar(value=cfg.pasta_exportacao)
        self.v_int = tk.StringVar(value=str(cfg.intervalo_minutos))
        self.v_auto = tk.BooleanVar(value=auto)

        f = ttk.Frame(self, padding=16)
        f.pack(fill="both", expand=True)
        f.columnconfigure(1, weight=1)
        ttk.Label(f, text="Ambiente:").grid(row=0, column=0, sticky="w", pady=4)
        ttk.Combobox(f, textvariable=self.v_amb, values=["Produção", "Homologação"],
                     state="readonly", width=16).grid(row=0, column=1, sticky="w")
        ttk.Label(f, text="Pasta padrão de exportação:").grid(row=1, column=0, sticky="w", pady=4)
        ttk.Entry(f, textvariable=self.v_pasta, width=44).grid(row=1, column=1, sticky="ew")
        ttk.Button(f, text="Procurar...", command=self._procurar).grid(row=1, column=2, padx=(6, 0))
        ttk.Label(f, text="Intervalo de consulta (minutos, mín. 60):").grid(row=2, column=0,
                                                                          sticky="w", pady=4)
        ttk.Spinbox(f, from_=60, to=1440, textvariable=self.v_int, width=8).grid(
            row=2, column=1, sticky="w")
        ttk.Checkbutton(f, text="Consultar automaticamente enquanto o programa estiver aberto",
                        variable=self.v_auto).grid(row=3, column=0, columnspan=3, sticky="w", pady=4)
        bt = ttk.Frame(f)
        bt.grid(row=4, column=0, columnspan=3, sticky="e", pady=(12, 0))
        ttk.Button(bt, text="Salvar", style="Principal.TButton", command=self._salvar).pack(
            side="left", padx=4)
        ttk.Button(bt, text="Cancelar", command=self.destroy).pack(side="left")
        self.abrir()

    def _procurar(self) -> None:
        pasta = filedialog.askdirectory(parent=self, title="Pasta padrão de exportação",
                                        initialdir=str(caminhos.resolver(self.v_pasta.get() or ".")))
        if pasta:
            self.v_pasta.set(caminho_para_config(pasta))

    def _salvar(self) -> None:
        try:
            minutos = int(self.v_int.get())
        except ValueError:
            return messagebox.showwarning(TITULO, "Informe o intervalo em minutos (número inteiro).",
                                          parent=self)
        if minutos < 60:
            minutos = 60
            messagebox.showinfo(TITULO, "O intervalo mínimo é de 60 minutos.", parent=self)
        pasta = self.v_pasta.get().strip()
        if not pasta:
            return messagebox.showwarning(TITULO, "Informe a pasta de exportação.", parent=self)
        self.resultado = {
            "ambiente": "producao" if self.v_amb.get() == "Produção" else "homologacao",
            "pasta_exportacao": caminho_para_config(pasta) if Path(pasta).is_absolute() else pasta,
            "intervalo_minutos": minutos, "auto": self.v_auto.get()}
        self.destroy()


# ---------- exportações (rodam em thread) ----------

@dataclass
class ResultadoExportacao:
    rotulo: str
    destino: Path
    gerados: int = 0
    falhas: list[str] = field(default_factory=list)


def _exp_xml(cfg, linhas, base: Path, nome: str, progresso) -> ResultadoExportacao:
    from . import exportar
    destino = base / f"xml_{nome}"
    return ResultadoExportacao("XML", destino, exportar.xml_pasta(linhas, destino))


def _exp_zip(cfg, linhas, base: Path, nome: str, progresso) -> ResultadoExportacao:
    from . import exportar
    destino = base / f"vorticis_{nome}.zip"
    return ResultadoExportacao("ZIP", destino, exportar.zip_xml(linhas, str(destino)))


def _exp_xlsx(cfg, linhas, base: Path, nome: str, progresso) -> ResultadoExportacao:
    from . import exportar
    destino = base / f"vorticis_{nome}.xlsx"
    return ResultadoExportacao("Planilha", destino, exportar.planilha(linhas, str(destino)))


def _exp_pdf(cfg, linhas, base: Path, nome: str, progresso) -> ResultadoExportacao:
    destino = base / f"pdf_{nome}"
    r = servico.gerar_pdfs(cfg, linhas, destino, progresso)
    return ResultadoExportacao("PDF", destino, r.gerados, list(r.falhas))


EXPORTACOES = {"xml": _exp_xml, "zip": _exp_zip, "xlsx": _exp_xlsx, "pdf": _exp_pdf}


# ---------- aplicação ----------

@dataclass
class InfoCert:
    titular: str
    validade: str
    dias: int


class App:
    def __init__(self, root: tk.Tk, cfg: cfgmod.Config | None = None):
        self.root = root
        self.fila: queue.Queue = queue.Queue()
        self.ocupado = False
        self.auto = False
        self.auto_atual = False
        self.id_auto: str | None = None
        self.linhas: list = []       # linhas exibidas (sqlite3.Row), na ordem da tela
        self.ordem: tuple[str, bool] | None = None
        self.infos: dict[tuple, InfoCert | None] = {}  # cache de certificados das empresas
        self.lendo_certs = False
        self.botoes_acao: list[ttk.Button] = []
        self.imagens: list = []      # evita coleta das imagens pelo GC
        self.caminho_log = servico.configurar_log(console=False)

        root.title(TITULO)
        root.geometry("1200x720")
        root.minsize(1000, 600)
        self._icone()
        self._estilos()

        if cfg is None:
            try:
                cfg = cfgmod.carregar()
            except Exception as e:
                erro(root, "Não foi possível ler o config.toml. Será usada uma configuração vazia.", e)
                cfg = cfgmod.Config()
        self.cfg = cfg
        self.auto = cfg.consulta_automatica
        self.mapa_emp: dict[str, str | None] = {TODAS_EMPRESAS: None}

        self._menu()
        self._cabecalho()
        self._rodape()  # antes do notebook, para a barra de status ficar sempre visível
        self.abas = ttk.Notebook(root)
        self.abas.pack(fill="both", expand=True, padx=8, pady=8)
        self._aba_empresas()
        self._aba_consultas()

        self._atualizar_empresas()
        self.carregar_documentos()
        self._agendar_auto()
        root.after(200, self._ler_fila)
        root.protocol("WM_DELETE_WINDOW", root.destroy)

        if self.cfg.empresas:
            self.abas.select(self.f_doc)
        else:
            self.abas.select(self.f_emp)
            root.after(300, self._boas_vindas)

    # --- aparência ---
    def _icone(self) -> None:
        if sys.platform == "win32":
            try:
                self.root.iconbitmap(str(pasta_recursos() / "icone.ico"))
            except tk.TclError:
                pass
        png = carregar_imagem("icone.png")
        if png:
            self.imagens.append(png)
            try:
                self.root.iconphoto(True, png)
            except tk.TclError:
                pass

    def _estilos(self) -> None:
        s = ttk.Style()
        s.theme_use("clam")
        s.configure("Cab.TFrame", background=AZUL)
        s.configure("Cab.TLabel", background=AZUL, foreground="white")
        s.configure("Principal.TButton", background=LARANJA, foreground=AZUL,
                    font=("TkDefaultFont", 10, "bold"), padding=(14, 6))
        s.map("Principal.TButton", background=[("disabled", "#D9C3A5"), ("active", LARANJA_CLARO)],
              foreground=[("disabled", "#7A7A7A")])
        s.configure("Sec.TButton", background=AZUL_CLARO, foreground="white", padding=(10, 6))
        s.map("Sec.TButton", background=[("disabled", "#4B5D80"), ("active", "#3A5C9E")],
              foreground=[("disabled", "#B0B8C8")])
        s.configure("Tipo.Toolbutton", background=CINZA_CLARO, foreground=AZUL, padding=(14, 5))
        s.map("Tipo.Toolbutton", background=[("selected", LARANJA), ("active", LARANJA_CLARO)],
              foreground=[("selected", AZUL)])
        s.configure("Treeview", rowheight=24)
        s.configure("Treeview.Heading", background=AZUL, foreground="white",
                    font=("TkDefaultFont", 9, "bold"), padding=4)
        s.map("Treeview.Heading", background=[("active", AZUL_CLARO)])
        s.map("Treeview", background=[("selected", AZUL_CLARO)], foreground=[("selected", "white")])
        s.configure("TNotebook.Tab", padding=(16, 6))
        s.configure("Rodape.TFrame", background=CINZA_CLARO)
        s.configure("Rodape.TLabel", background=CINZA_CLARO)

    # --- menu, cabeçalho e barra de status ---
    def _menu(self) -> None:
        barra = tk.Menu(self.root)
        arq = tk.Menu(barra, tearoff=False)
        arq.add_command(label="Importar XMLs de uma pasta...", command=self.importar_xmls)
        arq.add_separator()
        arq.add_command(label="Sair", command=self.root.destroy)
        ajuda = tk.Menu(barra, tearoff=False)
        ajuda.add_command(label="Abrir log", command=self.abrir_log)
        ajuda.add_command(label="Sobre", command=self.sobre)
        barra.add_cascade(label="Arquivo", menu=arq)
        barra.add_command(label="Configurações", command=self.abrir_configuracoes)
        barra.add_cascade(label="Ajuda", menu=ajuda)
        self.root.config(menu=barra)

    def _cabecalho(self) -> None:
        cab = ttk.Frame(self.root, style="Cab.TFrame", padding=(12, 8))
        cab.pack(fill="x")
        logo = carregar_imagem("logo.png", 64)
        if logo:
            self.imagens.append(logo)
            # fundo claro: o logo tem texto escuro e não aparece sobre o azul
            tk.Label(cab, image=logo, bg="white", padx=8, pady=4).pack(side="left", padx=(0, 20))
        else:
            ttk.Label(cab, text=TITULO, style="Cab.TLabel",
                      font=("TkDefaultFont", 16, "bold")).pack(side="left", padx=(0, 20))

        botoes = ttk.Frame(cab, style="Cab.TFrame")
        botoes.pack(side="right")
        self.btn_consultar = ttk.Button(botoes, text="Consultar", style="Principal.TButton",
                                        command=self.consultar)
        self.btn_consultar.pack(side="left", padx=4)
        self.btn_forcar = ttk.Button(botoes, text="Forçar atualização", style="Sec.TButton",
                                     command=self.forcar_atualizacao)
        self.btn_forcar.pack(side="left", padx=4)
        self.botoes_acao += [self.btn_consultar, self.btn_forcar]

        ctl = ttk.Frame(cab, style="Cab.TFrame")
        ctl.pack(side="left", fill="x", expand=True)
        ttk.Label(ctl, text="Empresa:", style="Cab.TLabel").grid(row=0, column=0, sticky="w")
        self.v_emp = tk.StringVar(value=TODAS_EMPRESAS)
        self.cb_emp = ttk.Combobox(ctl, textvariable=self.v_emp, state="readonly", width=38,
                                   values=[TODAS_EMPRESAS])
        self.cb_emp.grid(row=1, column=0, sticky="w", padx=(0, 20), pady=(2, 0))
        self.cb_emp.bind("<<ComboboxSelected>>", lambda _e: self.carregar_documentos())
        ttk.Label(ctl, text="Tipo de consulta:", style="Cab.TLabel").grid(row=0, column=1, sticky="w")
        self.v_tipo = tk.StringVar(value="nfe")
        tipos = ttk.Frame(ctl, style="Cab.TFrame")
        tipos.grid(row=1, column=1, sticky="w", pady=(2, 0))
        for valor, rotulo in TIPOS_CONSULTA:
            ttk.Radiobutton(tipos, text=rotulo, value=valor, variable=self.v_tipo,
                            style="Tipo.Toolbutton", command=self.carregar_documentos).pack(side="left")

    def _rodape(self) -> None:
        f = ttk.Frame(self.root, style="Rodape.TFrame", padding=(8, 3))
        f.pack(side="bottom", fill="x")
        self.v_status = tk.StringVar(value="Pronto.")
        ttk.Label(f, textvariable=self.v_status, style="Rodape.TLabel").pack(side="left")
        self.barra = ttk.Progressbar(f, mode="indeterminate", length=140)
        self.barra.pack(side="right")

    def status(self, msg: str) -> None:
        self.v_status.set(msg)

    def _definir_ocupado(self, ocupado: bool, msg: str | None = None) -> None:
        self.ocupado = ocupado
        for b in self.botoes_acao:
            b.state(["disabled"] if ocupado else ["!disabled"])
        if ocupado:
            self.barra.start(15)
        else:
            self.barra.stop()
        if msg:
            self.status(msg)

    # --- empresa / tipo selecionados no cabeçalho ---
    def _cnpj_filtro(self) -> str | None:
        return self.mapa_emp.get(self.v_emp.get())

    def _nome_empresa(self, cnpj: str) -> str:
        for e in self.cfg.empresas:
            if e.cnpj == cnpj:
                return e.nome
        return fmt_cnpj(cnpj)

    def _atualizar_seletor_empresas(self) -> None:
        atual = self._cnpj_filtro()
        self.mapa_emp = {TODAS_EMPRESAS: None}
        for e in self.cfg.empresas:
            self.mapa_emp[f"{e.nome} — {fmt_cnpj(e.cnpj)}"] = e.cnpj
        self.cb_emp["values"] = list(self.mapa_emp)
        rotulo = next((r for r, c in self.mapa_emp.items() if c == atual), TODAS_EMPRESAS)
        self.v_emp.set(rotulo)

    # --- aba Empresas ---
    def _aba_empresas(self) -> None:
        self.f_emp = f = ttk.Frame(self.abas, padding=8)
        self.abas.add(f, text="Empresas")
        cols = [("nome", "Nome", 230), ("cnpj", "CNPJ", 130), ("uf", "UF", 40),
                ("cert", "Certificado", 280), ("validade", "Validade", 90),
                ("nfe", "NF-e", 50), ("nfse", "NFS-e", 50)]
        meio = ttk.Frame(f)
        meio.pack(fill="both", expand=True)
        self.tv_emp = ttk.Treeview(meio, columns=[c[0] for c in cols], show="headings",
                                   selectmode="browse")
        for cid, titulo, larg in cols:
            self.tv_emp.heading(cid, text=titulo)
            self.tv_emp.column(cid, width=larg, anchor="center" if cid in ("uf", "nfe", "nfse") else "w")
        sb = ttk.Scrollbar(meio, orient="vertical", command=self.tv_emp.yview)
        self.tv_emp.configure(yscrollcommand=sb.set)
        self.tv_emp.pack(side="left", fill="both", expand=True)
        sb.pack(side="left", fill="y")
        self.tv_emp.tag_configure("vencido", foreground="#B00020")
        self.tv_emp.tag_configure("vencendo", foreground="#B35C00")
        self.tv_emp.bind("<Double-1>", lambda _e: self.editar_empresa())

        bt = ttk.Frame(f)
        bt.pack(fill="x", pady=(8, 0))
        ttk.Button(bt, text="Adicionar a partir de certificado instalado...",
                   style="Principal.TButton", command=self.adicionar_de_certificado).pack(side="left")
        for txt, cmd in (("Importar todos os certificados instalados...", self.importar_certificados),
                         ("Adicionar com arquivo .pfx...", self.adicionar_pfx)):
            ttk.Button(bt, text=txt, command=cmd).pack(side="left", padx=(6, 0))
        bt2 = ttk.Frame(f)
        bt2.pack(fill="x", pady=(6, 0))
        for txt, cmd in (("Editar", self.editar_empresa), ("Remover", self.remover_empresa),
                         ("Testar certificado", self.testar_certificado)):
            ttk.Button(bt2, text=txt, command=cmd).pack(side="left", padx=(0, 6))

    @staticmethod
    def _chave_cert(e: cfgmod.Empresa) -> tuple:
        return (e.cnpj, e.thumbprint, e.certificado)

    def _atualizar_empresas(self) -> None:
        self.tv_emp.delete(*self.tv_emp.get_children())
        for i, e in enumerate(self.cfg.empresas):
            chave = self._chave_cert(e)
            info = self.infos.get(chave)
            if e.usa_windows:
                lendo = chave not in self.infos
                cert = "Windows: " + (info.titular if info else "..." if lendo else "(indisponível)")
            else:
                cert = "Arquivo: " + Path(e.certificado).name
            tag = ()
            if info:
                tag = ("vencido",) if info.dias < 0 else ("vencendo",) if info.dias < 30 else ()
            self.tv_emp.insert("", "end", iid=str(i), tags=tag, values=(
                e.nome, fmt_cnpj(e.cnpj), e.uf, cert, info.validade if info else "",
                "Sim" if e.consultar_nfe else "Não", "Sim" if e.consultar_nfse else "Não"))
        self._atualizar_seletor_empresas()
        self._carregar_infos()

    def _carregar_infos(self) -> None:
        """Lê titular/validade dos certificados em segundo plano (pode ser lento)."""
        pend = [e for e in self.cfg.empresas if self._chave_cert(e) not in self.infos]
        if not pend or self.lendo_certs:
            return
        self.lendo_certs = True

        def tarefa():
            for e in pend:
                try:
                    cert = e.carregar_certificado()
                    d = cert.dados
                    info = InfoCert(nome_certificado(d), fmt_validade(cert.validade),
                                    dias_para(cert.validade))
                except Exception as ex:
                    log.warning("Certificado de %s: %s", e.nome, ex)
                    info = None
                self.fila.put(("info_cert", self._chave_cert(e), info))
            self.fila.put(("infos_fim",))

        threading.Thread(target=tarefa, daemon=True).start()

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
        self.carregar_documentos()
        return True

    def _cnpjs_cadastrados(self, exceto: int | None = None) -> set[str]:
        return {e.cnpj for i, e in enumerate(self.cfg.empresas) if i != exceto}

    def _dialogo_empresa(self, idx: int | None, dados: DadosCertificado | None = None) -> None:
        emp = self.cfg.empresas[idx] if idx is not None else None
        dlg = DialogoEmpresa(self.root, emp, dados, self._cnpjs_cadastrados(idx))
        self.root.wait_window(dlg)
        if dlg.resultado is None:
            return
        if idx is None:
            self.cfg.empresas.append(dlg.resultado)
        else:
            self.cfg.empresas[idx] = dlg.resultado
        self._gravar_config()

    def adicionar_pfx(self) -> None:
        self._dialogo_empresa(None)

    def editar_empresa(self) -> None:
        idx = self._empresa_selecionada()
        if idx is not None:
            self._dialogo_empresa(idx)

    def _exige_windows(self) -> bool:
        if sys.platform == "win32":
            return True
        messagebox.showinfo(TITULO, "O uso de certificados instalados só existe no Windows. "
                                    "Use \"Adicionar com arquivo .pfx...\".", parent=self.root)
        return False

    def adicionar_de_certificado(self) -> None:
        if not self._exige_windows():
            return
        try:
            hwnd = int(self.root.wm_frame(), 16)  # janela nativa de seleção do Windows
            dados = servico.selecionar_certificado_windows(hwnd)
        except Exception as e:
            return erro(self.root, "Não foi possível abrir a seleção de certificados.", e)
        if dados is not None:
            self.adicionar_com_dados(dados)

    def adicionar_com_dados(self, dados: DadosCertificado) -> None:
        if dados.cnpj and dados.cnpj in self._cnpjs_cadastrados():
            return messagebox.showwarning(TITULO, "Já existe uma empresa com o CNPJ desse "
                                                  "certificado.", parent=self.root)
        self._dialogo_empresa(None, dados)

    def importar_certificados(self) -> None:
        if not self._exige_windows():
            return
        try:
            certs = servico.certificados_windows()
        except Exception as e:
            return erro(self.root, "Não foi possível listar os certificados instalados.", e)
        self.importar_lista_certificados(certs)

    def importar_lista_certificados(self, certs: list[DadosCertificado]) -> None:
        if not certs:
            return messagebox.showinfo(TITULO, "Nenhum certificado com chave privada foi "
                                               "encontrado no Windows.", parent=self.root)
        dlg = DialogoImportarCertificados(self.root, certs, self._cnpjs_cadastrados())
        self.root.wait_window(dlg)
        if dlg.resultado:
            self.cfg.empresas.extend(dlg.resultado)
            if self._gravar_config():
                self.status(f"{len(dlg.resultado)} empresa(s) importada(s).")

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
        self.status(f"Testando o certificado de {emp.nome}... (o Windows pode pedir o PIN)")

        def tarefa():
            try:
                cert = emp.carregar_certificado()
                self.fila.put(("teste", emp, cert.dados, None))
            except Exception as e:
                self.fila.put(("teste", emp, None, e))

        threading.Thread(target=tarefa, daemon=True).start()

    def _on_teste(self, emp, dados: DadosCertificado | None, exc) -> None:
        self.status("Pronto.")
        if exc is not None:
            return erro(self.root, f"Falha ao abrir o certificado de '{emp.nome}'. "
                                   "Verifique o arquivo/senha ou se o token está conectado.", exc)
        dias = dias_para(dados.validade)
        self.infos[self._chave_cert(emp)] = InfoCert(nome_certificado(dados),
                                                     fmt_validade(dados.validade), dias)
        self._atualizar_empresas()
        msg = (f"Titular: {dados.titular}\nCNPJ: {fmt_cnpj(dados.cnpj)}\n"
               f"Válido até: {fmt_validade(dados.validade)}  ({dias} dia(s))")
        if dias < 0:
            messagebox.showerror(TITULO, msg + "\n\nCERTIFICADO VENCIDO!", parent=self.root)
        elif dias < 30:
            messagebox.showwarning(TITULO, msg + "\n\nAtenção: vence em menos de 30 dias.",
                                   parent=self.root)
        else:
            messagebox.showinfo(TITULO, msg, parent=self.root)

    def _boas_vindas(self) -> None:
        messagebox.showinfo(
            TITULO, f"Bem-vindo ao {TITULO}!\n\nPara começar, cadastre sua empresa. O jeito mais "
                    "fácil é clicar em \"Adicionar a partir de certificado instalado...\" e "
                    "escolher o certificado digital, como no navegador.", parent=self.root)

    # --- aba Consultas ---
    def _aba_consultas(self) -> None:
        self.f_doc = f = ttk.Frame(self.abas, padding=8)
        self.abas.add(f, text="Consultas")

        flt = ttk.Frame(f)
        flt.pack(fill="x")
        self.v_dir = tk.StringVar(value="Todas")
        self.v_sit = tk.StringVar(value="Todos")
        self.v_ini, self.v_fim, self.v_txt = tk.StringVar(), tk.StringVar(), tk.StringVar()
        for col, (rot, var, vals, larg) in enumerate((
                ("Direção", self.v_dir, list(DIRECOES), 10),
                ("Situação/tipo", self.v_sit, list(SITUACOES), 10))):
            ttk.Label(flt, text=rot).grid(row=0, column=col, sticky="w")
            c = ttk.Combobox(flt, textvariable=var, values=vals, width=larg, state="readonly")
            c.grid(row=1, column=col, padx=(0, 8), sticky="w")
            c.bind("<<ComboboxSelected>>", lambda _e: self.carregar_documentos())
        for col, (rot, var) in enumerate((("Período de (DD/MM/AAAA)", self.v_ini),
                                          ("até", self.v_fim)), 2):
            ttk.Label(flt, text=rot).grid(row=0, column=col, sticky="w")
            e = ttk.Entry(flt, textvariable=var, width=14)
            e.grid(row=1, column=col, padx=(0, 8))
            e.bind("<Return>", lambda _e: self.carregar_documentos())
        ttk.Label(flt, text="Texto (chave, emitente, CNPJ)").grid(row=0, column=4, sticky="w")
        e = ttk.Entry(flt, textvariable=self.v_txt, width=30)
        e.grid(row=1, column=4, padx=(0, 8))
        e.bind("<Return>", lambda _e: self.carregar_documentos())
        ttk.Button(flt, text="Filtrar", command=self.carregar_documentos).grid(row=1, column=5)
        ttk.Button(flt, text="Limpar", command=self.limpar_filtros).grid(row=1, column=6, padx=6)

        cols = [("tipo", "Tipo doc", 80), ("direcao", "Direção", 80), ("emissao", "Emissão", 90),
                ("emitente", "Emitente", 230), ("cnpj", "CNPJ emitente", 150),
                ("valor", "Valor", 100), ("chave", "Chave", 330), ("empresa", "Empresa", 160)]
        self.titulos_col = {c[0]: c[1] for c in cols}
        meio = ttk.Frame(f)
        meio.pack(fill="both", expand=True, pady=6)
        self.tv = ttk.Treeview(meio, columns=[c[0] for c in cols], show="headings",
                               selectmode="extended")
        for cid, titulo, larg in cols:
            self.tv.heading(cid, text=titulo, command=lambda c=cid: self.ordenar(c))
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
        self.tv.bind("<<TreeviewSelect>>", lambda _e: self._atualizar_totais())
        for seq in ("<Control-a>", "<Control-A>"):
            self.tv.bind(seq, self.selecionar_tudo)

        self.v_total = tk.StringVar()
        ttk.Label(f, textvariable=self.v_total).pack(anchor="w")
        self._painel_exportacao(f)

    def _painel_exportacao(self, pai) -> None:
        box = ttk.LabelFrame(pai, text="Exportação", padding=8)
        box.pack(fill="x", pady=(8, 0))
        box.columnconfigure(1, weight=1)
        self.v_pasta = tk.StringVar(value=str(self.cfg.caminho_exportacao))
        self.v_escopo = tk.StringVar(value="sel")
        ttk.Label(box, text="Pasta de destino:").grid(row=0, column=0, sticky="w")
        ttk.Entry(box, textvariable=self.v_pasta).grid(row=0, column=1, sticky="ew", padx=6)
        ttk.Button(box, text="Alterar...", command=self.alterar_pasta).grid(row=0, column=2)
        esc = ttk.Frame(box)
        esc.grid(row=1, column=0, columnspan=3, sticky="w", pady=(6, 0))
        ttk.Label(esc, text="Exportar:").pack(side="left")
        ttk.Radiobutton(esc, text="Selecionados", value="sel", variable=self.v_escopo).pack(
            side="left", padx=8)
        ttk.Radiobutton(esc, text="Todos da lista", value="todos", variable=self.v_escopo).pack(
            side="left")
        bt = ttk.Frame(box)
        bt.grid(row=2, column=0, columnspan=3, sticky="w", pady=(6, 0))
        for txt, fmt in (("XML", "xml"), ("XML em lote (ZIP)", "zip"), ("PDF", "pdf"),
                         ("Planilha (Excel)", "xlsx")):
            b = ttk.Button(bt, text=txt, command=lambda f=fmt: self.exportar(f))
            b.pack(side="left", padx=(0, 6))
            self.botoes_acao.append(b)

    # --- lista de documentos ---
    def limpar_filtros(self) -> None:
        self.v_dir.set("Todas")
        self.v_sit.set("Todos")
        for v in (self.v_ini, self.v_fim, self.v_txt):
            v.set("")
        self.carregar_documentos()

    def carregar_documentos(self) -> None:
        try:
            ini = ler_data(self.v_ini.get(), "Data inicial")
            fim = ler_data(self.v_fim.get(), "Data final")
        except ValueError as e:
            return messagebox.showwarning(TITULO, str(e), parent=self.root)
        db = None
        try:
            db = servico.abrir_banco(self.cfg)
            self.linhas = list(db.consultar(
                cnpj=self._cnpj_filtro(), origem=self.v_tipo.get(),
                direcao=DIRECOES[self.v_dir.get()], tipo=SITUACOES[self.v_sit.get()],
                data_ini=ini, data_fim=fim, texto=self.v_txt.get().strip() or None, limite=None))
        except Exception as e:
            return erro(self.root, "Não foi possível consultar o banco de dados.", e)
        finally:
            if db is not None:
                db.con.close()
        if self.ordem:
            self._ordenar_linhas()
        self._preencher()

    def _preencher(self) -> None:
        self.tv.delete(*self.tv.get_children())
        for i, r in enumerate(self.linhas):
            self.tv.insert("", "end", iid=str(i), values=(
                ROTULO_TIPO.get(r["tipo"], r["tipo"]),
                ROTULO_DIRECAO.get(r["direcao"], r["direcao"]),
                fmt_data(r["data_emissao"]), r["emitente_nome"] or "",
                fmt_cnpj(r["emitente_cnpj"]), fmt_valor(r["valor"]), r["chave"],
                self._nome_empresa(r["cnpj_empresa"])))
        self._atualizar_totais()

    def _linhas_selecionadas(self) -> list:
        return [self.linhas[i] for i in sorted(int(s) for s in self.tv.selection())]

    def _atualizar_totais(self) -> None:
        sel = self._linhas_selecionadas()
        base = sel or self.linhas
        soma = sum(r["valor"] for r in base if r["tipo"] in ("completo", "resumo") and r["valor"])
        n = len(self.linhas)
        self.v_total.set(f"{n} documento{'s' if n != 1 else ''}  |  {len(sel)} "
                         f"selecionado{'s' if len(sel) != 1 else ''}  |  Soma: R$ {fmt_valor(soma)}"
                         + ("  (seleção)" if sel else ""))

    def selecionar_tudo(self, _ev=None) -> str:
        self.tv.selection_set(self.tv.get_children())
        return "break"

    def _chave_ordem(self, col: str, r):
        if col == "valor":
            return r["valor"] if r["valor"] is not None else float("-inf")
        if col == "emissao":
            return r["data_emissao"] or ""
        if col == "empresa":
            return self._nome_empresa(r["cnpj_empresa"]).lower()
        campo = {"tipo": "tipo", "direcao": "direcao", "emitente": "emitente_nome",
                 "cnpj": "emitente_cnpj", "chave": "chave"}[col]
        return (r[campo] or "").lower()

    def _ordenar_linhas(self) -> None:
        col, desc = self.ordem
        self.linhas.sort(key=lambda r: self._chave_ordem(col, r), reverse=desc)
        for c, titulo in self.titulos_col.items():
            seta = (" ▼" if desc else " ▲") if c == col else ""
            self.tv.heading(c, text=titulo + seta)

    def ordenar(self, col: str) -> None:
        desc = bool(self.ordem and self.ordem[0] == col and not self.ordem[1])
        self.ordem = (col, desc)
        self._ordenar_linhas()
        self._preencher()

    def _copiar_chave(self, _ev=None) -> None:
        sel = self._linhas_selecionadas()
        if not sel:
            return
        chave = sel[0]["chave"]
        self.root.clipboard_clear()
        self.root.clipboard_append(chave)
        self.status(f"Chave copiada para a área de transferência: {chave}")

    # --- exportação ---
    def alterar_pasta(self) -> None:
        pasta = filedialog.askdirectory(parent=self.root, title="Pasta de destino",
                                        initialdir=self.v_pasta.get() or None)
        if pasta:
            self.v_pasta.set(pasta)

    def exportar(self, formato: str, pasta: str | Path | None = None) -> None:
        """Exporta em segundo plano. `pasta` permite injetar o destino (padrão: campo da tela)."""
        if self.ocupado:
            return messagebox.showinfo(TITULO, "Aguarde a operação em andamento.", parent=self.root)
        linhas = self.linhas if self.v_escopo.get() == "todos" else self._linhas_selecionadas()
        if not linhas:
            return messagebox.showinfo(
                TITULO, "Não há documentos para exportar: selecione linhas na lista ou escolha "
                        "\"Todos da lista\".", parent=self.root)
        texto = str(pasta or self.v_pasta.get()).strip()
        if not texto:
            return messagebox.showwarning(TITULO, "Informe a pasta de destino.", parent=self.root)
        base = caminhos.resolver(texto)
        carimbo = datetime.now().strftime("%Y%m%d_%H%M%S")
        alvo = self._cnpj_filtro() or "todas"
        nome = f"{self.v_tipo.get()}_{alvo}_{carimbo}" if formato in ("zip", "xlsx") else carimbo
        dados = [dict(r) for r in linhas]  # cópia simples para a thread
        cfg, fila = self.cfg, self.fila
        self._definir_ocupado(True, "Exportando...")

        def tarefa():
            try:
                res = EXPORTACOES[formato](cfg, dados, base, nome,
                                           lambda m: fila.put(("progresso", m)))
                fila.put(("exportado", res))
            except Exception as e:
                fila.put(("erro", "Falha na exportação.", e))

        threading.Thread(target=tarefa, daemon=True).start()

    def _on_exportado(self, res: ResultadoExportacao) -> None:
        self._definir_ocupado(False, f"{res.rotulo}: {res.gerados} arquivo(s) em {res.destino}")
        texto = f"{res.rotulo}: {res.gerados} gerado(s).\nLocal: {res.destino}"
        if res.falhas:
            texto += f"\n\nFalhas/avisos ({len(res.falhas)}):\n{texto_falhas(res.falhas)}"
        if res.gerados:
            if messagebox.askyesno(TITULO, texto + "\n\nAbrir a pasta?", parent=self.root):
                pasta = res.destino if res.destino.is_dir() else res.destino.parent
                try:
                    abrir_caminho(pasta)
                except Exception as e:
                    erro(self.root, "Não foi possível abrir a pasta.", e)
        else:
            messagebox.showwarning(TITULO, texto, parent=self.root)

    # --- menu ---
    def abrir_log(self) -> None:
        try:
            abrir_caminho(self.caminho_log)
        except Exception as e:
            erro(self.root, f"Não foi possível abrir o log:\n{self.caminho_log}", e)

    def sobre(self) -> None:
        messagebox.showinfo(
            f"Sobre o {TITULO}", f"{TITULO}\n\nConsulta e organiza NF-e e NFS-e das suas empresas "
                                 "usando o certificado digital.\n\nPrograma portátil: dados e "
                                 "configurações ficam na pasta do programa.", parent=self.root)

    def abrir_configuracoes(self) -> None:
        dlg = DialogoConfig(self.root, self.cfg, self.auto)
        self.root.wait_window(dlg)
        r = dlg.resultado
        if r is None:
            return
        self.cfg.ambiente = r["ambiente"]
        self.cfg.pasta_exportacao = r["pasta_exportacao"]
        self.cfg.intervalo_minutos = r["intervalo_minutos"]
        self.auto = self.cfg.consulta_automatica = r["auto"]
        self.v_pasta.set(str(self.cfg.caminho_exportacao))
        if self._gravar_config():
            self._agendar_auto()
            self.status("Configurações salvas.")

    # --- importação de XMLs (thread) ---
    def importar_xmls(self) -> None:
        if self.ocupado:
            return messagebox.showinfo(TITULO, "Aguarde a operação em andamento.", parent=self.root)
        if not self.cfg.empresas:
            return messagebox.showinfo(TITULO, "Cadastre uma empresa antes de importar.",
                                       parent=self.root)
        pasta = filedialog.askdirectory(parent=self.root, title="Pasta com os XMLs")
        if not pasta:
            return
        self._definir_ocupado(True, "Importando XMLs...")

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
    def forcar_atualizacao(self) -> None:
        if self.ocupado:
            return
        if messagebox.askyesno(
                TITULO, "Forçar atualização ignora a espera de 1 hora entre consultas.\n\n"
                        "Se a SEFAZ/ADN considerar o acesso como consumo indevido (cStat 656), "
                        "aquele CNPJ ficará bloqueado por 1 hora. Isso também afeta o Auditto, "
                        "que usa o mesmo CNPJ.\n\nDeseja continuar?",
                icon="warning", parent=self.root):
            self.consultar(forcar=True)

    def consultar(self, forcar: bool = False, auto: bool = False) -> None:
        if self.ocupado:
            return
        self.auto_atual = auto
        if not self.cfg.empresas:
            self.abas.select(self.f_emp)
            return messagebox.showinfo(TITULO, "Cadastre uma empresa antes de consultar.",
                                       parent=self.root)
        # consulta automática varre tudo; a manual segue o cabeçalho
        cnpj = None if auto else self._cnpj_filtro()
        cnpjs = [cnpj] if cnpj else None
        tipos = None if auto else [self.v_tipo.get()]
        self._definir_ocupado(True, "Consultando...")
        cfg, fila = self.cfg, self.fila

        def tarefa():
            try:
                db = servico.abrir_banco(cfg)  # conexão própria da thread
                try:
                    res = servico.executar_ciclo(cfg, db, lambda m: fila.put(("progresso", m)),
                                                 cnpjs=cnpjs, tipos=tipos, forcar=forcar)
                finally:
                    db.con.close()
                fila.put(("consulta_fim", res))
            except servico.ConsultaEmAndamento as e:
                fila.put(("em_andamento", str(e)))
            except Exception as e:
                fila.put(("erro", "Falha na consulta.", e))

        threading.Thread(target=tarefa, daemon=True).start()

    def _on_consulta_fim(self, resultados) -> None:
        self._definir_ocupado(False)
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
        if any("656" in (r.erro or "") for r in resultados):
            texto += ("\n\nA SEFAZ respondeu 656 (consumo indevido): esse CNPJ fica bloqueado "
                      "por 1 hora, inclusive para outros sistemas como o Auditto.")
        (messagebox.showwarning if com_erro else messagebox.showinfo)(TITULO, texto, parent=self.root)

    def _on_em_andamento(self, msg: str) -> None:
        self._definir_ocupado(False, "Já existe uma consulta em andamento.")
        if not self.auto_atual:
            messagebox.showinfo(TITULO, msg, parent=self.root)

    def _on_importado(self, n: int) -> None:
        self._definir_ocupado(False, f"Importação concluída: {n} documento(s) novo(s).")
        self.carregar_documentos()

    def _on_erro(self, msg: str, exc: Exception) -> None:
        self._definir_ocupado(False, "Erro. Veja o log para detalhes.")
        erro(self.root, msg, exc)

    def _on_progresso(self, msg: str) -> None:
        self.status(msg)

    def _on_info_cert(self, chave, info) -> None:
        self.infos[chave] = info
        self._atualizar_empresas_sem_recarregar()

    def _on_infos_fim(self) -> None:
        self.lendo_certs = False
        self._carregar_infos()  # empresas novas durante a leitura

    def _atualizar_empresas_sem_recarregar(self) -> None:
        sel = self.tv_emp.selection()
        self._atualizar_empresas()
        if sel and self.tv_emp.exists(sel[0]):
            self.tv_emp.selection_set(sel[0])

    def _ler_fila(self) -> None:
        try:
            while True:
                msg = self.fila.get_nowait()
                try:
                    getattr(self, "_on_" + msg[0])(*msg[1:])
                except Exception:
                    log.exception("Falha ao tratar a mensagem %s", msg[0])
        except queue.Empty:
            pass
        self.root.after(200, self._ler_fila)

    # --- consulta automática ---
    def _agendar_auto(self) -> None:
        if self.id_auto:
            self.root.after_cancel(self.id_auto)
            self.id_auto = None
        if self.auto:
            ms = max(60, self.cfg.intervalo_minutos) * 60 * 1000
            self.id_auto = self.root.after(ms, self._tick_auto)

    def _tick_auto(self) -> None:
        self.id_auto = None
        if self.cfg.empresas and not self.ocupado:
            self.consultar(auto=True)  # sem pop-up de sucesso
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
