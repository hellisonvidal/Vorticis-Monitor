"""Testes dos ciclos de captura (NF-e e NFS-e) com os clientes de rede substituídos por mocks."""
from datetime import datetime, timedelta, timezone
from unittest.mock import MagicMock, call, patch

import pytest

from monitor_notas.armazenamento import Armazenamento
from monitor_notas.certificado import Certificado
from monitor_notas.config import Config, Empresa
from monitor_notas.nfe import monitor as nfe_monitor
from monitor_notas.nfe.distdfe import DocZip, RetornoDist
from monitor_notas.nfse import monitor as nfse_monitor
from monitor_notas.nfse.adn import ItemDFe, RetornoADN

CNPJ = "12345678000195"
OUTRO = "11222333000181"


def _chave(n: int) -> str:
    return f"{n:044d}"


def _res_nfe(chave: str) -> str:
    return f"""<resNFe xmlns="http://www.portalfiscal.inf.br/nfe">
  <chNFe>{chave}</chNFe><CNPJ>{OUTRO}</CNPJ><xNome>FORN</xNome>
  <dhEmi>2026-03-01T08:00:00-03:00</dhEmi><vNF>10.00</vNF>
</resNFe>"""


def _nfse(chave: str) -> str:
    return f"""<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infNFSe Id="NFS{chave}">
    <dhEmi>2026-04-01T14:30:00-03:00</dhEmi>
    <emit><CNPJ>{OUTRO}</CNPJ><xNome>PREST</xNome></emit>
    <toma><CNPJ>{CNPJ}</CNPJ></toma>
    <valores><vServPrest><vServ>99.90</vServ></vServPrest></valores>
  </infNFSe>
</NFSe>"""


@pytest.fixture
def empresa():
    return Empresa(nome="Empresa Teste", cnpj=CNPJ, uf="SP",
                   certificado="x.pfx", senha_armazenada="segredo")


@pytest.fixture
def cfg(tmp_path, empresa):
    return Config(
        ambiente="homologacao",
        banco=str(tmp_path / "dados" / "monitor.db"),
        pasta_xml=str(tmp_path / "xml"),
        intervalo_minutos=60,
        verificar_tls=True,
        empresas=[empresa],
    )


@pytest.fixture
def cert():
    return MagicMock(spec=Certificado)


@pytest.fixture
def db(cfg):
    arm = Armazenamento(cfg.banco, cfg.pasta_xml)
    yield arm
    arm.con.close()


def _proxima_em_aproximadamente_uma_hora(db, chave_estado: str) -> bool:
    valor = db.obter_estado(chave_estado)
    assert valor is not None, f"{chave_estado} não foi gravado"
    proxima = datetime.fromisoformat(valor)
    agora = datetime.now(timezone.utc)
    return agora + timedelta(minutes=55) <= proxima <= agora + timedelta(minutes=65)


# ---------------------------------------------------------------- NF-e

def test_nfe_dois_lotes_e_fim(cfg, empresa, cert, db):
    lote1 = RetornoDist(
        cstat="138", motivo="Documento localizado",
        ult_nsu="000000000000010", max_nsu="000000000000030",
        docs=[
            DocZip("000000000000001", "resNFe_v1.01.xsd", _res_nfe(_chave(1))),
            DocZip("000000000000010", "resNFe_v1.01.xsd", _res_nfe(_chave(2))),
        ],
    )
    lote2 = RetornoDist(
        cstat="138", motivo="Documento localizado",
        ult_nsu="000000000000020", max_nsu="000000000000030",
        docs=[DocZip("000000000000020", "resNFe_v1.01.xsd", _res_nfe(_chave(3)))],
    )
    fim = RetornoDist(
        cstat="137", motivo="Nenhum documento localizado",
        ult_nsu="000000000000030", max_nsu="000000000000030", docs=[],
    )

    with patch.object(nfe_monitor, "ClienteDistDFe") as cls:
        cls.return_value.por_nsu.side_effect = [lote1, lote2, fim]

        novos = nfe_monitor.executar(cfg, empresa, cert, db)

        assert novos == 3
        assert cls.return_value.por_nsu.call_args_list == [call("0"), call("000000000000010"),
                                                          call("000000000000020")]
        # ult_nsu gravado com o último valor devolvido pela SEFAZ
        assert db.obter_estado(f"nfe:{CNPJ}:ult_nsu") == "000000000000030"
        # após o fim, a próxima consulta é agendada ~1h à frente
        assert _proxima_em_aproximadamente_uma_hora(db, f"nfe:{CNPJ}:proxima")

        # segunda execução imediata: não consulta a SEFAZ
        cls.reset_mock()
        assert nfe_monitor.executar(cfg, empresa, cert, db) == 0
        cls.assert_not_called()


