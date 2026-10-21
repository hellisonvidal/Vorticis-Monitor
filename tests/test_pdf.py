"""Testes da geração do PDF da DANFE (monitor_notas.pdf)."""
import pytest

from monitor_notas.pdf import danfe, pode_gerar_danfe

NS = 'xmlns="http://www.portalfiscal.inf.br/nfe"'

# Chave de acesso com DV válido (módulo 11).
CHAVE = "35260312345678000195550010000001231123456782"
assert len(CHAVE) == 44

NFE_PROC = f"""<?xml version="1.0" encoding="UTF-8"?>
<nfeProc {NS} versao="4.00">
  <NFe>
    <infNFe Id="NFe{CHAVE}" versao="4.00">
      <ide>
        <cUF>35</cUF>
        <cNF>12345678</cNF>
        <natOp>VENDA DE MERCADORIA</natOp>
        <mod>55</mod>
        <serie>1</serie>
        <nNF>123</nNF>
        <dhEmi>2026-03-10T10:00:00-03:00</dhEmi>
        <dhSaiEnt>2026-03-10T10:00:00-03:00</dhSaiEnt>
        <tpNF>1</tpNF>
        <idDest>1</idDest>
        <cMunFG>3550308</cMunFG>
        <tpImp>1</tpImp>
        <tpEmis>1</tpEmis>
        <cDV>2</cDV>
        <tpAmb>2</tpAmb>
        <finNFe>1</finNFe>
        <indFinal>1</indFinal>
        <indPres>1</indPres>
        <procEmi>0</procEmi>
        <verProc>1.0</verProc>
      </ide>
      <emit>
        <CNPJ>12345678000195</CNPJ>
        <xNome>EMPRESA EXEMPLO LTDA</xNome>
        <xFant>EXEMPLO</xFant>
        <enderEmit>
          <xLgr>RUA DAS FLORES</xLgr>
          <nro>100</nro>
          <xBairro>CENTRO</xBairro>
          <cMun>3550308</cMun>
          <xMun>SAO PAULO</xMun>
          <UF>SP</UF>
          <CEP>01001000</CEP>
          <cPais>1</cPais>
          <xPais>BRASIL</xPais>
          <fone>1133334444</fone>
        </enderEmit>
        <IE>111222333444</IE>
        <CRT>3</CRT>
      </emit>
      <dest>
        <CNPJ>11222333000181</CNPJ>
        <xNome>CLIENTE EXEMPLO COMERCIO ME</xNome>
        <enderDest>
          <xLgr>AVENIDA PAULISTA</xLgr>
          <nro>1000</nro>
          <xBairro>BELA VISTA</xBairro>
          <cMun>3550308</cMun>
          <xMun>SAO PAULO</xMun>
          <UF>SP</UF>
          <CEP>01310100</CEP>
          <cPais>1</cPais>
          <xPais>BRASIL</xPais>
          <fone>1155556666</fone>
        </enderDest>
        <indIEDest>9</indIEDest>
      </dest>
      <det nItem="1">
        <prod>
          <cProd>001</cProd>
          <cEAN>SEM GTIN</cEAN>
          <xProd>CAMISETA ALGODAO TAMANHO M</xProd>
          <NCM>61091000</NCM>
          <CFOP>5102</CFOP>
          <uCom>UN</uCom>
          <qCom>2.0000</qCom>
          <vUnCom>50.0000000000</vUnCom>
          <vProd>100.00</vProd>
          <cEANTrib>SEM GTIN</cEANTrib>
          <uTrib>UN</uTrib>
          <qTrib>2.0000</qTrib>
          <vUnTrib>50.0000000000</vUnTrib>
          <indTot>1</indTot>
        </prod>
        <imposto>
          <vTotTrib>30.00</vTotTrib>
          <ICMS>
            <ICMS00>
              <orig>0</orig>
              <CST>00</CST>
              <modBC>3</modBC>
              <vBC>100.00</vBC>
              <pICMS>18.00</pICMS>
              <vICMS>18.00</vICMS>
            </ICMS00>
          </ICMS>
          <PIS>
            <PISAliq>
              <CST>01</CST>
              <vBC>100.00</vBC>
              <pPIS>1.65</pPIS>
              <vPIS>1.65</vPIS>
            </PISAliq>
          </PIS>
          <COFINS>
            <COFINSAliq>
              <CST>01</CST>
              <vBC>100.00</vBC>
              <pCOFINS>7.60</pCOFINS>
              <vCOFINS>7.60</vCOFINS>
            </COFINSAliq>
          </COFINS>
        </imposto>
      </det>
      <total>
        <ICMSTot>
          <vBC>100.00</vBC>
          <vICMS>18.00</vICMS>
          <vICMSDeson>0.00</vICMSDeson>
          <vFCP>0.00</vFCP>
          <vBCST>0.00</vBCST>
          <vST>0.00</vST>
          <vFCPST>0.00</vFCPST>
          <vFCPSTRet>0.00</vFCPSTRet>
          <vProd>100.00</vProd>
          <vFrete>0.00</vFrete>
          <vSeg>0.00</vSeg>
          <vDesc>0.00</vDesc>
          <vII>0.00</vII>
          <vIPI>0.00</vIPI>
          <vIPIDevol>0.00</vIPIDevol>
          <vPIS>1.65</vPIS>
          <vCOFINS>7.60</vCOFINS>
          <vOutro>0.00</vOutro>
          <vNF>100.00</vNF>
        </ICMSTot>
      </total>
      <transp>
        <modFrete>9</modFrete>
      </transp>
      <pag>
        <detPag>
          <tPag>01</tPag>
          <vPag>100.00</vPag>
        </detPag>
      </pag>
      <infAdic>
        <infCpl>Documento de teste gerado para validacao do monitor de notas.</infCpl>
      </infAdic>
    </infNFe>
  </NFe>
  <protNFe versao="4.00">
    <infProt>
      <tpAmb>2</tpAmb>
      <verAplic>SP_NFE_PA_1.0</verAplic>
      <chNFe>{CHAVE}</chNFe>
      <dhRecbto>2026-03-10T10:00:05-03:00</dhRecbto>
      <nProt>135260000000001</nProt>
      <digVal>abcdefghijklmnopqrstuvwxyz0=</digVal>
      <cStat>100</cStat>
      <xMotivo>Autorizado o uso da NF-e</xMotivo>
    </infProt>
  </protNFe>
</nfeProc>
"""

