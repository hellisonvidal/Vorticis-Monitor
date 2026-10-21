"""Proteção da senha do certificado no config.

No Windows usa DPAPI (CryptProtectData): só o mesmo usuário, no mesmo computador, descriptografa.
Formatos aceitos no config:
  "dpapi:<base64>"  senha protegida (gravada pela interface)
  "env:NOME"        lê da variável de ambiente NOME
  qualquer outro    texto puro (não recomendado)
"""
from __future__ import annotations

import base64
import os
import sys


def _dpapi(dados: bytes, proteger: bool) -> bytes:
    import ctypes
    from ctypes import wintypes

    class BLOB(ctypes.Structure):
        _fields_ = [("cbData", wintypes.DWORD), ("pbData", ctypes.POINTER(ctypes.c_char))]

    crypt32, kernel32 = ctypes.windll.crypt32, ctypes.windll.kernel32
    buf = ctypes.create_string_buffer(dados, len(dados))
    entrada = BLOB(len(dados), ctypes.cast(buf, ctypes.POINTER(ctypes.c_char)))
    saida = BLOB()
    CRYPTPROTECT_UI_FORBIDDEN = 0x1
    fn = crypt32.CryptProtectData if proteger else crypt32.CryptUnprotectData
    ok = fn(ctypes.byref(entrada), None, None, None, None, CRYPTPROTECT_UI_FORBIDDEN,
            ctypes.byref(saida))
    if not ok:
        raise OSError("DPAPI falhou: a senha foi gravada por outro usuário ou computador? "
                      "Informe a senha novamente.")
    try:
        return ctypes.string_at(saida.pbData, saida.cbData)
    finally:
        kernel32.LocalFree(saida.pbData)


def disponivel() -> bool:
    return sys.platform == "win32"


def proteger(senha: str) -> str:
    if not disponivel():
        return senha  # fora do Windows (desenvolvimento): texto puro
    return "dpapi:" + base64.b64encode(_dpapi(senha.encode("utf-8"), True)).decode()


def revelar(valor: str) -> str:
    if valor.startswith("dpapi:"):
        return _dpapi(base64.b64decode(valor[6:]), False).decode("utf-8")
    if valor.startswith("env:"):
        nome = valor[4:]
        if nome not in os.environ:
            raise ValueError(f"Variável de ambiente {nome} não definida")
        return os.environ[nome]
    return valor