def test_nfe_consumo_indevido_656_agenda_espera(cfg, empresa, cert, db):
    rejeicao = RetornoDist(
        cstat="656", motivo="Consumo indevido",
        ult_nsu="0", max_nsu="0", docs=[],
    )
    with patch.object(nfe_monitor, "ClienteDistDFe") as cls:
        cls.return_value.por_nsu.return_value = rejeicao

        assert nfe_monitor.executar(cfg, empresa, cert, db) == 0

        assert cls.return_value.por_nsu.call_count == 1
        assert _proxima_em_aproximadamente_uma_hora(db, f"nfe:{CNPJ}:proxima")
        # 656 não avança o ult_nsu
        assert db.obter_estado(f"nfe:{CNPJ}:ult_nsu") is None

        # enquanto a espera não passa, nada é consultado
        cls.reset_mock()
        assert nfe_monitor.executar(cfg, empresa, cert, db) == 0
        cls.assert_not_called()


def test_nfe_retomada_usa_ult_nsu_salvo(cfg, empresa, cert, db):
    db.gravar_estado(f"nfe:{CNPJ}:ult_nsu", "000000000000005")
    vazio = RetornoDist(cstat="137", motivo="Nenhum", ult_nsu="000000000000005",
                        max_nsu="000000000000005", docs=[])
    with patch.object(nfe_monitor, "ClienteDistDFe") as cls:
        cls.return_value.por_nsu.return_value = vazio
        nfe_monitor.executar(cfg, empresa, cert, db)
        cls.return_value.por_nsu.assert_called_once_with("000000000000005")


# ---------------------------------------------------------------- NFS-e

def test_nfse_dois_lotes_e_fim(cfg, empresa, cert, db):
    lote1 = RetornoADN(
        status="DOCUMENTOS_LOCALIZADOS", http=200,
        itens=[
            ItemDFe(nsu=1, chave=None, tipo_documento="NFSE", tipo_evento=None,
                    xml=_nfse("1" * 50)),
            ItemDFe(nsu=2, chave=None, tipo_documento="NFSE", tipo_evento=None,
                    xml=_nfse("2" * 50)),
        ],
    )
    lote2 = RetornoADN(
        status="DOCUMENTOS_LOCALIZADOS", http=200,
        itens=[ItemDFe(nsu=3, chave=None, tipo_documento="NFSE", tipo_evento=None,
                       xml=_nfse("3" * 50))],
    )
    fim = RetornoADN(status="NENHUM_DOCUMENTO_LOCALIZADO", http=200, itens=[])

    with patch.object(nfse_monitor, "ClienteADN") as cls:
        cls.return_value.por_nsu.side_effect = [lote1, lote2, fim]

        novos = nfse_monitor.executar(cfg, empresa, cert, db)

        assert novos == 3
        assert cls.return_value.por_nsu.call_args_list == [call(0), call(2), call(3)]
        assert db.obter_estado(f"nfse:{CNPJ}:ult_nsu") == "3"
        assert _proxima_em_aproximadamente_uma_hora(db, f"nfse:{CNPJ}:proxima")

        cls.reset_mock()
        assert nfse_monitor.executar(cfg, empresa, cert, db) == 0
        cls.assert_not_called()


def test_nfse_documentos_gravados_como_completos(cfg, empresa, cert, db):
    lote = RetornoADN(
        status="DOCUMENTOS_LOCALIZADOS", http=200,
        itens=[ItemDFe(nsu=7, chave=None, tipo_documento="NFSE", tipo_evento=None,
                       xml=_nfse("7" * 50))],
    )
    fim = RetornoADN(status="NENHUM_DOCUMENTO_LOCALIZADO", http=200, itens=[])
    with patch.object(nfse_monitor, "ClienteADN") as cls:
        cls.return_value.por_nsu.side_effect = [lote, fim]
        nfse_monitor.executar(cfg, empresa, cert, db)

    linhas = db.listar(CNPJ)
    assert len(linhas) == 1
    assert linhas[0]["origem"] == "nfse"
    assert linhas[0]["direcao"] == "recebida"
    assert linhas[0]["chave"] == "7" * 50
    assert linhas[0]["nsu"] == "7"