RES_NFE = f"""<?xml version="1.0" encoding="UTF-8"?>
<resNFe {NS}>
  <chNFe>{CHAVE}</chNFe>
  <CNPJ>11222333000181</CNPJ>
  <xNome>FORNECEDOR LTDA</xNome>
  <dhEmi>2026-03-10T10:00:00-03:00</dhEmi>
  <vNF>100.00</vNF>
</resNFe>
"""


def test_gera_pdf_valido(tmp_path):
    destino = tmp_path / "danfe.pdf"

    caminho = danfe(NFE_PROC, destino)

    assert caminho == destino
    conteudo = destino.read_bytes()
    assert conteudo.startswith(b"%PDF")
    assert b"%%EOF" in conteudo[-64:]
    assert len(conteudo) > 2_000


def test_gera_pdf_layout_nt_2026_010(tmp_path):
    # NF-e emitida a partir de 2026-12-01 usa o layout NT 2026.010.
    xml = NFE_PROC.replace("2026-03-10T10:00:00-03:00", "2027-01-15T10:00:00-03:00")
    destino = tmp_path / "danfe_nt.pdf"

    danfe(xml, destino)

    assert destino.read_bytes().startswith(b"%PDF")


def test_aceita_destino_str(tmp_path):
    destino = tmp_path / "danfe2.pdf"

    caminho = danfe(NFE_PROC, str(destino))

    assert caminho == destino
    assert destino.read_bytes().startswith(b"%PDF")


def test_pode_gerar_para_nfe_proc():
    assert pode_gerar_danfe(NFE_PROC) is True


def test_nao_pode_gerar_para_res_nfe():
    assert pode_gerar_danfe(RES_NFE) is False


def test_pode_gerar_para_xml_invalido():
    assert pode_gerar_danfe("isto nao e xml") is False


def test_danfe_levanta_erro_para_res_nfe(tmp_path):
    destino = tmp_path / "nao_gerar.pdf"

    with pytest.raises(ValueError, match="nfeProc"):
        danfe(RES_NFE, destino)

    assert not destino.exists()
