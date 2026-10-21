"""Operações de alto nível usadas pela interface gráfica e pela linha de comando."""
from __future__ import annotations

import contextlib
import logging
import os
import sys
from dataclasses import dataclass
from logging.handlers import RotatingFileHandler
from pathlib import Path
from typing import Callable, Iterable

from .armazenamento import Armazenamento
from .caminhos import resolver
from .certificado import DadosCertificado, ler_dados
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


class ConsultaEmAndamento(RuntimeError):
    pass


@contextlib.contextmanager
def _trava_consulta():
    """Impede duas consultas simultâneas (ex.: interface aberta + tarefa agendada),
    que gastariam o limite de consumo da SEFAZ em dobro. A trava some se o processo morrer."""
    arq = resolver("dados/consulta.lock")
    arq.parent.mkdir(parents=True, exist_ok=True)
    f = open(arq, "a+")
    try:
        try:
            if sys.platform == "win32":
                import msvcrt
                f.seek(0)
                msvcrt.locking(f.fileno(), msvcrt.LK_NBLCK, 1)
            else:
                import fcntl
                fcntl.flock(f.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            raise ConsultaEmAndamento("Já existe uma consulta em andamento (outra janela ou "
                                      "a tarefa agendada). Tente novamente em alguns minutos.")
        f.seek(0); f.truncate(); f.write(str(os.getpid())); f.flush()
        yield
    finally:
        f.close()  # fechar o arquivo libera a trava


@dataclass
class ResultadoEmpresa:
    empresa: str
    nfe: int | None = None
    nfse: int | None = None
    erro: str | None = None


def executar_ciclo(cfg: Config, db: Armazenamento,
                   progresso: Callable[[str], None] | None = None,
                   cnpjs: Iterable[str] | None = None, tipos: Iterable[str] | None = None,
                   forcar: bool = False) -> list[ResultadoEmpresa]:
    """Consulta as empresas (todas ou só `cnpjs`) e os tipos ("nfe", "nfse"; padrão: ambos).

    `forcar` ignora a espera local de 1 h. A SEFAZ pode responder 656 (consumo indevido) e
    bloquear novas consultas daquele CNPJ por 1 h — por isso a interface pede confirmação.
    """
    filtro = set(cnpjs) if cnpjs else None
    empresas = [e for e in cfg.empresas if filtro is None or e.cnpj in filtro]
    with _trava_consulta():
        return _ciclo(cfg, db, progresso or (lambda _m: None), empresas,
                      set(tipos or ("nfe", "nfse")), forcar)


def _ciclo(cfg: Config, db: Armazenamento, avisar: Callable[[str], None], empresas,
           tipos: set[str], forcar: bool) -> list[ResultadoEmpresa]:
    resultados = []
    for emp in empresas:
        r = ResultadoEmpresa(emp.nome)
        resultados.append(r)
        avisar(f"{emp.nome}: carregando certificado...")
        try:
            cert = emp.carregar_certificado()
        except Exception as e:  # senha errada, arquivo ausente, token desconectado
            r.erro = f"certificado: {e}"
            log.error("[%s] %s", emp.nome, r.erro)
            continue
        for ativo, nome, mod, attr in ((emp.consultar_nfe, "NF-e", mon_nfe, "nfe"),
                                       (emp.consultar_nfse, "NFS-e", mon_nfse, "nfse")):
            if not ativo or attr not in tipos:
                continue
            avisar(f"{emp.nome}: consultando {nome}...")
            try:
                n = mod.executar(cfg, emp, cert, db, forcar=forcar)
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


# --- certificados instalados no Windows -----------------------------------------------------

def certificados_windows() -> list[DadosCertificado]:
    """Certificados com chave privada do usuário atual (vazio fora do Windows)."""
    if sys.platform != "win32":
        return []
    from cryptography import x509

    from . import winstore
    return [ler_dados(x509.load_der_x509_certificate(d)) for d in winstore.listar_der()]


def selecionar_certificado_windows(hwnd: int = 0) -> DadosCertificado | None:
    """Janela nativa de seleção (como no navegador). Mostra só certificados válidos com chave."""
    from datetime import datetime, timezone

    from cryptography import x509

    from . import winstore

    def valido(der: bytes) -> bool:
        try:
            return x509.load_der_x509_certificate(der).not_valid_after_utc > datetime.now(timezone.utc)
        except ValueError:
            return False

    tp = winstore.selecionar(hwnd, filtro=valido)
    if not tp:
        return None
    return ler_dados(x509.load_der_x509_certificate(winstore.obter_der(tp)))


# --- PDF ----------------------------------------------------------------------------------

@dataclass
class ResultadoPdf:
    gerados: int = 0
    falhas: list[str] = None

    def __post_init__(self):
        self.falhas = self.falhas or []


def gerar_pdfs(cfg: Config, linhas, pasta: str | Path,
               progresso: Callable[[str], None] | None = None) -> ResultadoPdf:
    """NF-e: DANFE gerada localmente a partir do XML completo (resumos não têm dados suficientes).
    NFS-e: DANFSe oficial baixada do ADN com o certificado da empresa."""
    from . import pdf
    from .nfse.adn import baixar_danfse

    avisar = progresso or (lambda _m: None)
    destino = Path(pasta)
    destino.mkdir(parents=True, exist_ok=True)
    res = ResultadoPdf()
    certs: dict[str, object] = {}
    empresas = {e.cnpj: e for e in cfg.empresas}
    alvo = [l for l in linhas if l["tipo"] == "completo"]
    ignorados = len(linhas) - len(alvo)
    for i, l in enumerate(alvo, 1):
        avisar(f"Gerando PDF {i}/{len(alvo)}...")
        arq = destino / f"{l['chave']}.pdf"
        try:
            if l["origem"] == "nfe":
                pdf.danfe(l["xml"], arq)
            else:
                emp = empresas.get(l["cnpj_empresa"])
                if emp is None:
                    raise RuntimeError("empresa não cadastrada")
                if emp.cnpj not in certs:
                    certs[emp.cnpj] = emp.carregar_certificado()
                arq.write_bytes(baixar_danfse(certs[emp.cnpj], cfg.tp_amb, l["chave"], cfg.tls))
            res.gerados += 1
        except Exception as e:
            res.falhas.append(f"{l['chave']}: {e}")
            log.warning("PDF %s: %s", l["chave"], e)
    if ignorados:
        res.falhas.append(f"{ignorados} resumo(s)/evento(s) ignorado(s): só notas com XML "
                          "completo geram PDF")
    return res
