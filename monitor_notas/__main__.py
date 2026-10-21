"""Ponto de entrada.

Sem argumentos abre a interface gráfica. Com argumentos (útil no Agendador de Tarefas):
  MonitorNotas.exe executar            # um ciclo de consulta para todas as empresas
  MonitorNotas.exe importar PASTA      # importa XMLs de NF-e emitidas (do seu emissor)
  MonitorNotas.exe listar [--cnpj CNPJ] [--limite 50]
  MonitorNotas.exe certificados        # mostra titular e validade
"""
from __future__ import annotations

import argparse
import sys

from . import config as cfgmod
from . import servico
from .certificado import Certificado


def main(argv=None) -> None:
    argv = sys.argv[1:] if argv is None else argv
    if not argv:
        from .gui import iniciar
        iniciar()
        return

    ap = argparse.ArgumentParser(prog="MonitorNotas")
    ap.add_argument("--config", default=cfgmod.ARQUIVO_PADRAO)
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("executar")
    sub.add_parser("certificados")
    p = sub.add_parser("importar")
    p.add_argument("pasta")
    p = sub.add_parser("listar")
    p.add_argument("--cnpj")
    p.add_argument("--limite", type=int, default=50)
    args = ap.parse_args(argv)

    servico.configurar_log(console=sys.stderr is not None)
    cfg = cfgmod.carregar(args.config)
    db = servico.abrir_banco(cfg)

    if args.cmd == "executar":
        res = servico.executar_ciclo(cfg, db)
        sys.exit(1 if any(r.erro for r in res) else 0)
    elif args.cmd == "importar":
        servico.importar_pasta(cfg, db, args.pasta)
    elif args.cmd == "certificados":
        for e in cfg.empresas:
            c = Certificado.de_pfx(e.caminho_certificado, e.senha)
            print(f"{e.nome} ({e.cnpj}): {c.titular} — válido até {c.validade:%d/%m/%Y}")
    elif args.cmd == "listar":
        for r in db.listar(args.cnpj, args.limite):
            print(f"{r['origem']:5} {r['direcao']:9} {r['tipo']:8} {r['chave']} "
                  f"{(r['data_emissao'] or '')[:10]} {r['valor'] or '':>12} {r['emitente_nome'] or ''}")


if __name__ == "__main__":
    main()
