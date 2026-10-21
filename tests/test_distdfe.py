"""Testes da interpretação da resposta SOAP do NFeDistribuicaoDFe."""
import base64
import gzip

from lxml import etree

from monitor_notas.nfe.distdfe import (
    DOCUMENTOS_LOCALIZADOS,
    NENHUM_DOCUMENTO,
    interpretar,
)
from monitor_notas.nfe.parser import de_xml

CNPJ = "12345678000195"
CHAVE_1 = "35260311222333000181550010000000011000000011"
CHAVE_2 = "35260312345678000195550010000000021000000022"

XML_RES_1 = f"""<resNFe xmlns="http://www.portalfiscal.inf.br/nfe">
  <chNFe>{CHAVE_1}</chNFe><CNPJ>11222333000181</CNPJ><xNome>FORN 1</xNome>
  <dhEmi>2026-03-01T08:00:00-03:00</dhEmi><vNF>10.00</vNF>
</resNFe>"""

XML_PROC_2 = f"""<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00">
  <NFe><infNFe Id="NFe{CHAVE_2}">
    <emit><CNPJ>{CNPJ}</CNPJ><xNome>MINHA</xNome></emit>
    <dest><CNPJ>11222333000181</CNPJ></dest>
    <ide><dhEmi>2026-03-02T08:00:00-03:00</dhEmi></ide>
    <total><ICMSTot><vNF>20.00</vNF></ICMSTot></total>
  </infNFe></NFe>
</nfeProc>"""


def _doczip(xml: str) -> str:
    return base64.b64encode(gzip.compress(xml.encode("utf-8"))).decode()


def _envelope(cstat: str, ult: str, maxi: str, docs: list[tuple[str, str, str]]) -> etree._Element:
    """Monta um envelope SOAP 1.2 de resposta; docs = [(NSU, schema, xml), ...]."""
    itens = "".join(
        f'<docZip NSU="{nsu}" schema="{schema}">{_doczip(xml)}</docZip>'
        for nsu, schema, xml in docs
    )
    lote = f"<loteDistDFeInt>{itens}</loteDistDFeInt>" if docs else ""
    soap = f"""<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">
  <soap12:Body>
    <nfeDistDFeInteresseResponse xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeDistribuicaoDFe">
      <nfeDistDFeInteresseResult>
        <retDistDFeInt xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.01">
          <tpAmb>1</tpAmb>
          <verAplic>SVRS202603</verAplic>
          <cStat>{cstat}</cStat>
          <xMotivo>Documento localizado</xMotivo>
          <dhResp>2026-03-20T10:00:00-03:00</dhResp>
          <ultNSU>{ult}</ultNSU>
          <maxNSU>{maxi}</maxNSU>
          {lote}
        </retDistDFeInt>
      </nfeDistDFeInteresseResult>
    </nfeDistDFeInteresseResponse>
  </soap12:Body>
</soap12:Envelope>"""
    return etree.fromstring(soap.encode("utf-8"))


def test_interpreta_lote_com_dois_docs():
    raiz = _envelope(
        DOCUMENTOS_LOCALIZADOS, "000000000000002", "000000000000005",
        [
            ("000000000000001", "resNFe_v1.01.xsd", XML_RES_1),
            ("000000000000002", "procNFe_v4.00.xsd", XML_PROC_2),
        ],
    )
    ret = interpretar(raiz)

    assert ret.cstat == "138"
    assert ret.motivo == "Documento localizado"
    assert ret.ult_nsu == "000000000000002"
    assert ret.max_nsu == "000000000000005"
    assert len(ret.docs) == 2

    d1, d2 = ret.docs
    assert d1.nsu == "000000000000001"
    assert d1.schema == "resNFe_v1.01.xsd"
    assert d1.xml == XML_RES_1
    assert d2.nsu == "000000000000002"
    assert d2.schema == "procNFe_v4.00.xsd"
    assert d2.xml == XML_PROC_2


def test_docs_decodificados_sao_parseaveis():
    raiz = _envelope(
        DOCUMENTOS_LOCALIZADOS, "000000000000002", "000000000000005",
        [
            ("000000000000001", "resNFe_v1.01.xsd", XML_RES_1),
            ("000000000000002", "procNFe_v4.00.xsd", XML_PROC_2),
        ],
    )
    ret = interpretar(raiz)
    doc1 = de_xml(ret.docs[0].xml, CNPJ, ret.docs[0].nsu)
    doc2 = de_xml(ret.docs[1].xml, CNPJ, ret.docs[1].nsu)
    assert doc1.tipo == "resumo" and doc1.chave == CHAVE_1 and doc1.valor == 10.0
    assert doc2.tipo == "completo" and doc2.chave == CHAVE_2 and doc2.direcao == "emitida"


def test_nenhum_documento_sem_lote():
    raiz = _envelope(NENHUM_DOCUMENTO, "000000000000005", "000000000000005", [])
    ret = interpretar(raiz)
    assert ret.cstat == "137"
    assert ret.docs == []
    assert ret.ult_nsu == "000000000000005"
    assert ret.max_nsu == "000000000000005"
