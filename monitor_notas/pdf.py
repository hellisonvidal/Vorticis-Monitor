"""Geração do PDF da DANFE a partir do XML completo da NF-e (nfeProc).

Usa a biblioteca brazilfiscalreport (pura Python, baseada em fpdf2).
"""
from __future__ import annotations

import re
from pathlib import Path

from lxml import etree

from brazilfiscalreport.danfe import Danfe

NFE_NS = "http://www.portalfiscal.inf.br/nfe"
_RAIZES_VALIDAS = frozenset({f"{{{NFE_NS}}}nfeProc", f"{{{NFE_NS}}}NFe"})
_DECLARACAO_XML = re.compile(r"^\s*<\?xml[^>]*\?>")


def _parse(xml: str | bytes) -> etree._Element:
    """Converte o XML em elemento raiz. Não resolve entidades externas."""
    if isinstance(xml, str):
        # lxml recusa str com declaração de encoding; a leitura de str já
        # é Unicode, então a declaração é descartada.
        xml = _DECLARACAO_XML.sub("", xml, count=1)
    parser = etree.XMLParser(resolve_entities=False, no_network=True)
    return etree.fromstring(xml, parser)


def pode_gerar_danfe(xml: str) -> bool:
    """True se o XML for uma NF-e completa (raiz nfeProc ou NFe)."""
    try:
        raiz = _parse(xml)
    except (etree.XMLSyntaxError, ValueError):
        return False
    return raiz.tag in _RAIZES_VALIDAS


def danfe(xml: str, destino: str | Path) -> Path:
    """Gera o PDF da DANFE em `destino` e retorna o caminho.

    Levanta ValueError se o XML não for uma NF-e completa (ex.: resNFe).
    """
    try:
        raiz = _parse(xml)
    except (etree.XMLSyntaxError, ValueError) as exc:
        raise ValueError("XML não é uma NF-e completa (nfeProc)") from exc
    if raiz.tag not in _RAIZES_VALIDAS:
        raise ValueError("XML não é uma NF-e completa (nfeProc)")

    if isinstance(xml, bytes):
        xml = xml.decode("utf-8")
    documento = Danfe(xml=xml)

    destino = Path(destino)
    documento.output(str(destino))
    return destino
