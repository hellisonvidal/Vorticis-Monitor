"""Leitura do config.toml."""
from __future__ import annotations

import os
import tomllib
from dataclasses import dataclass, field
from pathlib import Path

# Código IBGE das UFs (cUFAutor da DistDFe)
CODIGOS_UF = {
    "RO": 11, "AC": 12, "AM": 13, "RR": 14, "PA": 15, "AP": 16, "TO": 17,
    "MA": 21, "PI": 22, "CE": 23, "RN": 24, "PB": 25, "PE": 26, "AL": 27,
    "SE": 28, "BA": 29, "MG": 31, "ES": 32, "RJ": 33, "SP": 35, "PR": 41,
    "SC": 42, "RS": 43, "MS": 50, "MT": 51, "GO": 52, "DF": 53,
}


@dataclass
class Empresa:
    nome: str
    cnpj: str
    uf: str
    certificado: str
    senha: str
    consultar_nfe: bool = True
    consultar_nfse: bool = True
    # Deixe False se outro sistema (ex.: Auditto) já faz a manifestação.
    manifestar_ciencia: bool = False

    @property
    def cuf(self) -> int:
        return CODIGOS_UF[self.uf.upper()]


@dataclass
class Config:
    ambiente: str  # "producao" | "homologacao"
    banco: str
    pasta_xml: str
    intervalo_minutos: int
    verificar_tls: bool | str
    empresas: list[Empresa] = field(default_factory=list)

    @property
    def tp_amb(self) -> int:
        return 1 if self.ambiente == "producao" else 2


def _senha(bruta: str) -> str:
    # "env:NOME" lê a senha da variável de ambiente NOME (evita senha em texto no arquivo)
    if bruta.startswith("env:"):
        nome = bruta[4:]
        valor = os.environ.get(nome)
        if valor is None:
            raise ValueError(f"Variável de ambiente {nome} não definida")
        return valor
    return bruta


def carregar(caminho: str | Path = "config.toml") -> Config:
    dados = tomllib.loads(Path(caminho).read_text(encoding="utf-8"))
    geral = dados.get("geral", {})
    empresas = []
    for e in dados.get("empresa", []):
        cnpj = "".join(c for c in e["cnpj"] if c.isdigit())
        if len(cnpj) != 14:
            raise ValueError(f"CNPJ inválido: {e['cnpj']}")
        empresas.append(Empresa(
            nome=e.get("nome", cnpj),
            cnpj=cnpj,
            uf=e["uf"],
            certificado=e["certificado"],
            senha=_senha(e["senha"]),
            consultar_nfe=e.get("consultar_nfe", True),
            consultar_nfse=e.get("consultar_nfse", True),
            manifestar_ciencia=e.get("manifestar_ciencia", False),
        ))
    return Config(
        ambiente=geral.get("ambiente", "producao"),
        banco=geral.get("banco", "dados/monitor.db"),
        pasta_xml=geral.get("pasta_xml", "dados/xml"),
        intervalo_minutos=max(60, int(geral.get("intervalo_minutos", 60))),
        verificar_tls=geral.get("verificar_tls", True),
        empresas=empresas,
    )
