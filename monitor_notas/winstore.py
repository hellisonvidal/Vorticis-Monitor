"""Certificados instalados no Windows (repositório "Pessoal" do usuário) via CryptoAPI + WinHTTP.

Permite usar certificados A1 instalados e A3 (token/cartão): a chave privada nunca sai do
Windows — o TLS com certificado do cliente é feito pelo próprio WinHTTP (Schannel), que também
usa as raízes confiáveis do Windows (inclui a ICP-Brasil).
"""
from __future__ import annotations

import ctypes
import hashlib
import sys
from contextlib import contextmanager
from ctypes import wintypes
from urllib.parse import urlsplit

if sys.platform != "win32":  # pragma: no cover - módulo só funciona no Windows
    raise ImportError("winstore só está disponível no Windows")

crypt32 = ctypes.WinDLL("crypt32", use_last_error=True)
cryptui = ctypes.WinDLL("cryptui", use_last_error=True)
winhttp = ctypes.WinDLL("winhttp", use_last_error=True)

ENCODING = 0x00000001 | 0x00010000  # X509_ASN_ENCODING | PKCS_7_ASN_ENCODING
CERT_FIND_SHA1_HASH = 0x00010000
CERT_KEY_PROV_INFO_PROP_ID = 2
CERT_STORE_PROV_MEMORY = 2
CERT_STORE_ADD_ALWAYS = 4

HCERTSTORE = ctypes.c_void_p


class CERT_CONTEXT(ctypes.Structure):
    _fields_ = [
        ("dwCertEncodingType", wintypes.DWORD),
        ("pbCertEncoded", ctypes.POINTER(ctypes.c_ubyte)),
        ("cbCertEncoded", wintypes.DWORD),
        ("pCertInfo", ctypes.c_void_p),
        ("hCertStore", HCERTSTORE),
    ]


PCCERT_CONTEXT = ctypes.POINTER(CERT_CONTEXT)


class CRYPT_HASH_BLOB(ctypes.Structure):
    _fields_ = [("cbData", wintypes.DWORD), ("pbData", ctypes.POINTER(ctypes.c_ubyte))]


def _sig(fn, res, *args):
    fn.restype, fn.argtypes = res, list(args)
    return fn


_sig(crypt32.CertOpenSystemStoreW, HCERTSTORE, ctypes.c_void_p, wintypes.LPCWSTR)
_sig(crypt32.CertOpenStore, HCERTSTORE, ctypes.c_void_p, wintypes.DWORD, ctypes.c_void_p,
     wintypes.DWORD, ctypes.c_void_p)
_sig(crypt32.CertCloseStore, wintypes.BOOL, HCERTSTORE, wintypes.DWORD)
_sig(crypt32.CertEnumCertificatesInStore, PCCERT_CONTEXT, HCERTSTORE, PCCERT_CONTEXT)
_sig(crypt32.CertFindCertificateInStore, PCCERT_CONTEXT, HCERTSTORE, wintypes.DWORD,
     wintypes.DWORD, wintypes.DWORD, ctypes.c_void_p, PCCERT_CONTEXT)
_sig(crypt32.CertGetCertificateContextProperty, wintypes.BOOL, PCCERT_CONTEXT, wintypes.DWORD,
     ctypes.c_void_p, ctypes.POINTER(wintypes.DWORD))
_sig(crypt32.CertAddCertificateContextToStore, wintypes.BOOL, HCERTSTORE, PCCERT_CONTEXT,
     wintypes.DWORD, ctypes.c_void_p)
_sig(crypt32.CertFreeCertificateContext, wintypes.BOOL, PCCERT_CONTEXT)
_sig(cryptui.CryptUIDlgSelectCertificateFromStore, PCCERT_CONTEXT, HCERTSTORE, wintypes.HWND,
     wintypes.LPCWSTR, wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.c_void_p)


def _erro(msg: str) -> OSError:
    cod = ctypes.get_last_error()
    return OSError(cod, f"{msg} (erro Windows {cod}: {ctypes.FormatError(cod).strip()})")


