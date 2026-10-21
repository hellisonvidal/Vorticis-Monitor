"""Converte XMLs da NF-e (resumos, notas completas e eventos) em Documento."""
from __future__ import annotations

from lxml import etree

from ..armazenamento import Documento
from .soap import NFE, texto


def _float(v: str | None) -> float | None:
    return float(v) if v else None


def _cnpj(el, caminho_base: str = "") -> str | None:
    p = f"{caminho_base}/" if caminho_base else ""
    return texto(el, f"{p}CNPJ") or texto(el, f"{p}CPF")


def de_xml(xml: str, cnpj_empresa: str, nsu: str | None = None) -> Documento | None:
    raiz = etree.fromstring(xml.encode("utf-8"))
    tag = etree.QName(raiz).localname
    base = dict(origem="nfe", cnpj_empresa=cnpj_empresa, nsu=nsu, xml=xml)

    if tag == "resNFe":
        emit = _cnpj(raiz)
        return Documento(
            tipo="resumo", chave=texto(raiz, "chNFe"),
            direcao="emitida" if emit == cnpj_empresa else "recebida",
            emitente_cnpj=emit, emitente_nome=texto(raiz, "xNome"),
            valor=_float(texto(raiz, "vNF")), data_emissao=texto(raiz, "dhEmi"), **base)

    if tag in ("nfeProc", "NFe"):
        inf = raiz.find(f".//{{{NFE}}}infNFe")
        emit = _cnpj(inf, "emit")
        return Documento(
            tipo="completo", chave=inf.get("Id")[3:],
            direcao="emitida" if emit == cnpj_empresa else "recebida",
            emitente_cnpj=emit, emitente_nome=texto(inf, "emit/xNome"),
            destinatario_cnpj=_cnpj(inf, "dest"),
            valor=_float(texto(inf, "total/ICMSTot/vNF")),
            data_emissao=texto(inf, "ide/dhEmi") or texto(inf, "ide/dEmi"), **base)

    if tag == "resEvento":
        return Documento(
            tipo="evento", direcao="recebida", chave=texto(raiz, "chNFe"),
            evento=texto(raiz, "tpEvento", ""), data_emissao=texto(raiz, "dhEvento"),
            emitente_cnpj=_cnpj(raiz), **base)

    if tag == "procEventoNFe":
        inf = raiz.find(f".//{{{NFE}}}evento/{{{NFE}}}infEvento")
        return Documento(
            tipo="evento", direcao="recebida", chave=texto(inf, "chNFe"),
            evento=texto(inf, "tpEvento", ""), data_emissao=texto(inf, "dhEvento"),
            emitente_cnpj=_cnpj(inf), **base)

    return None
