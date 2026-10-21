"""Testes da persistência SQLite e da cópia dos XMLs em disco."""
import pytest

from monitor_notas.armazenamento import Armazenamento, Documento

CNPJ = "12345678000195"
OUTRO = "11222333000181"
CHAVE_A = "A" * 44
CHAVE_B = "B" * 44


@pytest.fixture
def db(tmp_path):
    arm = Armazenamento(str(tmp_path / "dados" / "monitor.db"), str(tmp_path / "xml"))
    yield arm
    arm.con.close()


def _doc(chave=CHAVE_A, tipo="completo", cnpj=CNPJ, direcao="recebida", evento="",
         data="2026-03-10T10:00:00-03:00", xml="<nfeProc/>"):
    return Documento(
        origem="nfe", tipo=tipo, direcao=direcao, cnpj_empresa=cnpj, chave=chave,
        xml=xml, nsu="1", emitente_cnpj=OUTRO, emitente_nome="FORN", valor=10.5,
        data_emissao=data, evento=evento,
    )


def test_salvar_retorna_true_e_depois_false_para_duplicado(db):
    assert db.salvar(_doc()) is True
    assert db.salvar(_doc()) is False
    linhas = db.listar(CNPJ)
    assert len(linhas) == 1


def test_mesma_chave_com_tipo_diferente_nao_e_duplicado(db):
    assert db.salvar(_doc(tipo="resumo", xml="<resNFe/>")) is True
    assert db.salvar(_doc(tipo="completo")) is True
    assert len(db.listar(CNPJ)) == 2


def test_eventos_com_codigos_diferentes_sao_distintos(db):
    assert db.salvar(_doc(tipo="evento", evento="110111", xml="<e1/>")) is True
    assert db.salvar(_doc(tipo="evento", evento="110111", xml="<e1/>")) is False
    assert db.salvar(_doc(tipo="evento", evento="110110", xml="<e2/>")) is True


def test_arquivo_xml_gravado_na_pasta_esperada(db, tmp_path):
    xml = "<nfeProc>conteudo</nfeProc>"
    db.salvar(_doc(xml=xml, data="2026-03-10T10:00:00-03:00"))
    caminho = tmp_path / "xml" / CNPJ / "nfe" / "recebida" / "2026-03" / f"{CHAVE_A}.xml"
    assert caminho.is_file()
    assert caminho.read_text(encoding="utf-8") == xml


def test_arquivo_de_resumo_e_evento_tem_sufixo(db, tmp_path):
    db.salvar(_doc(tipo="resumo", xml="<resNFe/>"))
    db.salvar(_doc(tipo="evento", evento="110111", xml="<ev/>"))
    pasta = tmp_path / "xml" / CNPJ / "nfe" / "recebida" / "2026-03"
    assert (pasta / f"{CHAVE_A}-resumo.xml").read_text(encoding="utf-8") == "<resNFe/>"
    assert (pasta / f"{CHAVE_A}-evento110111.xml").read_text(encoding="utf-8") == "<ev/>"


def test_duplicado_nao_regrava_arquivo(db, tmp_path):
    db.salvar(_doc(xml="<v1/>"))
    db.salvar(_doc(xml="<v2/>"))
    caminho = tmp_path / "xml" / CNPJ / "nfe" / "recebida" / "2026-03" / f"{CHAVE_A}.xml"
    assert caminho.read_text(encoding="utf-8") == "<v1/>"


def test_sem_data_de_emissao_usa_pasta_sem_data(db, tmp_path):
    db.salvar(_doc(data=None))
    assert (tmp_path / "xml" / CNPJ / "nfe" / "recebida" / "sem-data" / f"{CHAVE_A}.xml").is_file()


def test_estado_get_set(db):
    assert db.obter_estado("nfe:x:ult_nsu") is None
    assert db.obter_estado("nfe:x:ult_nsu", "0") == "0"
    db.gravar_estado("nfe:x:ult_nsu", "10")
    assert db.obter_estado("nfe:x:ult_nsu") == "10"
    db.gravar_estado("nfe:x:ult_nsu", "25")
    assert db.obter_estado("nfe:x:ult_nsu", "0") == "25"


def test_estado_persiste_entre_conexoes(tmp_path):
    banco = str(tmp_path / "m.db")
    a = Armazenamento(banco, str(tmp_path / "xml"))
    a.gravar_estado("k", "v")
    a.con.close()
    b = Armazenamento(banco, str(tmp_path / "xml"))
    assert b.obter_estado("k") == "v"
    b.con.close()


def test_tem_completo(db):
    db.salvar(_doc(tipo="resumo", chave=CHAVE_A, xml="<r/>"))
    assert db.tem_completo("nfe", CHAVE_A) is False
    db.salvar(_doc(tipo="completo", chave=CHAVE_A))
    assert db.tem_completo("nfe", CHAVE_A) is True
    assert db.tem_completo("nfse", CHAVE_A) is False


def test_resumos_sem_completo(db):
    db.salvar(_doc(tipo="resumo", chave=CHAVE_A, xml="<a/>"))
    db.salvar(_doc(tipo="resumo", chave=CHAVE_B, xml="<b/>"))
    db.salvar(_doc(tipo="completo", chave=CHAVE_B))
    # resumo de outra empresa não entra na lista
    db.salvar(_doc(tipo="resumo", chave="C" * 44, cnpj=OUTRO, xml="<c/>"))

    assert db.resumos_sem_completo(CNPJ) == [CHAVE_A]
    assert db.resumos_sem_completo(OUTRO) == ["C" * 44]

    db.salvar(_doc(tipo="completo", chave=CHAVE_A))
    assert db.resumos_sem_completo(CNPJ) == []


def test_falha_ao_gravar_arquivo_desfaz_insert(tmp_path, monkeypatch):
    from monitor_notas.armazenamento import Armazenamento, Documento
    db = Armazenamento(str(tmp_path / "m.db"), str(tmp_path / "xml"))
    doc = Documento(origem="nfe", tipo="completo", direcao="recebida",
                    cnpj_empresa="12345678000190", chave="1" * 44, xml="<x/>")

    def falha(_):
        raise OSError("disco cheio")
    monkeypatch.setattr(db, "_gravar_arquivo", falha)
    import pytest
    with pytest.raises(OSError):
        db.salvar(doc)
    monkeypatch.undo()
    assert db.salvar(doc) is True
