"""Linha de comando do monitor.

  python -m monitor_notas executar          # um ciclo para todas as empresas
  python -m monitor_notas servico           # roda em loop (intervalo do config)
  python -m monitor_notas importar PASTA    # importa XMLs de NF-e emitidas (do seu emissor)
  python -m monitor_notas manifestar CNPJ CHAVE [--evento 210200] [--justificativa ...]
  python -m monitor_notas listar [--cnpj CNPJ] [--limite 50]
  python -m monitor_notas certificados      # mostra titular e validade
"""
from __future__ import annotations

import argparse
import logging
import time
from pathlib import Path

from . import config as cfgmod
from .armazenamento import Armazenamento
from .certificado import Certificado
from .nfe import manifestacao
from .nfe import monitor as mon_nfe
from .nfe import parser as parser_nfe
from .nfse import monitor as mon_nfse

log = logging.getLogger("monitor_notas")


def _ciclo(cfg, db) -> None:
    for emp in cfg.empresas:
        try:
            cert = Certificado.de_pfx(emp.certificado, emp.senha)
        except Exception as e:  # senha errada, arquivo ausente
            log.error("[%s] certificado: %s", emp.nome, e)
            continue
        for ativo, nome, mod in ((emp.consultar_nfe, "NF-e", mon_nfe),
                                 (emp.consultar_nfse, "NFS-e", mon_nfse)):
            if not ativo:
                continue
            try:
                n = mod.executar(cfg, emp, cert, db)
                log.info("[%s] %s: %d documento(s) novo(s)", emp.nome, nome, n)
            except Exception:
                log.exception("[%s] %s: falha no ciclo", emp.nome, nome)


def _importar(cfg, db, pasta: str) -> None:
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
    log.info("Importados %d documento(s) novo(s)", total)


def main(argv=None) -> None:
    ap = argparse.ArgumentParser(prog="monitor_notas")
    ap.add_argument("--config", default="config.toml")
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("executar")
    sub.add_parser("servico")
    sub.add_parser("certificados")
    p = sub.add_parser("importar"); p.add_argument("pasta")
    p = sub.add_parser("manifestar")
    p.add_argument("cnpj"); p.add_argument("chave")
    p.add_argument("--evento", default=manifestacao.CIENCIA, choices=list(manifestacao.DESCRICOES))
    p.add_argument("--justificativa")
    p = sub.add_parser("listar")
    p.add_argument("--cnpj"); p.add_argument("--limite", type=int, default=50)
    args = ap.parse_args(argv)

    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    cfg = cfgmod.carregar(args.config)
    db = Armazenamento(cfg.banco, cfg.pasta_xml)

    if args.cmd == "executar":
        _ciclo(cfg, db)
    elif args.cmd == "servico":
        while True:
            _ciclo(cfg, db)
            time.sleep(cfg.intervalo_minutos * 60)
    elif args.cmd == "importar":
        _importar(cfg, db, args.pasta)
    elif args.cmd == "certificados":
        for e in cfg.empresas:
            c = Certificado.de_pfx(e.certificado, e.senha)
            print(f"{e.nome} ({e.cnpj}): {c.titular} — válido até {c.validade:%d/%m/%Y}")
    elif args.cmd == "manifestar":
        emp = next(e for e in cfg.empresas if e.cnpj == "".join(filter(str.isdigit, args.cnpj)))
        cert = Certificado.de_pfx(emp.certificado, emp.senha)
        ret = manifestacao.manifestar(cert, cfg.tp_amb, emp.cnpj, args.chave, args.evento,
                                      args.justificativa, cfg.verificar_tls)
        print(ret.cstat, ret.motivo)
    elif args.cmd == "listar":
        for r in db.listar(args.cnpj, args.limite):
            print(f"{r['origem']:5} {r['direcao']:9} {r['tipo']:8} {r['chave']} "
                  f"{(r['data_emissao'] or '')[:10]} {r['valor'] or '':>12} {r['emitente_nome'] or ''}")


if __name__ == "__main__":
    main()
