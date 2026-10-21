"""Leitura e gravação do config.toml (fica na pasta do executável)."""
from __future__ import annotations

import tomllib
from dataclasses import dataclass, field
from pathlib import Path

from . import segredo
from .caminhos import resolver

ARQUIVO_PADRAO = "config.toml"

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
    certificado: str = ""        # arquivo .pfx (quando não usa o repositório do Windows)
    senha_armazenada: str = ""   # como está no arquivo (dpapi:/env:/texto)
    thumbprint: str = ""         # impressão digital do certificado instalado no Windows
    consultar_nfe: bool = True
    consultar_nfse: bool = True

    @property
    def usa_windows(self) -> bool:
        return bool(self.thumbprint)

    def carregar_certificado(self):
        from .certificado import Certificado
        if self.usa_windows:
            return Certificado.do_windows(self.thumbprint)
        return Certificado.de_pfx(self.caminho_certificado, self.senha)

    @property
    def cuf(self) -> int:
        return CODIGOS_UF[self.uf.upper()]

    @property
    def senha(self) -> str:
        return segredo.revelar(self.senha_armazenada)

    @property
    def caminho_certificado(self) -> Path:
        return resolver(self.certificado)


@dataclass
class Config:
    ambiente: str = "producao"  # "producao" | "homologacao"
    banco: str = "dados/monitor.db"
    pasta_xml: str = "dados/xml"
    pasta_exportacao: str = "exportacoes"
    intervalo_minutos: int = 60
    consulta_automatica: bool = False  # enquanto a janela estiver aberta
    verificar_tls: bool | str = True
    empresas: list[Empresa] = field(default_factory=list)

    @property
    def tp_amb(self) -> int:
        return 1 if self.ambiente == "producao" else 2

    @property
    def caminho_banco(self) -> Path:
        return resolver(self.banco)

    @property
    def caminho_xml(self) -> Path:
        return resolver(self.pasta_xml)

    @property
    def caminho_exportacao(self) -> Path:
        return resolver(self.pasta_exportacao)

    @property
    def tls(self) -> bool | str:
        return str(resolver(self.verificar_tls)) if isinstance(self.verificar_tls, str) \
            else self.verificar_tls


def so_digitos(v: str) -> str:
    return "".join(c for c in v if c.isdigit())


def carregar(caminho: str | Path = ARQUIVO_PADRAO) -> Config:
    arq = resolver(caminho)
    if not arq.exists():
        return Config()
    dados = tomllib.loads(arq.read_text(encoding="utf-8"))
    geral = dados.get("geral", {})
    empresas = []
    for e in dados.get("empresa", []):
        cnpj = so_digitos(e["cnpj"])
        if len(cnpj) != 14:
            raise ValueError(f"CNPJ inválido: {e['cnpj']}")
        empresas.append(Empresa(
            nome=e.get("nome", cnpj),
            cnpj=cnpj,
            uf=e["uf"],
            certificado=e.get("certificado", ""),
            senha_armazenada=e.get("senha", ""),
            thumbprint=e.get("thumbprint", "").upper(),
            consultar_nfe=e.get("consultar_nfe", True),
            consultar_nfse=e.get("consultar_nfse", True),
        ))
    return Config(
        ambiente=geral.get("ambiente", "producao"),
        banco=geral.get("banco", "dados/monitor.db"),
        pasta_xml=geral.get("pasta_xml", "dados/xml"),
        pasta_exportacao=geral.get("pasta_exportacao", "exportacoes"),
        intervalo_minutos=max(60, int(geral.get("intervalo_minutos", 60))),
        consulta_automatica=bool(geral.get("consulta_automatica", False)),
        verificar_tls=geral.get("verificar_tls", True),
        empresas=empresas,
    )


def _toml(v) -> str:
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, int):
        return str(v)
    s = str(v).replace("\\", "\\\\").replace('"', '\\"')
    return f'"{s}"'


def salvar(cfg: Config, caminho: str | Path = ARQUIVO_PADRAO) -> None:
    linhas = ["[geral]"]
    for k in ("ambiente", "banco", "pasta_xml", "pasta_exportacao", "intervalo_minutos",
              "consulta_automatica", "verificar_tls"):
        linhas.append(f"{k} = {_toml(getattr(cfg, k))}")
    for e in cfg.empresas:
        linhas += ["", "[[empresa]]"]
        for k, v in (("nome", e.nome), ("cnpj", e.cnpj), ("uf", e.uf.upper()),
                     ("certificado", e.certificado), ("senha", e.senha_armazenada),
                     ("thumbprint", e.thumbprint),
                     ("consultar_nfe", e.consultar_nfe), ("consultar_nfse", e.consultar_nfse)):
            linhas.append(f"{k} = {_toml(v)}")
    arq = resolver(caminho)
    tmp = arq.with_suffix(".tmp")
    tmp.write_text("\n".join(linhas) + "\n", encoding="utf-8")
    tmp.replace(arq)