def _der(ctx: PCCERT_CONTEXT) -> bytes:
    c = ctx.contents
    return ctypes.string_at(c.pbCertEncoded, c.cbCertEncoded)


def impressao_digital(der: bytes) -> str:
    return hashlib.sha1(der).hexdigest().upper()


def _tem_chave(ctx: PCCERT_CONTEXT) -> bool:
    tam = wintypes.DWORD(0)
    return bool(crypt32.CertGetCertificateContextProperty(
        ctx, CERT_KEY_PROV_INFO_PROP_ID, None, ctypes.byref(tam)))


@contextmanager
def _repositorio():
    store = crypt32.CertOpenSystemStoreW(None, "MY")
    if not store:
        raise _erro("Não foi possível abrir o repositório de certificados")
    try:
        yield store
    finally:
        crypt32.CertCloseStore(store, 0)


def listar_der() -> list[bytes]:
    """Certificados do usuário que têm chave privada associada."""
    res = []
    with _repositorio() as store:
        ctx = crypt32.CertEnumCertificatesInStore(store, None)
        while ctx:
            if _tem_chave(ctx):
                res.append(_der(ctx))
            ctx = crypt32.CertEnumCertificatesInStore(store, ctx)  # libera o anterior
    return res


@contextmanager
def _contexto(thumbprint: str):
    hash_bytes = bytes.fromhex(thumbprint)
    buf = (ctypes.c_ubyte * len(hash_bytes)).from_buffer_copy(hash_bytes)
    blob = CRYPT_HASH_BLOB(len(hash_bytes), buf)
    with _repositorio() as store:
        ctx = crypt32.CertFindCertificateInStore(
            store, ENCODING, 0, CERT_FIND_SHA1_HASH, ctypes.byref(blob), None)
        if not ctx:
            raise LookupError(f"Certificado {thumbprint} não encontrado no Windows. "
                              "Ele foi removido ou o token não está conectado?")
        try:
            yield ctx
        finally:
            crypt32.CertFreeCertificateContext(ctx)


def obter_der(thumbprint: str) -> bytes:
    with _contexto(thumbprint) as ctx:
        return _der(ctx)


def selecionar(hwnd: int = 0, titulo: str = "Selecionar certificado digital",
               texto: str = "Escolha o certificado da empresa:", filtro=None) -> str | None:
    """Abre a janela nativa do Windows (a mesma dos navegadores). Retorna a impressão digital.

    `filtro(der) -> bool` restringe os certificados exibidos (ex.: só e-CNPJ válidos).
    """
    memoria = crypt32.CertOpenStore(CERT_STORE_PROV_MEMORY, 0, None, 0, None)
    if not memoria:
        raise _erro("Não foi possível criar a lista de certificados")
    try:
        with _repositorio() as store:
            ctx = crypt32.CertEnumCertificatesInStore(store, None)
            while ctx:
                if _tem_chave(ctx) and (filtro is None or filtro(_der(ctx))):
                    crypt32.CertAddCertificateContextToStore(memoria, ctx, CERT_STORE_ADD_ALWAYS, None)
                ctx = crypt32.CertEnumCertificatesInStore(store, ctx)
        escolhido = cryptui.CryptUIDlgSelectCertificateFromStore(
            memoria, hwnd, titulo, texto, 0, 0, None)
        if not escolhido:
            return None  # cancelado
        try:
            return impressao_digital(_der(escolhido))
        finally:
            crypt32.CertFreeCertificateContext(escolhido)
    finally:
        crypt32.CertCloseStore(memoria, 0)


# --- WinHTTP -------------------------------------------------------------------------------

