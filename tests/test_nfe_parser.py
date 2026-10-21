"""Testes do parser de XMLs da NF-e (resumos, notas completas e eventos)."""
from monitor_notas.nfe.parser import de_xml

NS = 'xmlns="http://www.portalfiscal.inf.br/nfe"'

CNPJ_EMPRESA = "12345678000195"
CNPJ_OUTRO = "11222333000181"
CHAVE = "35260312345678000195550010000001231000000123"
assert len(CHAVE) == 44

RES_NFE = f"""<resNFe {NS}>
  <chNFe>{CHAVE}</chNFe>
  <CNPJ>{CNPJ_OUTRO}</CNPJ>
  <xNome>FORNECEDOR LTDA</xNome>
  <dhEmi>2026-03-10T10:00:00-03:00</dhEmi>
  <vNF>1500.75</vNF>
</resNFe>"""

NFE_PROC = f"""<nfeProc {NS} versao="4.00">
  <NFe>
    <infNFe Id="NFe{CHAVE}" versao="4.00">
      <ide>
        <cUF>35</cUF>
        <dhEmi>2026-03-10T10:00:00-03:00</dhEmi>
      </ide>
      <emit>
        <CNPJ>{CNPJ_OUTRO}</CNPJ>
        <xNome>FORNECEDOR LTDA</xNome>
      </emit>
      <dest>
        <CNPJ>{CNPJ_EMPRESA}</CNPJ>
        <xNome>MINHA EMPRESA</xNome>
      </dest>
      <total>
        <ICMSTot>
          <vNF>1500.75</vNF>
        </ICMSTot>
      </total>
    </infNFe>
  </NFe>
</nfeProc>"""

RES_EVENTO = f"""<resEvento {NS} versao="1.01">
  <chNFe>{CHAVE}</chNFe>
  <CNPJ>{CNPJ_OUTRO}</CNPJ>
  <dhEvento>2026-03-11T09:00:00-03:00</dhEvento>
  <tpEvento>110111</tpEvento>
  <nSeqEvento>1</nSeqEvento>
  <xEvento>Cancelamento</xEvento>
</resEvento>"""

PROC_EVENTO = f"""<procEventoNFe {NS} versao="1.00">
  <evento versao="1.00">
    <infEvento Id="ID1101112602603123456780001955500100000012310001">
      <cOrgao>35</cOrgao>
      <tpAmb>1</tpAmb>
      <CNPJ>{CNPJ_OUTRO}</CNPJ>
      <chNFe>{CHAVE}</chNFe>
      <dhEvento>2026-03-11T09:00:00-03:00</dhEvento>
      <tpEvento>110111</tpEvento>
      <nSeqEvento>1</nSeqEvento>
    </infEvento>
  </evento>
</procEventoNFe>"""


def test_res_nfe_recebida():
    doc = de_xml(RES_NFE, CNPJ_EMPRESA, nsu="7")
    assert doc is not None
    assert doc.origem == "nfe"
    assert doc.tipo == "resumo"
    assert doc.chave == CHAVE
    assert doc.direcao == "recebida"
    assert doc.valor == 1500.75
    assert doc.emitente_cnpj == CNPJ_OUTRO
    assert doc.emitente_nome == "FORNECEDOR LTDA"
    assert doc.data_emissao == "2026-03-10T10:00:00-03:00"
    assert doc.nsu == "7"
    assert doc.evento == ""


def test_res_nfe_emitida_quando_emitente_e_a_empresa():
    doc = de_xml(RES_NFE.replace(CNPJ_OUTRO, CNPJ_EMPRESA), CNPJ_EMPRESA)
    assert doc.direcao == "emitida"
    assert doc.emitente_cnpj == CNPJ_EMPRESA


def test_nfe_proc_completa():
    doc = de_xml(NFE_PROC, CNPJ_EMPRESA, nsu="9")
    assert doc.tipo == "completo"
    assert doc.chave == CHAVE
    assert doc.direcao == "recebida"
    assert doc.emitente_cnpj == CNPJ_OUTRO
    assert doc.emitente_nome == "FORNECEDOR LTDA"
    assert doc.destinatario_cnpj == CNPJ_EMPRESA
    assert doc.valor == 1500.75
    assert doc.data_emissao == "2026-03-10T10:00:00-03:00"
    assert doc.nsu == "9"
    assert doc.xml == NFE_PROC


def test_nfe_proc_emitida():
    doc = de_xml(NFE_PROC, CNPJ_OUTRO)
    assert doc.direcao == "emitida"


def test_nfe_sem_proc_usa_raiz_nfe():
    # NFe sem o envelope nfeProc (raiz já é NFe)
    xml = f"<NFe {NS}>" + NFE_PROC.split("<NFe>")[1].split("</NFe>")[0] + "</NFe>"
    doc = de_xml(xml, CNPJ_EMPRESA)
    assert doc.tipo == "completo"
    assert doc.chave == CHAVE


def test_res_evento_cancelamento():
    doc = de_xml(RES_EVENTO, CNPJ_EMPRESA)
    assert doc.tipo == "evento"
    assert doc.chave == CHAVE
    assert doc.evento == "110111"
    assert doc.direcao == "recebida"
    assert doc.data_emissao == "2026-03-11T09:00:00-03:00"
    assert doc.emitente_cnpj == CNPJ_OUTRO


def test_proc_evento_nfe_cancelamento():
    doc = de_xml(PROC_EVENTO, CNPJ_EMPRESA)
    assert doc.tipo == "evento"
    assert doc.chave == CHAVE
    assert doc.evento == "110111"
    assert doc.direcao == "recebida"
    assert doc.data_emissao == "2026-03-11T09:00:00-03:00"
    assert doc.emitente_cnpj == CNPJ_OUTRO


def test_raiz_desconhecida_retorna_none():
    assert de_xml(f"<retOutraCoisa {NS}><cStat>137</cStat></retOutraCoisa>", CNPJ_EMPRESA) is None
