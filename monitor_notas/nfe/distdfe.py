"""NFeDistribuicaoDFe (Ambiente Nacional): notas e eventos em que o CNPJ é interessado."""
from __future__ import annotations

import base64
import gzip
from dataclasses import dataclass

from lxml import etree

from ..certificado import Certificado
from .soap import NFE, achar, chamar, texto

WSDL = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe"
URLS = {
    1: "https://www1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx",
    2: "https://hom1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx",
}

# cStat relevantes
DOCUMENTOS_LOCALIZADOS = "138"
NENHUM_DOCUMENTO = "137"
CONSUMO_INDEVIDO = "656"


@dataclass
class DocZip:
    nsu: str
    schema: str
    xml: str


@dataclass
class RetornoDist:
    cstat: str
    motivo: str
    ult_nsu: str
    max_nsu: str
    docs: list[DocZip]


def _montar(tp_amb: int, cuf: int, cnpj: str, filtro: etree._Element) -> etree._Element:
    op = etree.Element(f"{{{WSDL}}}nfeDistDFeInteresse", nsmap={None: WSDL})
    msg = etree.SubElement(op, f"{{{WSDL}}}nfeDadosMsg")
    dist = etree.SubElement(msg, f"{{{NFE}}}distDFeInt", nsmap={None: NFE}, versao="1.01")
    etree.SubElement(dist, f"{{{NFE}}}tpAmb").text = str(tp_amb)
    etree.SubElement(dist, f"{{{NFE}}}cUFAutor").text = str(cuf)
    etree.SubElement(dist, f"{{{NFE}}}CNPJ").text = cnpj
    dist.append(filtro)
    return op


def _filtro_nsu(ult_nsu: str) -> etree._Element:
    f = etree.Element(f"{{{NFE}}}distNSU")
    etree.SubElement(f, f"{{{NFE}}}ultNSU").text = ult_nsu.zfill(15)
    return f


def _filtro_chave(chave: str) -> etree._Element:
    f = etree.Element(f"{{{NFE}}}consChNFe")
    etree.SubElement(f, f"{{{NFE}}}chNFe").text = chave
    return f


def interpretar(raiz: etree._Element) -> RetornoDist:
    ret = achar(raiz, "retDistDFeInt")
    docs = []
    for dz in ret.iter(f"{{{NFE}}}docZip"):
        xml = gzip.decompress(base64.b64decode(dz.text)).decode("utf-8")
        docs.append(DocZip(nsu=dz.get("NSU"), schema=dz.get("schema"), xml=xml))
    return RetornoDist(
        cstat=texto(ret, "cStat"),
        motivo=texto(ret, "xMotivo", ""),
        ult_nsu=texto(ret, "ultNSU", "0"),
        max_nsu=texto(ret, "maxNSU", "0"),
        docs=docs,
    )


class ClienteDistDFe:
    def __init__(self, cert: Certificado, tp_amb: int, cuf: int, cnpj: str,
                 verificar_tls: bool | str = True):
        self.cert, self.tp_amb, self.cuf, self.cnpj = cert, tp_amb, cuf, cnpj
        self.verificar_tls = verificar_tls

    def _consultar(self, filtro: etree._Element) -> RetornoDist:
        raiz = chamar(URLS[self.tp_amb], f"{WSDL}/nfeDistDFeInteresse",
                      _montar(self.tp_amb, self.cuf, self.cnpj, filtro),
                      self.cert, self.verificar_tls)
        return interpretar(raiz)

    def por_nsu(self, ult_nsu: str) -> RetornoDist:
        return self._consultar(_filtro_nsu(ult_nsu))

    def por_chave(self, chave: str) -> RetornoDist:
        return self._consultar(_filtro_chave(chave))
