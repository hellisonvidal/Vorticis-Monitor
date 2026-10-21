"""Converte XMLs da NFS-e nacional (nota e eventos) em Documento.

Busca campos por nome local, para tolerar variações de versão do leiaute.
"""
from __future__ import annotations

from lxml import etree

from ..armazenamento import Documento


def _primeiro(raiz, caminho: str) -> str | None:
    """caminho tipo 'emit/CNPJ', procurado em qualquer profundidade e ignorando namespace."""
    partes = caminho.split("/")
    xp = ".//" + "/".join(f"*[local-name()='{p}']" for p in partes)
    achados = raiz.xpath(xp)
    return achados[0].text if achados and achados[0].text else None


def _float(v):
    try:
        return float(v) if v else None
    except ValueError:
        return None


def de_xml(xml: str, cnpj_empresa: str, nsu: int | None, chave: str | None,
           tipo_evento: str | None = None) -> Documento | None:
    raiz = etree.fromstring(xml.encode("utf-8"))
    tag = etree.QName(raiz).localname
    base = dict(origem="nfse", cnpj_empresa=cnpj_empresa,
                nsu=str(nsu) if nsu is not None else None, xml=xml)

    if tag == "NFSe":
        inf = raiz.xpath(".//*[local-name()='infNFSe']")
        inf = inf[0] if inf else raiz
        chave = chave or (inf.get("Id") or "")[3:]
        emit = _primeiro(inf, "emit/CNPJ") or _primeiro(inf, "emit/CPF") \
            or _primeiro(inf, "prest/CNPJ")
        toma = _primeiro(inf, "toma/CNPJ") or _primeiro(inf, "toma/CPF")
        return Documento(
            tipo="completo", chave=chave,
            direcao="emitida" if emit == cnpj_empresa else "recebida",
            emitente_cnpj=emit, emitente_nome=_primeiro(inf, "emit/xNome"),
            destinatario_cnpj=toma,
            valor=_float(_primeiro(inf, "vServPrest/vServ") or _primeiro(inf, "valores/vLiq")),
            data_emissao=_primeiro(inf, "dhEmi") or _primeiro(inf, "dhProc"), **base)

    if tag in ("evento", "Evento", "pedRegEvento"):
        cod = tipo_evento or ""
        if not cod:
            det = raiz.xpath(".//*[local-name()='infPedReg']/*[starts-with(local-name(), 'e')]")
            cod = etree.QName(det[0]).localname if det else ""
        return Documento(
            tipo="evento", direcao="recebida",
            chave=chave or _primeiro(raiz, "chNFSe") or "",
            evento=cod, data_emissao=_primeiro(raiz, "dhEvento") or _primeiro(raiz, "dhProc"),
            **base)

    return None
