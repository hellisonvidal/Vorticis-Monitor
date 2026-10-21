"""Testes do parser de NFS-e Padrão Nacional (emitida x recebida)."""
from monitor_notas.nfse.parser import de_xml

NS = 'xmlns="http://www.sped.fazenda.gov.br/nfse"'

CNPJ_EMPRESA = "12345678000195"
CNPJ_TOMADOR = "11222333000181"
CHAVE = "1" * 50  # chave de acesso da NFS-e: 50 dígitos


def _nfse(cnpj_emit: str) -> str:
    return f"""<NFSe {NS} versao="1.00">
  <infNFSe Id="NFS{CHAVE}">
    <nNFSe>123</nNFSe>
    <dhEmi>2026-04-01T14:30:00-03:00</dhEmi>
    <emit>
      <CNPJ>{cnpj_emit}</CNPJ>
      <xNome>PRESTADORA LTDA</xNome>
    </emit>
    <toma>
      <CNPJ>{CNPJ_TOMADOR}</CNPJ>
      <xNome>TOMADOR SA</xNome>
    </toma>
    <valores>
      <vServPrest>
        <vServ>250.00</vServ>
      </vServPrest>
    </valores>
  </infNFSe>
</NFSe>"""


def test_nfse_emitida_quando_emitente_e_a_empresa():
    doc = de_xml(_nfse(CNPJ_EMPRESA), CNPJ_EMPRESA, nsu=42, chave=None)
    assert doc is not None
    assert doc.origem == "nfse"
    assert doc.tipo == "completo"
    assert doc.direcao == "emitida"
    assert doc.chave == CHAVE
    assert doc.nsu == "42"
    assert doc.emitente_cnpj == CNPJ_EMPRESA
    assert doc.emitente_nome == "PRESTADORA LTDA"
    assert doc.destinatario_cnpj == CNPJ_TOMADOR
    assert doc.valor == 250.0
    assert doc.data_emissao == "2026-04-01T14:30:00-03:00"


def test_nfse_recebida_quando_empresa_e_tomadora():
    doc = de_xml(_nfse(CNPJ_TOMADOR), CNPJ_EMPRESA, nsu=None, chave=None)
    assert doc.direcao == "recebida"
    assert doc.emitente_cnpj == CNPJ_TOMADOR
    assert doc.destinatario_cnpj == CNPJ_TOMADOR
    assert doc.nsu is None
    assert doc.chave == CHAVE


def test_chave_informada_tem_prioridade_sobre_id():
    doc = de_xml(_nfse(CNPJ_TOMADOR), CNPJ_EMPRESA, nsu=1, chave="9" * 50)
    assert doc.chave == "9" * 50


def test_raiz_desconhecida_retorna_none():
    assert de_xml(f"<outro {NS}/>", CNPJ_EMPRESA, nsu=1, chave=None) is None