HINTERNET = ctypes.c_void_p
WINHTTP_ACCESS_TYPE_DEFAULT_PROXY = 0
WINHTTP_ACCESS_TYPE_AUTOMATIC_PROXY = 4
WINHTTP_FLAG_SECURE = 0x00800000
WINHTTP_OPTION_SECURITY_FLAGS = 31
WINHTTP_OPTION_CLIENT_CERT_CONTEXT = 47
WINHTTP_QUERY_STATUS_CODE = 19
WINHTTP_QUERY_RAW_HEADERS_CRLF = 22
WINHTTP_QUERY_FLAG_NUMBER = 0x20000000
IGNORAR_ERROS_TLS = 0x00000100 | 0x00000200 | 0x00001000 | 0x00002000

_sig(winhttp.WinHttpOpen, HINTERNET, wintypes.LPCWSTR, wintypes.DWORD, wintypes.LPCWSTR,
     wintypes.LPCWSTR, wintypes.DWORD)
_sig(winhttp.WinHttpSetTimeouts, wintypes.BOOL, HINTERNET, ctypes.c_int, ctypes.c_int,
     ctypes.c_int, ctypes.c_int)
_sig(winhttp.WinHttpConnect, HINTERNET, HINTERNET, wintypes.LPCWSTR, wintypes.WORD, wintypes.DWORD)
_sig(winhttp.WinHttpOpenRequest, HINTERNET, HINTERNET, wintypes.LPCWSTR, wintypes.LPCWSTR,
     wintypes.LPCWSTR, wintypes.LPCWSTR, ctypes.c_void_p, wintypes.DWORD)
_sig(winhttp.WinHttpSetOption, wintypes.BOOL, HINTERNET, wintypes.DWORD, ctypes.c_void_p,
     wintypes.DWORD)
_sig(winhttp.WinHttpSendRequest, wintypes.BOOL, HINTERNET, wintypes.LPCWSTR, wintypes.DWORD,
     ctypes.c_void_p, wintypes.DWORD, wintypes.DWORD, ctypes.c_size_t)
_sig(winhttp.WinHttpReceiveResponse, wintypes.BOOL, HINTERNET, ctypes.c_void_p)
_sig(winhttp.WinHttpQueryHeaders, wintypes.BOOL, HINTERNET, wintypes.DWORD, wintypes.LPCWSTR,
     ctypes.c_void_p, ctypes.POINTER(wintypes.DWORD), ctypes.POINTER(wintypes.DWORD))
_sig(winhttp.WinHttpQueryDataAvailable, wintypes.BOOL, HINTERNET, ctypes.POINTER(wintypes.DWORD))
_sig(winhttp.WinHttpReadData, wintypes.BOOL, HINTERNET, ctypes.c_void_p, wintypes.DWORD,
     ctypes.POINTER(wintypes.DWORD))
_sig(winhttp.WinHttpCloseHandle, wintypes.BOOL, HINTERNET)


def _werro(msg: str) -> OSError:
    cod = ctypes.get_last_error()
    # Mensagens dos códigos WinHTTP mais comuns (12xxx não aparecem no FormatError padrão)
    conhecidos = {
        12002: "tempo esgotado", 12007: "servidor não encontrado (DNS)",
        12029: "não foi possível conectar", 12030: "conexão interrompida",
        12044: "o servidor exigiu certificado do cliente",
        12175: "erro de segurança/TLS (certificado do servidor ou do cliente)",
        12157: "erro no canal seguro (TLS)",
    }
    return OSError(cod, f"{msg}: {conhecidos.get(cod, 'erro WinHTTP')} ({cod})")


