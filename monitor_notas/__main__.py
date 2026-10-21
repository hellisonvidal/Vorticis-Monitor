"""Ponto de entrada.

Sem argumentos abre a interface gráfica. Com argumentos (útil no Agendador de Tarefas):
  VorticisMonitor.exe executar            # um ciclo de consulta para todas as empresas
  VorticisMonitor.exe importar PASTA      # importa XMLs de NF-e emitidas (do seu emissor)
  VorticisMonitor.exe listar [--cnpj CNPJ] [--limite 50]
  VorticisMonitor.exe certificados        # mostra titular e validade
"""
from __future__ import annotations

import argparse
import logging
import os
import sys

from . import config as cfgmod
from . import servico


def main(argv=None) -> None:
    argv = sys.argv[1:] if argv is None else argv
    if not argv:
        from .gui import iniciar
        iniciar()
        return

    ap = argparse.ArgumentParser(prog="VorticisMonitor")
    ap.add_argument("--config", default=cfgmod.ARQUIVO_PADRAO)
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("executar")
    sub.add_parser("certificados")
    sub.add_parser("diagnostico")  # usado no CI: confere se o .exe empacotou tudo
    p = sub.add_parser("importar")
    p.add_argument("pasta")
    p = sub.add_parser("listar")
    p.add_argument("--cnpj")
    p.add_argument("--limite", type=int, default=50)
    args = ap.parse_args(argv)

    # No .exe sem console (windowed) stdout/stderr são None: a saída vai só para o log
    if sys.stdout is None:
        sys.stdout = open(os.devnull, "w")
    servico.configurar_log(console=sys.stderr is not None)
    cfg = cfgmod.carregar(args.config)
    db = servico.abrir_banco(cfg)

    if args.cmd == "executar":
        try:
            res = servico.executar_ciclo(cfg, db)
        except servico.ConsultaEmAndamento as e:
            logging.getLogger("monitor_notas").warning("%s", e)
            sys.exit(0)
        sys.exit(1 if any(r.erro for r in res) else 0)
    elif args.cmd == "diagnostico":
        sys.exit(_diagnostico())
    elif args.cmd == "importar":
        servico.importar_pasta(cfg, db, args.pasta)
    elif args.cmd == "certificados":
        for e in cfg.empresas:
            c = e.carregar_certificado()
            print(f"{e.nome} ({e.cnpj}): {c.titular} — válido até {c.validade:%d/%m/%Y}")
    elif args.cmd == "listar":
        for r in db.listar(args.cnpj, args.limite):
            print(f"{r['origem']:5} {r['direcao']:9} {r['tipo']:8} {r['chave']} "
                  f"{(r['data_emissao'] or '')[:10]} {r['valor'] or '':>12} {r['emitente_nome'] or ''}")


def _diagnostico() -> int:
    """Importa todos os módulos, abre/fecha a janela e gera uma DANFE de exemplo. Resultado no log."""
    log = logging.getLogger("monitor_notas")
    falhas = 0
    for nome in ("monitor_notas.gui", "monitor_notas.pdf", "monitor_notas.exportar", "openpyxl",
                 "brazilfiscalreport.danfe", "lxml.etree", "requests", "cryptography"):
        try:
            __import__(nome)
            log.info("diagnóstico: %s OK", nome)
        except Exception as e:
            falhas += 1
            log.error("diagnóstico: %s FALHOU: %s", nome, e)
    if sys.platform == "win32":
        try:
            from . import servico
            log.info("diagnóstico: %d certificado(s) no Windows", len(servico.certificados_windows()))
        except Exception as e:
            falhas += 1
            log.error("diagnóstico: winstore FALHOU: %s", e)
    try:
        import tkinter as tk

        from .gui import App
        raiz = tk.Tk()
        App(raiz, cfgmod.Config())
        raiz.update()
        raiz.destroy()
        log.info("diagnóstico: interface OK")
    except Exception as e:
        falhas += 1
        log.error("diagnóstico: interface FALHOU: %s", e)
    log.info("diagnóstico: %s", "OK" if not falhas else f"{falhas} falha(s)")
    return 1 if falhas else 0


if __name__ == "__main__":
    main()
