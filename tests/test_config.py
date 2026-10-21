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


def test_dados_do_certificado_icp_brasil():
    from datetime import datetime, timedelta, timezone

    from cryptography import x509
    from cryptography.hazmat.primitives import hashes
    from cryptography.hazmat.primitives.asymmetric import rsa
    from cryptography.x509.oid import NameOID

    from monitor_notas.certificado import OID_CNPJ, ler_dados

    chave = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    agora = datetime.now(timezone.utc)

    def gerar(cn, san=None):
        nome = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, cn),
                          x509.NameAttribute(NameOID.STATE_OR_PROVINCE_NAME, "mg")])
        b = (x509.CertificateBuilder().subject_name(nome).issuer_name(nome)
             .public_key(chave.public_key()).serial_number(1)
             .not_valid_before(agora).not_valid_after(agora + timedelta(days=1)))
        if san:
            b = b.add_extension(x509.SubjectAlternativeName(san), False)
        return b.sign(chave, hashes.SHA256())

    d = ler_dados(gerar("ACME COMERCIO LTDA:12345678000190"))
    assert (d.razao_social, d.cnpj, d.uf) == ("ACME COMERCIO LTDA", "12345678000190", "MG")
    # CNPJ só no SAN (OID ICP-Brasil 2.16.76.1.3.3, valor DER PrintableString)
    d = ler_dados(gerar("ACME", [x509.OtherName(OID_CNPJ, b"\x13\x0e98765432000110")]))
    assert d.cnpj == "98765432000110"
