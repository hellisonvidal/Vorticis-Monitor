"""Integração com o repositório de certificados do Windows + WinHTTP (só roda no Windows)."""
import hashlib
import ssl
import subprocess
import sys
import threading
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, HTTPServer

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives.serialization import pkcs12
from cryptography.x509.oid import NameOID

pytestmark = pytest.mark.skipif(sys.platform != "win32", reason="somente Windows")

CNPJ = "12345678000190"


def _cert(cn, chave, ca=None, uso_cliente=False):
    """`ca` = (cert, chave) emissora. Sem `ca`, autoassinado.

    O certificado do cliente NÃO pode ser autoassinado: o certutil tentaria instalá-lo como
    raiz confiável e abriria uma confirmação (invisível no CI), travando o teste."""
    agora = datetime.now(timezone.utc)
    nome = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, cn),
                      x509.NameAttribute(NameOID.STATE_OR_PROVINCE_NAME, "SP")])
    emissor, chave_assina = (ca[0].subject, ca[1]) if ca else (nome, chave)
    b = (x509.CertificateBuilder().subject_name(nome).issuer_name(emissor)
         .public_key(chave.public_key()).serial_number(x509.random_serial_number())
         .not_valid_before(agora - timedelta(days=1)).not_valid_after(agora + timedelta(days=30)))
    if uso_cliente:
        b = b.add_extension(x509.ExtendedKeyUsage([x509.oid.ExtendedKeyUsageOID.CLIENT_AUTH]), False)
    else:
        b = b.add_extension(x509.SubjectAlternativeName([x509.DNSName("localhost")]), False)
    return b.sign(chave_assina, hashes.SHA256())


@pytest.fixture(scope="module")
def ambiente(tmp_path_factory):
    """AC de teste confiável na máquina (como a ICP-Brasil), certificado do cliente emitido
    por ela e instalado no repositório Pessoal do usuário."""
    pasta = tmp_path_factory.mktemp("cert")
    chave_ca = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    ca = _cert("AC TESTE VORTICIS", chave_ca)
    ca_der = pasta / "ca.cer"
    ca_der.write_bytes(ca.public_bytes(serialization.Encoding.DER))
    tp_ca = ca.fingerprint(hashes.SHA1()).hex().upper()
    # raízes da máquina (runner é administrador; não abre confirmação como a raiz do usuário)
    subprocess.run(["certutil", "-f", "-addstore", "Root", str(ca_der)],
                   check=True, capture_output=True, timeout=60)

    chave = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    cert = _cert(f"EMPRESA TESTE VORTICIS LTDA:{CNPJ}", chave, ca=(ca, chave_ca), uso_cliente=True)
    pfx = pasta / "cliente.pfx"
    pfx.write_bytes(pkcs12.serialize_key_and_certificates(
        b"cliente", chave, cert, None, serialization.BestAvailableEncryption(b"1234")))
    tp = cert.fingerprint(hashes.SHA1()).hex().upper()
    subprocess.run(["certutil", "-f", "-user", "-p", "1234", "-importpfx", "My", str(pfx),
                    "NoRoot"], check=True, capture_output=True, timeout=60)
    (pasta / "ca.pem").write_bytes(ca.public_bytes(serialization.Encoding.PEM))
    yield tp, pasta, (ca, chave_ca)
    subprocess.run(["certutil", "-user", "-delstore", "My", tp], capture_output=True, timeout=60)
    subprocess.run(["certutil", "-delstore", "Root", tp_ca], capture_output=True, timeout=60)


def test_lista_e_le_certificado(ambiente):
    from monitor_notas import servico, winstore
    from monitor_notas.certificado import Certificado
    tp = ambiente[0]
    assert tp in {winstore.impressao_digital(d) for d in winstore.listar_der()}
    dados = {d.thumbprint: d for d in servico.certificados_windows()}[tp]
    assert dados.cnpj == CNPJ
    assert dados.razao_social == "EMPRESA TESTE VORTICIS LTDA"
    assert dados.uf == "SP"
    assert Certificado.do_windows(tp).dados.cnpj == CNPJ


class _Eco(BaseHTTPRequestHandler):
    """Devolve a impressão digital do certificado apresentado pelo cliente, o método e o corpo."""

    def _responder(self):
        der = self.connection.getpeercert(binary_form=True) or b""
        n = int(self.headers.get("Content-Length") or 0)
        corpo = self.rfile.read(n) if n else b""
        resp = (hashlib.sha1(der).hexdigest().upper() + "|" + self.command + "|").encode() + corpo
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.send_header("Content-Length", str(len(resp)))
        self.end_headers()
        self.wfile.write(resp)

    do_GET = do_POST = _responder

    def log_message(self, *a):
        pass


@contextmanager
def _servidor(pasta, nome, emissor):
    chave_srv = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    cert_srv = _cert("localhost", chave_srv, ca=emissor)
    pem = pasta / f"{nome}.pem"
    pem.write_bytes(
        cert_srv.public_bytes(serialization.Encoding.PEM)
        + chave_srv.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
                                  serialization.NoEncryption()))
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    ctx.maximum_version = ssl.TLSVersion.TLSv1_2  # como a SEFAZ
    ctx.load_cert_chain(pem)
    ctx.verify_mode = ssl.CERT_REQUIRED  # exige certificado do cliente
    ctx.load_verify_locations(pasta / "ca.pem")
    srv = HTTPServer(("127.0.0.1", 0), _Eco)
    srv.socket = ctx.wrap_socket(srv.socket, server_side=True)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    try:
        yield f"https://localhost:{srv.server_address[1]}/eco?x=1"
    finally:
        srv.shutdown()


def test_winhttp_apresenta_certificado_do_cliente(ambiente):
    from monitor_notas.certificado import Certificado
    tp, pasta, ca = ambiente
    cert = Certificado.do_windows(tp)
    with _servidor(pasta, "confiavel", ca) as url:
        r = cert.requisitar("POST", url, b"<soap/>", {"Content-Type": "application/soap+xml"},
                            verificar_tls=True, timeout=20)
        assert r.status_code == 200
        assert r.content == f"{tp}|POST|<soap/>".encode()
        assert r.headers.get("content-type") == "text/plain"
        r = cert.requisitar("GET", url, verificar_tls=True, timeout=20)
        assert r.content == f"{tp}|GET|".encode()


def test_winhttp_servidor_nao_confiavel(ambiente):
    from monitor_notas.certificado import Certificado
    tp, pasta, _ = ambiente
    cert = Certificado.do_windows(tp)
    with _servidor(pasta, "autoassinado", None) as url:
        with pytest.raises(OSError):
            cert.requisitar("GET", url, verificar_tls=True, timeout=20)
        r = cert.requisitar("GET", url, verificar_tls=False, timeout=20)
        assert r.content == f"{tp}|GET|".encode()
