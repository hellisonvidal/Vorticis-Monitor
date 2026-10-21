"""Manifestação do Destinatário (NFeRecepcaoEvento4, Ambiente Nacional, cOrgao 91)."""
from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

from lxml import etree

from ..certificado import Certificado
from .assinatura import assinar
from .soap import NFE, chamar, texto

WSDL = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeRecepcaoEvento4"
URLS = {
    1: "https://www.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx",
    2: "https://hom1.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx",
}

CIENCIA = "210210"
CONFIRMACAO = "210200"
DESCONHECIMENTO = "210220"
NAO_REALIZADA = "210240"

DESCRICOES = {
    CIENCIA: "Ciencia da Operacao",
    CONFIRMACAO: "Confirmacao da Operacao",
    DESCONHECIMENTO: "Desconhecimento da Operacao",
    NAO_REALIZADA: "Operacao nao Realizada",
}

# cStat do evento considerados sucesso (135 registrado, 136 registrado sem vínculo, 573 duplicidade)
SUCESSO = {"135", "136", "573"}


@dataclass
class RetornoEvento:
    cstat: str
    motivo: str
    xml: str

    @property
    def ok(self) -> bool:
        return self.cstat in SUCESSO


def montar_evento(tp_amb: int, cnpj: str, chave: str, tp_evento: str, cert: Certificado,
                  justificativa: str | None = None, agora: datetime | None = None) -> etree._Element:
    if tp_evento == NAO_REALIZADA and not (justificativa and 15 <= len(justificativa) <= 255):
        raise ValueError("Operação não realizada exige justificativa de 15 a 255 caracteres")
    agora = agora or datetime.now(timezone(timedelta(hours=-3)))
    n = lambda pai, tag, txt=None, **a: _sub(pai, tag, txt, **a)  # noqa: E731

    env = etree.Element(f"{{{NFE}}}envEvento", nsmap={None: NFE}, versao="1.00")
    n(env, "idLote", agora.strftime("%y%m%d%H%M%S%f")[:15])
    evento = n(env, "evento", versao="1.00")
    inf = n(evento, "infEvento", Id=f"ID{tp_evento}{chave}01")
    n(inf, "cOrgao", "91")
    n(inf, "tpAmb", str(tp_amb))
    n(inf, "CNPJ", cnpj)
    n(inf, "chNFe", chave)
    n(inf, "dhEvento", agora.replace(microsecond=0).isoformat())
    n(inf, "tpEvento", tp_evento)
    n(inf, "nSeqEvento", "1")
    n(inf, "verEvento", "1.00")
    det = n(inf, "detEvento", versao="1.00")
    n(det, "descEvento", DESCRICOES[tp_evento])
    if tp_evento == NAO_REALIZADA:
        n(det, "xJust", justificativa)
    assinar(evento, inf, cert)
    return env


def _sub(pai, tag, txt=None, **attrs):
    el = etree.SubElement(pai, f"{{{NFE}}}{tag}", **attrs)
    if txt is not None:
        el.text = txt
    return el


def manifestar(cert: Certificado, tp_amb: int, cnpj: str, chave: str,
               tp_evento: str = CIENCIA, justificativa: str | None = None,
               verificar_tls: bool | str = True) -> RetornoEvento:
    env = montar_evento(tp_amb, cnpj, chave, tp_evento, cert, justificativa)
    msg = etree.Element(f"{{{WSDL}}}nfeDadosMsg", nsmap={None: WSDL})
    msg.append(env)
    raiz = chamar(URLS[tp_amb], f"{WSDL}/nfeRecepcaoEvento", msg, cert, verificar_tls)
    ret = raiz.find(f".//{{{NFE}}}retEvento/{{{NFE}}}infEvento")
    if ret is None:  # lote rejeitado como um todo
        lote = raiz.find(f".//{{{NFE}}}retEnvEvento")
        return RetornoEvento(texto(lote, "cStat", "?"), texto(lote, "xMotivo", ""),
                             etree.tostring(raiz, encoding="unicode"))
    return RetornoEvento(texto(ret, "cStat"), texto(ret, "xMotivo", ""),
                         etree.tostring(raiz, encoding="unicode"))
