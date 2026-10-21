import pytest

from monitor_notas import caminhos, config, segredo


def test_salvar_e_carregar_ida_e_volta(tmp_path):
    arq = tmp_path / "config.toml"
    cfg = config.Config(ambiente="homologacao", intervalo_minutos=90, empresas=[
        config.Empresa(nome='Empresa "A" \\ Ltda', cnpj="12345678000190", uf="sp",
                       certificado="certificados/a.pfx", senha_armazenada="env:SENHA_A",
                       consultar_nfse=False),
    ])
    config.salvar(cfg, arq)
    lido = config.carregar(arq)
    assert lido.ambiente == "homologacao" and lido.tp_amb == 2
    assert lido.intervalo_minutos == 90
    e = lido.empresas[0]
    assert e.nome == 'Empresa "A" \\ Ltda'
    assert e.uf == "SP" and e.cuf == 35
    assert e.senha_armazenada == "env:SENHA_A"
    assert e.consultar_nfe is True and e.consultar_nfse is False


def test_intervalo_minimo_60(tmp_path):
    arq = tmp_path / "config.toml"
    arq.write_text("[geral]\nintervalo_minutos = 5\n", encoding="utf-8")
    assert config.carregar(arq).intervalo_minutos == 60


def test_config_ausente_retorna_padrao(tmp_path):
    cfg = config.carregar(tmp_path / "nao_existe.toml")
    assert cfg.empresas == [] and cfg.ambiente == "producao"


def test_cnpj_invalido(tmp_path):
    arq = tmp_path / "config.toml"
    arq.write_text('[[empresa]]\ncnpj = "123"\nuf = "SP"\ncertificado = "x"\n', encoding="utf-8")
    with pytest.raises(ValueError):
        config.carregar(arq)


def test_caminhos_relativos_a_pasta_base(tmp_path):
    assert caminhos.resolver("dados/x.db") == caminhos.pasta_base() / "dados/x.db"
    assert caminhos.resolver(tmp_path) == tmp_path


def test_senha_env_e_texto(monkeypatch):
    monkeypatch.setenv("SENHA_TESTE", "abc")
    assert segredo.revelar("env:SENHA_TESTE") == "abc"
    assert segredo.revelar("puro") == "puro"
    monkeypatch.delenv("SENHA_TESTE")
    with pytest.raises(ValueError):
        segredo.revelar("env:SENHA_TESTE")


@pytest.mark.skipif(not segredo.disponivel(), reason="DPAPI só no Windows")
def test_dpapi_ida_e_volta():
    protegido = segredo.proteger("minha senha ç")
    assert protegido.startswith("dpapi:")
    assert segredo.revelar(protegido) == "minha senha ç"


def test_trava_impede_consulta_simultanea(tmp_path, monkeypatch):
    from monitor_notas import servico
    monkeypatch.setattr(servico, "resolver", lambda p: tmp_path / p)
    with servico._trava_consulta():
        with pytest.raises(servico.ConsultaEmAndamento):
            with servico._trava_consulta():
                pass
    with servico._trava_consulta():  # liberada após sair
        pass
