"""Persistência em SQLite + cópia dos XMLs em disco."""
from __future__ import annotations

import sqlite3
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

ESQUEMA = """
CREATE TABLE IF NOT EXISTS estado (
    chave TEXT PRIMARY KEY,
    valor TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS documentos (
    id INTEGER PRIMARY KEY,
    origem TEXT NOT NULL,          -- 'nfe' | 'nfse'
    tipo TEXT NOT NULL,            -- 'resumo' | 'completo' | 'evento'
    direcao TEXT NOT NULL,         -- 'recebida' | 'emitida'
    cnpj_empresa TEXT NOT NULL,
    chave TEXT NOT NULL,
    nsu TEXT,
    emitente_cnpj TEXT,
    emitente_nome TEXT,
    destinatario_cnpj TEXT,
    valor REAL,
    data_emissao TEXT,
    evento TEXT,                   -- código do evento (ex.: 110111 cancelamento)
    xml TEXT NOT NULL,
    registrado_em TEXT NOT NULL,
    UNIQUE (origem, tipo, chave, evento)
);
CREATE INDEX IF NOT EXISTS ix_doc_chave ON documentos (chave);
CREATE INDEX IF NOT EXISTS ix_doc_empresa ON documentos (cnpj_empresa, data_emissao);
"""


@dataclass
class Documento:
    origem: str
    tipo: str
    direcao: str
    cnpj_empresa: str
    chave: str
    xml: str
    nsu: str | None = None
    emitente_cnpj: str | None = None
    emitente_nome: str | None = None
    destinatario_cnpj: str | None = None
    valor: float | None = None
    data_emissao: str | None = None
    evento: str = ""


class Armazenamento:
    def __init__(self, banco: str, pasta_xml: str):
        Path(banco).parent.mkdir(parents=True, exist_ok=True)
        self.pasta_xml = Path(pasta_xml)
        self.con = sqlite3.connect(banco)
        self.con.row_factory = sqlite3.Row
        self.con.executescript(ESQUEMA)

    def obter_estado(self, chave: str, padrao: str | None = None) -> str | None:
        linha = self.con.execute("SELECT valor FROM estado WHERE chave = ?", (chave,)).fetchone()
        return linha["valor"] if linha else padrao

    def gravar_estado(self, chave: str, valor: str) -> None:
        with self.con:
            self.con.execute(
                "INSERT INTO estado (chave, valor) VALUES (?, ?) "
                "ON CONFLICT(chave) DO UPDATE SET valor = excluded.valor",
                (chave, valor),
            )

    def salvar(self, doc: Documento) -> bool:
        """Retorna True se o documento é novo."""
        with self.con:
            cur = self.con.execute(
                """INSERT OR IGNORE INTO documentos
                   (origem, tipo, direcao, cnpj_empresa, chave, nsu, emitente_cnpj, emitente_nome,
                    destinatario_cnpj, valor, data_emissao, evento, xml, registrado_em)
                   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)""",
                (doc.origem, doc.tipo, doc.direcao, doc.cnpj_empresa, doc.chave, doc.nsu,
                 doc.emitente_cnpj, doc.emitente_nome, doc.destinatario_cnpj, doc.valor,
                 doc.data_emissao, doc.evento, doc.xml,
                 datetime.now(timezone.utc).isoformat(timespec="seconds")),
            )
            novo = cur.rowcount == 1
            # dentro da transação: se a gravação do arquivo falhar, o INSERT é desfeito
            if novo:
                self._gravar_arquivo(doc)
        return novo

    def _gravar_arquivo(self, doc: Documento) -> None:
        mes = (doc.data_emissao or "")[:7] or "sem-data"
        pasta = self.pasta_xml / doc.cnpj_empresa / doc.origem / doc.direcao / mes
        pasta.mkdir(parents=True, exist_ok=True)
        sufixo = {"completo": "", "resumo": "-resumo"}.get(doc.tipo, f"-evento{doc.evento}")
        (pasta / f"{doc.chave}{sufixo}.xml").write_text(doc.xml, encoding="utf-8")

    def tem_completo(self, origem: str, chave: str) -> bool:
        return self.con.execute(
            "SELECT 1 FROM documentos WHERE origem = ? AND chave = ? AND tipo = 'completo'",
            (origem, chave),
        ).fetchone() is not None

    def resumos_sem_completo(self, cnpj: str) -> list[str]:
        linhas = self.con.execute(
            """SELECT DISTINCT r.chave FROM documentos r
               WHERE r.origem = 'nfe' AND r.tipo = 'resumo' AND r.cnpj_empresa = ?
               AND NOT EXISTS (SELECT 1 FROM documentos c
                               WHERE c.origem = 'nfe' AND c.tipo = 'completo' AND c.chave = r.chave)""",
            (cnpj,),
        ).fetchall()
        return [l["chave"] for l in linhas]

    def listar(self, cnpj: str | None = None, limite: int = 50) -> list[sqlite3.Row]:
        sql = "SELECT * FROM documentos"
        args: tuple = ()
        if cnpj:
            sql += " WHERE cnpj_empresa = ?"
            args = (cnpj,)
        sql += " ORDER BY registrado_em DESC, id DESC LIMIT ?"
        return self.con.execute(sql, args + (limite,)).fetchall()
