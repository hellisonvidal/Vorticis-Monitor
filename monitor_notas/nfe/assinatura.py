"""Assinatura XML-DSig enveloped no padrão da NF-e (RSA-SHA1, C14N 2001)."""
from __future__ import annotations

import base64
import hashlib

from lxml import etree

from ..certificado import Certificado

DS = "http://www.w3.org/2000/09/xmldsig#"
C14N = "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"


def _c14n(el: etree._Element) -> bytes:
    return etree.tostring(el, method="c14n", exclusive=False, with_comments=False)


def assinar(pai: etree._Element, alvo: etree._Element, cert: Certificado) -> None:
    """Assina `alvo` (que tem atributo Id) e anexa <Signature> como último filho de `pai`."""
    uri = "#" + alvo.get("Id")
    digest = base64.b64encode(hashlib.sha1(_c14n(alvo)).digest()).decode()

    E = lambda tag, **attrs: etree.Element(f"{{{DS}}}{tag}", nsmap={None: DS}, **attrs)  # noqa: E731
    sig = E("Signature")
    pai.append(sig)

    si = etree.SubElement(sig, f"{{{DS}}}SignedInfo")
    etree.SubElement(si, f"{{{DS}}}CanonicalizationMethod", Algorithm=C14N)
    etree.SubElement(si, f"{{{DS}}}SignatureMethod", Algorithm=DS + "rsa-sha1")
    ref = etree.SubElement(si, f"{{{DS}}}Reference", URI=uri)
    tr = etree.SubElement(ref, f"{{{DS}}}Transforms")
    etree.SubElement(tr, f"{{{DS}}}Transform", Algorithm=DS + "enveloped-signature")
    etree.SubElement(tr, f"{{{DS}}}Transform", Algorithm=C14N)
    etree.SubElement(ref, f"{{{DS}}}DigestMethod", Algorithm=DS + "sha1")
    etree.SubElement(ref, f"{{{DS}}}DigestValue").text = digest

    valor = base64.b64encode(cert.assinar_sha1(_c14n(si))).decode()
    etree.SubElement(sig, f"{{{DS}}}SignatureValue").text = valor
    ki = etree.SubElement(sig, f"{{{DS}}}KeyInfo")
    x509 = etree.SubElement(ki, f"{{{DS}}}X509Data")
    etree.SubElement(x509, f"{{{DS}}}X509Certificate").text = cert.cert_base64()
