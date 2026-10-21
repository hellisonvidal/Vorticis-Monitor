"""Testes da interpretação da resposta JSON do ADN (NFS-e Padrão Nacional)."""
import base64
import gzip


from monitor_notas.nfse.adn import NENHUM, interpretar

CHAVE_1 = "12345678000195" + "2026" + "0" * 30
CHAVE_2 = "11222333000181" + "2026" + "1" * 30

XML_1 = "<NFSe xmlns='http://www.sped.fazenda.gov.br/nfse'><infNFSe Id='NFS1'/></NFSe>"
XML_2 = "<NFSe xmlns='http://www.sped.fazenda.gov.br/nfse'><infNFSe Id='NFS2'/></NFSe>"


def _b64gz(texto: str) -> str:
    return base64.b64encode(gzip.compress(texto.encode("utf-8"))).decode()


def test_lote_com_documentos_pascal_case():
    dados = {
        "StatusProcessamento": "DOCUMENTOS_LOCALIZADOS",
        "LoteDFe": [
            {"NSU": 10, "ChaveAcesso": CHAVE_1, "TipoDocumento": "NFSE",
             "TipoEvento": None, "ArquivoXml": _b64gz(XML_1)},
            {"NSU": 11, "ChaveAcesso": CHAVE_2, "TipoDocumento": "EVENTO",
             "TipoEvento": "e105102", "ArquivoXml": _b64gz(XML_2)},
        ],
        "Erros": [],
    }
    ret = interpretar(200, dados)

    assert ret.http == 200
    assert ret.status == "DOCUMENTOS_LOCALIZADOS"
    assert [i.nsu for i in ret.itens] == [10, 11]
    assert ret.itens[0].chave == CHAVE_1
    assert ret.itens[0].tipo_documento == "NFSE"
    assert ret.itens[0].tipo_evento is None
    assert ret.itens[0].xml == XML_1
    assert ret.itens[1].tipo_documento == "EVENTO"
    assert ret.itens[1].tipo_evento == "e105102"
    assert ret.itens[1].xml == XML_2
    assert ret.maior_nsu == 11
    assert ret.fim is False
    assert ret.erro is False


def test_variante_camel_case_dos_campos():
    # Mesmos campos em camelCase (exceto NSU, que segue a grafia do leiaute)
    dados = {
        "statusProcessamento": "DOCUMENTOS_LOCALIZADOS",
        "loteDFe": [
            {"NSU": 20, "chaveAcesso": CHAVE_1, "tipoDocumento": "NFSE",
             "arquivoXml": _b64gz(XML_1)},
        ],
    }
    ret = interpretar(200, dados)
    assert ret.status == "DOCUMENTOS_LOCALIZADOS"
    assert len(ret.itens) == 1
    assert ret.itens[0].chave == CHAVE_1
    assert ret.itens[0].tipo_documento == "NFSE"
    assert ret.itens[0].xml == XML_1
    assert ret.maior_nsu == 20


def test_variante_camel_case_com_nsu_minusculo():
    dados = {
        "statusProcessamento": "DOCUMENTOS_LOCALIZADOS",
        "loteDFe": [{"nsu": 30, "chaveAcesso": CHAVE_1, "arquivoXml": _b64gz(XML_1)}],
    }
    ret = interpretar(200, dados)
    assert ret.itens[0].nsu == 30


def test_nenhum_documento_localizado_marca_fim():
    dados = {"StatusProcessamento": NENHUM, "LoteDFe": []}
    ret = interpretar(200, dados)
    assert ret.status == NENHUM
    assert ret.itens == []
    assert ret.fim is True
    assert ret.erro is False
    assert ret.maior_nsu is None


def test_item_sem_arquivo_xml_e_ignorado():
    dados = {
        "StatusProcessamento": "DOCUMENTOS_LOCALIZADOS",
        "LoteDFe": [
            {"NSU": 5, "ChaveAcesso": CHAVE_1, "ArquivoXml": None},
            {"NSU": 6, "ChaveAcesso": CHAVE_2, "ArquivoXml": _b64gz(XML_2)},
        ],
    }
    ret = interpretar(200, dados)
    assert [i.nsu for i in ret.itens] == [6]


def test_erros_marcam_erro_e_fim_sem_itens():
    dados = {
        "StatusProcessamento": "REJEICAO",
        "LoteDFe": [],
        "Erros": [{"Codigo": "E2243", "Descricao": "Certificado sem CNPJ autorizado"}],
    }
    ret = interpretar(400, dados)
    assert ret.http == 400
    assert ret.erro is True
    assert ret.fim is True
    assert ret.erros == [{"Codigo": "E2243", "Descricao": "Certificado sem CNPJ autorizado"}]


def test_erros_com_documentos_localizados_ainda_sao_erro():
    dados = {
        "StatusProcessamento": "DOCUMENTOS_LOCALIZADOS",
        "LoteDFe": [{"NSU": 1, "ChaveAcesso": CHAVE_1, "ArquivoXml": _b64gz(XML_1)}],
        "Erros": [{"Codigo": "E2243"}],
    }
    ret = interpretar(400, dados)
    assert ret.erro is True
    assert len(ret.itens) == 1


def test_nenhum_documento_com_erros_nao_e_erro():
    dados = {"StatusProcessamento": NENHUM, "LoteDFe": [], "Erros": [{"Codigo": "X"}]}
    ret = interpretar(400, dados)
    assert ret.erro is False
    assert ret.fim is True
