"""Chamada SOAP 1.2 com mTLS aos webservices da NF-e."""
from __future__ import annotations

from lxml import etree

from ..certificado import Certificado

SOAP12 = "http://www.w3.org/2003/05/soap-envelope"
NFE = "http://www.portalfiscal.inf.br/nfe"


def chamar(url: str, acao: str, corpo: etree._Element, cert: Certificado,
           verificar_tls: bool | str = True, timeout: int = 60) -> etree._Element:
    env = etree.Element(f"{{{SOAP12}}}Envelope", nsmap={"soap12": SOAP12})
    body = etree.SubElement(env, f"{{{SOAP12}}}Body")
    body.append(corpo)
    dados = etree.tostring(env, xml_declaration=True, encoding="utf-8")
    resp = cert.requisitar(
        "POST", url, dados,
        {"Content-Type": f'application/soap+xml; charset=utf-8; action="{acao}"'},
        verificar_tls, timeout)
    resp.raise_for_status()
    return etree.fromstring(resp.content)


def achar(raiz: etree._Element, tag: str) -> etree._Element:
    el = raiz.find(f".//{{{NFE}}}{tag}")
    if el is None:
        raise RuntimeError(f"Resposta sem <{tag}>: {etree.tostring(raiz)[:500]!r}")
    return el


def texto(el: etree._Element | None, caminho: str, padrao: str | None = None) -> str | None:
    if el is None:
        return padrao
    # caminho no formato "a/b/c" sem namespace
    xp = "/".join(f"{{{NFE}}}{p}" for p in caminho.split("/"))
    achado = el.find(xp)
    return achado.text if achado is not None and achado.text is not None else padrao
