"""Carregamento do certificado digital ICP-Brasil.

A1 (.pfx/.p12): suportado diretamente.
A3 (token/cartão): ver README — exige PKCS#11 e fica para a fase 2.
"""
from __future__ import annotations

import atexit
import base64
import os
import shutil
import tempfile
from dataclasses import dataclass
from pathlib import Path

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.serialization import pkcs12


@dataclass
class Certificado:
    chave: object
    cert: x509.Certificate
    _dir_temp: str | None = None

    @classmethod
    def de_pfx(cls, caminho: str | Path, senha: str) -> "Certificado":
        dados = Path(caminho).read_bytes()
        chave, cert, _ = pkcs12.load_key_and_certificates(dados, senha.encode())
        if chave is None or cert is None:
            raise ValueError(f"PFX sem chave privada ou certificado: {caminho}")
        return cls(chave=chave, cert=cert)

    @property
    def validade(self):
        return self.cert.not_valid_after_utc

    @property
    def titular(self) -> str:
        return self.cert.subject.rfc4514_string()

    def cert_base64(self) -> str:
        return base64.b64encode(self.cert.public_bytes(serialization.Encoding.DER)).decode()

    def assinar_sha1(self, dados: bytes) -> bytes:
        return self.chave.sign(dados, padding.PKCS1v15(), hashes.SHA1())

    def arquivos_pem(self) -> tuple[str, str]:
        """Grava cert e chave em PEM (0600) num diretório temporário, para o mTLS do requests.

        Os arquivos são apagados ao final do processo.
        """
        if self._dir_temp is None:
            self._dir_temp = tempfile.mkdtemp(prefix="monitor-notas-")
            os.chmod(self._dir_temp, 0o700)
            atexit.register(shutil.rmtree, self._dir_temp, True)
            cert_path = os.path.join(self._dir_temp, "cert.pem")
            key_path = os.path.join(self._dir_temp, "key.pem")
            for caminho, conteudo in (
                (cert_path, self.cert.public_bytes(serialization.Encoding.PEM)),
                (key_path, self.chave.private_bytes(
                    serialization.Encoding.PEM,
                    serialization.PrivateFormat.PKCS8,
                    serialization.NoEncryption(),
                )),
            ):
                fd = os.open(caminho, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
                with os.fdopen(fd, "wb") as f:
                    f.write(conteudo)
        return (os.path.join(self._dir_temp, "cert.pem"), os.path.join(self._dir_temp, "key.pem"))
