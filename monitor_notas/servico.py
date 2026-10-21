"""Operações de alto nível usadas pela interface gráfica e pela linha de comando."""
from __future__ import annotations

import logging
from dataclasses import dataclass
from logging.handlers import RotatingFileHandler
from pathlib import Path
from typing import Callable

from .armazenamento import Armazenamento
from .caminhos import resolver
from .certificado import Certificado
from .config import Config
from .nfe import monitor as mon_nfe
from .nfe import parser as parser_nfe
from .nfse import monitor as mon_nfse

log = logging.getLogger("monitor_notas")


def configurar_log(console: bool = True) -> Path:
    arq = resolver("dados/monitor.log")
    arq.parent.mkdir(parents=True, exist_ok=True)
    fmt = logging.Formatter("%(asctime)s %(levelname)s %(message)s")
    raiz = logging.getLogger()
    raiz.setLevel(logging.INFO)
    if not any(isinstance(h, RotatingFileHandler) for h in raiz.handlers):
        h = RotatingFileHandler(arq, maxBytes=2_000_000, backupCount=3, encoding="utf-8")
        h.setFormatter(fmt)
        raiz.addHandler(h)
    if console and not any(type(h) is logging.StreamHandler for h in raiz.handlers):
        h = logging.StreamHandler()
        h.setFormatter(fmt)
        raiz.addHandler(h)
    return arq


def abrir_banco(cfg: Config) -> Armazenamento:
    return Armazenamento(str(cfg.caminho_banco), str(cfg.caminho_xml))


@dataclass
class ResultadoEmpresa:
    empresa: str
    nfe: int | None = None
    nfse: int | None = None
    erro: str | None = None


def executar_ciclo(cfg: Config, db: Armazenamento,
                   progresso: Callable[[str], None] | None = None) -> list[ResultadoEmpresa]:
    avisar = progresso or (lambda _m: None)
    resultados = []
    for emp in cfg.empresas:
        r = ResultadoEmpresa(emp.nome)
        resultados.append(r)
        avisar(f"{emp.nome}: carregando certificado...")
        try:
            cert = Certificado.de_pfx(emp.caminho_certificado, emp.senha)
        except Exception as e:  # senha errada, arquivo ausente
            r.erro = f"certificado: {e}"
            log.error("[%s] %s", emp.nome, r.erro)
            continue
        for ativo, nome, mod, attr in ((emp.consultar_nfe, "NF-e", mon_nfe, "nfe"),
                                       (emp.consultar_nfse, "NFS-e", mon_nfse, "nfse")):
            if not ativo:
                continue
            avisar(f"{emp.nome}: consultando {nome}...")
            try:
                n = mod.executar(cfg, emp, cert, db)
                setattr(r, attr, n)
                log.info("[%s] %s: %d documento(s) novo(s)", emp.nome, nome, n)
            except Exception as e:
                r.erro = f"{nome}: {e}"
                log.exception("[%s] %s: falha no ciclo", emp.nome, nome)
    avisar("Consulta concluída.")
    return resultados


def importar_pasta(cfg: Config, db: Armazenamento, pasta: str | Path) -> int:
    """Importa XMLs de NF-e (ex.: emitidas pelo seu emissor) para as empresas configuradas."""
    cnpjs = {e.cnpj for e in cfg.empresas}
    total = 0
    for arq in Path(pasta).rglob("*.xml"):
        xml = arq.read_text(encoding="utf-8", errors="replace")
        for cnpj in cnpjs:
            try:
                doc = parser_nfe.de_xml(xml, cnpj)
            except Exception as e:
                log.warning("%s: %s", arq, e)
                break
            if doc and (doc.emitente_cnpj == cnpj or doc.destinatario_cnpj == cnpj):
                total += db.salvar(doc)
    log.info("Importados %d documento(s) novo(s) de %s", total, pasta)
    return total
