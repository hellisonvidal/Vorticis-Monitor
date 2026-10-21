"""Distribuição de DF-e da NFS-e Padrão Nacional (ADN — API de contribuintes).

Fonte oficial: Manual dos Contribuintes — APIs ADN (v1.0, fev/2026): GET /DFe/{NSU}, mTLS ICP-Brasil.
Nomes de campos JSON e o parâmetro `lote`/`cnpjConsulta` vêm de integrações de terceiros e
DEVEM ser validados na produção restrita antes de uso em produção.
"""
from __future__ import annotations

import base64
import gzip
from dataclasses import dataclass, field
from urllib.parse import urlencode

from ..certificado import Certificado

URLS = {
    1: "https://adn.nfse.gov.br/contribuintes",
    2: "https://adn.producaorestrita.nfse.gov.br/contribuintes",
}

LOCALIZADOS = "DOCUMENTOS_LOCALIZADOS"
NENHUM = "NENHUM_DOCUMENTO_LOCALIZADO"


@dataclass
class ItemDFe:
    nsu: int
    chave: str | None
    tipo_documento: str | None
    tipo_evento: str | None
    xml: str


@dataclass
class RetornoADN:
    status: str
    http: int
    itens: list[ItemDFe] = field(default_factory=list)
    erros: list = field(default_factory=list)

    @property
    def fim(self) -> bool:
        return self.status == NENHUM or not self.itens

    @property
    def erro(self) -> bool:
        # E2243 e outros vêm com HTTP 400 mesmo com status DOCUMENTOS_LOCALIZADOS
        return bool(self.erros) and self.status != NENHUM

    @property
    def maior_nsu(self) -> int | None:
        return max((i.nsu for i in self.itens), default=None)


def _campo(d: dict, *nomes):
    # tolera PascalCase / camelCase / minúsculas (ex.: "NSU" ou "nsu")
    por_nome = {k.lower(): v for k, v in d.items()}
    for n in nomes:
        if n.lower() in por_nome:
            return por_nome[n.lower()]
    return None


def interpretar(http: int, dados: dict) -> RetornoADN:
    itens = []
    for it in _campo(dados, "LoteDFe") or []:
        bruto = _campo(it, "ArquivoXml")
        if not bruto:
            continue
        xml = gzip.decompress(base64.b64decode(bruto)).decode("utf-8")
        itens.append(ItemDFe(
            nsu=int(_campo(it, "NSU")),
            chave=_campo(it, "ChaveAcesso"),
            tipo_documento=_campo(it, "TipoDocumento"),
            tipo_evento=_campo(it, "TipoEvento"),
            xml=xml,
        ))
    return RetornoADN(
        status=_campo(dados, "StatusProcessamento") or "",
        http=http,
        itens=itens,
        erros=_campo(dados, "Erros") or [],
    )


class ClienteADN:
    def __init__(self, cert: Certificado, tp_amb: int, cnpj: str,
                 verificar_tls: bool | str = True, enviar_cnpj_consulta: bool = True):
        self.cert, self.base, self.cnpj = cert, URLS[tp_amb], cnpj
        self.verificar_tls = verificar_tls
        self.enviar_cnpj_consulta = enviar_cnpj_consulta

    def por_nsu(self, ult_nsu: int) -> RetornoADN:
        params = {"lote": "true"}
        if self.enviar_cnpj_consulta:
            params["cnpjConsulta"] = self.cnpj
        resp = self.cert.requisitar(
            "GET", f"{self.base}/DFe/{ult_nsu}?{urlencode(params)}",
            cabecalhos={"Accept": "application/json"},
            verificar_tls=self.verificar_tls)
        if resp.status_code == 429 or resp.status_code >= 500:
            resp.raise_for_status()
        try:
            dados = resp.json()
        except ValueError:
            resp.raise_for_status()
            raise RuntimeError(f"ADN: resposta não-JSON (HTTP {resp.status_code}): {resp.text[:300]}")
        return interpretar(resp.status_code, dados)


def baixar_danfse(cert: Certificado, tp_amb: int, chave: str,
                  verificar_tls: bool | str = True) -> bytes:
    """PDF oficial da NFS-e (DANFSe) gerado pelo ADN. Endpoint /danfse/{chave} no host do ADN."""
    base = URLS[tp_amb].rsplit("/contribuintes", 1)[0]
    resp = cert.requisitar("GET", f"{base}/danfse/{chave}",
                           cabecalhos={"Accept": "application/pdf"}, verificar_tls=verificar_tls)
    if resp.status_code != 200 or not resp.content.startswith(b"%PDF"):
        raise RuntimeError(f"DANFSe indisponível no ADN (HTTP {resp.status_code})")
    return resp.content