def requisitar(thumbprint: str, metodo: str, url: str, corpo: bytes | None = None,
               cabecalhos: dict[str, str] | None = None, verificar_tls: bool = True,
               timeout: int = 60) -> tuple[int, dict[str, str], bytes]:
    """Requisição HTTPS autenticada com o certificado do Windows. Retorna (status, cabeçalhos, corpo)."""
    partes = urlsplit(url)
    if partes.scheme != "https":
        raise ValueError("Somente HTTPS é suportado")
    caminho = (partes.path or "/") + (f"?{partes.query}" if partes.query else "")
    porta = partes.port or 443
    ms = timeout * 1000
    handles = []
    try:
        sessao = winhttp.WinHttpOpen("VorticisMonitor/1.0", WINHTTP_ACCESS_TYPE_AUTOMATIC_PROXY,
                                     None, None, 0) \
            or winhttp.WinHttpOpen("VorticisMonitor/1.0", WINHTTP_ACCESS_TYPE_DEFAULT_PROXY,
                                   None, None, 0)
        if not sessao:
            raise _werro("WinHttpOpen")
        handles.append(sessao)
        winhttp.WinHttpSetTimeouts(sessao, ms, ms, ms, ms)
        conexao = winhttp.WinHttpConnect(sessao, partes.hostname, porta, 0)
        if not conexao:
            raise _werro(f"Conexão com {partes.hostname}")
        handles.append(conexao)
        req = winhttp.WinHttpOpenRequest(conexao, metodo, caminho, None, None, None,
                                         WINHTTP_FLAG_SECURE)
        if not req:
            raise _werro("WinHttpOpenRequest")
        handles.append(req)

        if not verificar_tls:
            flags = wintypes.DWORD(IGNORAR_ERROS_TLS)
            winhttp.WinHttpSetOption(req, WINHTTP_OPTION_SECURITY_FLAGS, ctypes.byref(flags),
                                     ctypes.sizeof(flags))

        with _contexto(thumbprint) as ctx:
            if not winhttp.WinHttpSetOption(req, WINHTTP_OPTION_CLIENT_CERT_CONTEXT, ctx,
                                            ctypes.sizeof(CERT_CONTEXT)):
                raise _werro("Configurar certificado do cliente")
            texto_cab = "".join(f"{k}: {v}\r\n" for k, v in (cabecalhos or {}).items())
            dados = corpo or b""
            buf = ctypes.create_string_buffer(dados, len(dados)) if dados else None
            if not winhttp.WinHttpSendRequest(req, texto_cab or None,
                                              0xFFFFFFFF if texto_cab else 0,
                                              buf, len(dados), len(dados), 0):
                raise _werro(f"Envio para {partes.hostname}")
            if not winhttp.WinHttpReceiveResponse(req, None):
                raise _werro(f"Resposta de {partes.hostname}")

        status = wintypes.DWORD(0)
        tam = wintypes.DWORD(ctypes.sizeof(status))
        winhttp.WinHttpQueryHeaders(req, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER,
                                    None, ctypes.byref(status), ctypes.byref(tam), None)
        cab = _cabecalhos(req)

        pedacos = []
        while True:
            disp = wintypes.DWORD(0)
            if not winhttp.WinHttpQueryDataAvailable(req, ctypes.byref(disp)):
                raise _werro("Leitura da resposta")
            if disp.value == 0:
                break
            b = ctypes.create_string_buffer(disp.value)
            lidos = wintypes.DWORD(0)
            if not winhttp.WinHttpReadData(req, b, disp.value, ctypes.byref(lidos)):
                raise _werro("Leitura da resposta")
            pedacos.append(b.raw[:lidos.value])
        return status.value, cab, b"".join(pedacos)
    finally:
        for h in reversed(handles):
            winhttp.WinHttpCloseHandle(h)


def _cabecalhos(req) -> dict[str, str]:
    tam = wintypes.DWORD(0)
    winhttp.WinHttpQueryHeaders(req, WINHTTP_QUERY_RAW_HEADERS_CRLF, None, None,
                                ctypes.byref(tam), None)
    if not tam.value:
        return {}
    buf = ctypes.create_unicode_buffer(tam.value // 2 + 1)
    if not winhttp.WinHttpQueryHeaders(req, WINHTTP_QUERY_RAW_HEADERS_CRLF, None, buf,
                                       ctypes.byref(tam), None):
        return {}
    res = {}
    for linha in buf.value.split("\r\n")[1:]:
        if ":" in linha:
            k, v = linha.split(":", 1)
            res[k.strip().lower()] = v.strip()
    return res
