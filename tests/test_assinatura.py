"""Assinatura do evento de manifestação, verificada de forma independente."""
import base64
import hashlib
from datetime import datetime, timedelta, timezone

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding, rsa
from cryptography.x509.oid import NameOID
from lxml import etree

from monitor_notas.certificado import Certificado
from monitor_notas.nfe import manifestacao

CHAVE = "35261012345678000190550010000001231000001234"
CNPJ = "12345678000190"
NFE = "http://www.portalfiscal.inf.br/nfe"
DS = "http://www.w3.org/2000/09/xmldsig#"


@pytest.fixture(scope="module")
def cert():
    chave = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    nome = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, f"EMPRESA TESTE:{CNPJ}")])
    agora = datetime.now(timezone.utc)
    c = (x509.CertificateBuilder().subject_name(nome).issuer_name(nome)
         .public_key(chave.public_key()).serial_number(1)
         .not_valid_before(agora - timedelta(days=1)).not_valid_after(agora + timedelta(days=30))
         .sign(chave, hashes.SHA256()))
    return Certificado(chave=chave, cert=c)


def _verificar(xml: bytes, cert: Certificado) -> etree._Element:
    """Refaz o que o validador da SEFAZ faz: digest do elemento referenciado + RSA sobre SignedInfo.

    Usa apenas o certificado embutido no XML. Retorna o elemento assinado.
    """
    evento = etree.fromstring(xml)
    sig = evento.find(f"{{{DS}}}Signature")
    ref = sig.find(f".//{{{DS}}}Reference")
    alvo = evento.find(f".//*[@Id='{ref.get('URI')[1:]}']")
    digest = base64.b64encode(hashlib.sha1(etree.tostring(alvo, method="c14n")).digest()).decode()
    if digest != ref.find(f"{{{DS}}}DigestValue").text:
        raise ValueError("DigestValue não confere")
    der = base64.b64decode(sig.find(f".//{{{DS}}}X509Certificate").text)
    assert der == cert.cert.public_bytes(serialization.Encoding.DER)
    x509.load_der_x509_certificate(der).public_key().verify(
        base64.b64decode(sig.find(f"{{{DS}}}SignatureValue").text),
        etree.tostring(sig.find(f"{{{DS}}}SignedInfo"), method="c14n"),
        padding.PKCS1v15(), hashes.SHA1())
    return alvo


def test_evento_assinado_e_verificavel_apos_serializar(cert):
    env = manifestacao.montar_evento(2, CNPJ, CHAVE, manifestacao.CIENCIA, cert)
    # simula o que o servidor recebe: o XML serializado dentro do envelope SOAP
    xml = etree.tostring(env)
    evento = etree.fromstring(xml).find(f"{{{NFE}}}evento")
    alvo = _verificar(etree.tostring(evento), cert)
    assert etree.QName(alvo).localname == "infEvento"
    assert alvo.get("Id") == f"ID210210{CHAVE}01"


def test_alteracao_invalida_assinatura(cert):
    env = manifestacao.montar_evento(2, CNPJ, CHAVE, manifestacao.CIENCIA, cert)
    env.find(f".//{{{NFE}}}tpAmb").text = "1"
    evento = env.find(f"{{{NFE}}}evento")
    with pytest.raises(Exception):
        _verificar(etree.tostring(evento), cert)


def test_campos_do_evento(cert):
    agora = datetime(2026, 10, 21, 10, 30, 0, tzinfo=timezone(timedelta(hours=-3)))
    env = manifestacao.montar_evento(1, CNPJ, CHAVE, manifestacao.CIENCIA, cert, agora=agora)
    t = lambda tag: env.find(f".//{{{NFE}}}{tag}").text  # noqa: E731
    assert t("cOrgao") == "91"
    assert t("dhEvento") == "2026-10-21T10:30:00-03:00"
    assert t("descEvento") == "Ciencia da Operacao"
    assert len(t("idLote")) == 15
    # assinatura deve ser filha de <evento>, depois de <infEvento>
    filhos = [etree.QName(e).localname for e in env.find(f"{{{NFE}}}evento")]
    assert filhos == ["infEvento", "Signature"]


def test_nao_realizada_exige_justificativa(cert):
    with pytest.raises(ValueError):
        manifestacao.montar_evento(2, CNPJ, CHAVE, manifestacao.NAO_REALIZADA, cert)
