"""Ciclo de captura de NFS-e (emitidas e tomadas) pelo ADN."""
from __future__ import annotations

import logging
from datetime import datetime, timedelta, timezone

from ..armazenamento import Armazenamento
from ..certificado import Certificado
from ..config import Config, Empresa
from . import parser
from .adn import ClienteADN

log = logging.getLogger(__name__)

# Sem limite oficial publicado; ficamos conservadores para conviver com outro monitor (Auditto)
ESPERA_SEM_DOCS = timedelta(hours=1)
MAX_LOTES_POR_CICLO = 20


def _agora():
    return datetime.now(timezone.utc)


def executar(cfg: Config, emp: Empresa, cert: Certificado, db: Armazenamento) -> int:
    k_nsu, k_prox = f"nfse:{emp.cnpj}:ult_nsu", f"nfse:{emp.cnpj}:proxima"
    proxima = db.obter_estado(k_prox)
    if proxima and _agora() < datetime.fromisoformat(proxima):
        log.info("[%s] NFS-e: aguardando até %s", emp.nome, proxima)
        return 0

    cli = ClienteADN(cert, cfg.tp_amb, emp.cnpj, cfg.verificar_tls)
    ult = int(db.obter_estado(k_nsu, "0"))
    novos = 0
    for _ in range(MAX_LOTES_POR_CICLO):
        ret = cli.por_nsu(ult)
        log.info("[%s] NFS-e: HTTP %s %s (%d docs a partir do NSU %s)",
                 emp.nome, ret.http, ret.status, len(ret.itens), ult)
        if ret.erro:
            log.error("[%s] NFS-e: erro ADN %s", emp.nome, ret.erros)
            break
        for it in ret.itens:
            doc = parser.de_xml(it.xml, emp.cnpj, it.nsu, it.chave, it.tipo_evento)
            if doc and db.salvar(doc):
                novos += 1
        if ret.maior_nsu is not None:
            ult = ret.maior_nsu
            db.gravar_estado(k_nsu, str(ult))
        if ret.fim:
            db.gravar_estado(k_prox, (_agora() + ESPERA_SEM_DOCS).isoformat())
            break
    return novos
