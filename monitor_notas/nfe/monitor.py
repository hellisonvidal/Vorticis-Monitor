"""Ciclo de captura de NF-e de uma empresa (somente consulta).

Sem manifestação, notas recebidas chegam como resumo (resNFe). Quando outro sistema
(ex.: Auditto) manifesta, a SEFAZ libera o XML completo em um novo NSU e ele chega aqui também.
"""
from __future__ import annotations

import logging
from datetime import datetime, timedelta, timezone

from ..armazenamento import Armazenamento
from ..certificado import Certificado
from ..config import Config, Empresa
from . import parser
from .distdfe import CONSUMO_INDEVIDO, DOCUMENTOS_LOCALIZADOS, NENHUM_DOCUMENTO, ClienteDistDFe

log = logging.getLogger(__name__)

ESPERA = timedelta(hours=1)   # regra da SEFAZ após 137/656
MAX_LOTES_POR_CICLO = 20      # cada lote traz até 50 documentos


def _agora() -> datetime:
    return datetime.now(timezone.utc)


def executar(cfg: Config, emp: Empresa, cert: Certificado, db: Armazenamento) -> int:
    """Baixa novos documentos via DistDFe. Retorna a quantidade de documentos novos."""
    k_nsu, k_prox = f"nfe:{emp.cnpj}:ult_nsu", f"nfe:{emp.cnpj}:proxima"
    proxima = db.obter_estado(k_prox)
    if proxima and _agora() < datetime.fromisoformat(proxima):
        log.info("[%s] NF-e: aguardando até %s (regra de consumo da SEFAZ)", emp.nome, proxima)
        return 0

    cli = ClienteDistDFe(cert, cfg.tp_amb, emp.cuf, emp.cnpj, cfg.tls)
    ult = db.obter_estado(k_nsu, "0")
    novos = 0
    for _ in range(MAX_LOTES_POR_CICLO):
        ret = cli.por_nsu(ult)
        log.info("[%s] NF-e: cStat %s %s (ultNSU %s / maxNSU %s, %d docs)",
                 emp.nome, ret.cstat, ret.motivo, ret.ult_nsu, ret.max_nsu, len(ret.docs))
        if ret.cstat not in (DOCUMENTOS_LOCALIZADOS, NENHUM_DOCUMENTO):
            if ret.cstat == CONSUMO_INDEVIDO:
                db.gravar_estado(k_prox, (_agora() + ESPERA).isoformat())
            else:
                log.error("[%s] NF-e: retorno inesperado %s %s", emp.nome, ret.cstat, ret.motivo)
            break
        for dz in ret.docs:
            doc = parser.de_xml(dz.xml, emp.cnpj, dz.nsu)
            if doc and db.salvar(doc):
                novos += 1
        ult = ret.ult_nsu
        db.gravar_estado(k_nsu, ult)
        if ret.cstat == NENHUM_DOCUMENTO or int(ret.ult_nsu) >= int(ret.max_nsu):
            db.gravar_estado(k_prox, (_agora() + ESPERA).isoformat())
            break

    return novos
