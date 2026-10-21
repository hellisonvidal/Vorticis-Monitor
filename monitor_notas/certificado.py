"""Certificado digital ICP-Brasil e transporte HTTPS autenticado.

Duas origens:
- arquivo .pfx/.p12 (A1) com senha — TLS via `requests`;
- instalado no Windows (A1 instalado ou A3 em token/cartão) — TLS via WinHTTP, sem exportar a chave.
"""
from __future__ import annotations

import atexit
import json
import os
import re
import shutil
import tempfile
from dataclasses import dataclass
from pathlib import Path

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.serialization import pkcs12
from cryptography.x509.oid import NameOID

OID_CNPJ = x509.ObjectIdentifier("2.16.76.1.3.3")  # ICP-Brasil: CNPJ da pessoa jurídica


@dataclass
class Resposta:
    status_code: int
    content: bytes
    headers: dict

    @property
    def text(self) -> str:
        return self.content.decode("utf-8", errors="replace")

    def json(self):
        return json.loads(self.content)

    def raise_for_status(self) -> None:
        if self.status_code >= 400:
            raise RuntimeError(f"HTTP {self.status_code}: {self.text[:300]}")


@dataclass
class DadosCertificado:
    """Informações extraídas do certificado (para preencher o cadastro da empresa)."""
    titular: str
    razao_social: str
    cnpj: str | None
    uf: str | None
    cidade: str | None
    validade: object
    emissor: str
    thumbprint: str


def ler_dados(cert: x509.Certificate) -> DadosCertificado:
    def attr(oid):
        v = cert.subject.get_attributes_for_oid(oid)
        return v[0].value if v else None

    cn = attr(NameOID.COMMON_NAME) or ""
    # e-CNPJ ICP-Brasil: CN = "RAZAO SOCIAL:12345678000190"
    m = re.match(r"^(.*?):(\d{14})$", cn)
    razao, cnpj = (m.group(1), m.group(2)) if m else (cn, None)
    if cnpj is None:
        cnpj = _cnpj_san(cert)
    emissor = cert.issuer.get_attributes_for_oid(NameOID.COMMON_NAME)
    return DadosCertificado(
        titular=cn,
        razao_social=razao.strip(),
        cnpj=cnpj,
        uf=(attr(NameOID.STATE_OR_PROVINCE_NAME) or "").strip().upper()[:2] or None,
        cidade=attr(NameOID.LOCALITY_NAME),
        validade=cert.not_valid_after_utc,
        emissor=emissor[0].value if emissor else "",
        thumbprint=cert.fingerprint(hashes.SHA1()).hex().upper(),
    )


def _cnpj_san(cert: x509.Certificate) -> str | None:
    try:
        san = cert.extensions.get_extension_for_class(x509.SubjectAlternativeName).value
    except x509.ExtensionNotFound:
        return None
    for nome in san.get_values_for_type(x509.OtherName):
        if nome.type_id == OID_CNPJ:
            digitos = re.search(rb"\d{14}", nome.value)
            if digitos:
                return digitos.group().decode()
    return None


@dataclass
class Certificado:
    cert: x509.Certificate
    chave: object | None = None           # origem arquivo
    thumbprint_windows: str | None = None  # origem Windows
    _dir_temp: str | None = None

    @classmethod
    def de_pfx(cls, caminho: str | Path, senha: str) -> "Certificado":
        dados = Path(caminho).read_bytes()
        chave, cert, _ = pkcs12.load_key_and_certificates(dados, senha.encode())
        if chave is None or cert is None:
            raise ValueError(f"PFX sem chave privada ou certificado: {caminho}")
        return cls(cert=cert, chave=chave)

    @classmethod
    def do_windows(cls, thumbprint: str) -> "Certificado":
        from . import winstore
        der = winstore.obter_der(thumbprint)
        return cls(cert=x509.load_der_x509_certificate(der), thumbprint_windows=thumbprint.upper())

    @property
    def dados(self) -> DadosCertificado:
        return ler_dados(self.cert)

    @property
    def validade(self):
        return self.cert.not_valid_after_utc

    @property
    def titular(self) -> str:
        return self.cert.subject.rfc4514_string()

    # --- transporte -------------------------------------------------------------------

    def requisitar(self, metodo: str, url: str, dados: bytes | None = None,
                   cabecalhos: dict | None = None, verificar_tls: bool | str = True,
                   timeout: int = 60) -> Resposta:
        if self.thumbprint_windows:
            from . import winstore
            # WinHTTP usa as raízes do Windows; um caminho de cadeia .pem não se aplica
            status, cab, corpo = winstore.requisitar(
                self.thumbprint_windows, metodo, url, dados, cabecalhos,
                verificar_tls=verificar_tls is not False, timeout=timeout)
            return Resposta(status, corpo, cab)
        import requests
        r = requests.request(metodo, url, data=dados, headers=cabecalhos,
                             cert=self._arquivos_pem(), verify=verificar_tls, timeout=timeout)
        return Resposta(r.status_code, r.content, dict(r.headers))

    def _arquivos_pem(self) -> tuple[str, str]:
        """Grava cert e chave em PEM (0600) num diretório temporário, para o mTLS do requests.

        Os arquivos são apagados ao final do processo.
        """
        if self._dir_temp is None:
            self._dir_temp = tempfile.mkdtemp(prefix="vorticis-")
            os.chmod(self._dir_temp, 0o700)
            atexit.register(shutil.rmtree, self._dir_temp, True)
            for nome, conteudo in (
                ("cert.pem", self.cert.public_bytes(serialization.Encoding.PEM)),
                ("key.pem", self.chave.private_bytes(
                    serialization.Encoding.PEM,
                    serialization.PrivateFormat.PKCS8,
                    serialization.NoEncryption(),
                )),
            ):
                fd = os.open(os.path.join(self._dir_temp, nome),
                             os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
                with os.fdopen(fd, "wb") as f:
                    f.write(conteudo)
        return (os.path.join(self._dir_temp, "cert.pem"), os.path.join(self._dir_temp, "key.pem"))
