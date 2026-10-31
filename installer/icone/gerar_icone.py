"""Gera o ícone próprio do Vorticis Monitor (installer/vorticis.ico e PNGs).

Inspirado na família Vorticis (espiral/vórtice), mas desenhado do zero e diferenciado:
bloco arredondado em degradê azul-petróleo, vórtice de 7 braços em branco/ciano e um
selo com a linha de pulso (monitoramento) no lugar da sigla dos módulos.
Uso: python3 -I installer/icone/gerar_icone.py   (requer Pillow)
"""
import math
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter

PASTA = Path(__file__).resolve().parent
SS, LADO = 4, 512
TOPO, BASE = (8, 47, 73), (13, 116, 144)       # degradê do bloco
CIANO, BRANCO = (103, 232, 249), (255, 255, 255)
SELO, PULSO = (16, 185, 129), (255, 255, 255)


def bloco(t: int) -> Image.Image:
    grad = Image.new("RGBA", (t, t))
    px = grad.load()
    for y in range(t):
        for x in range(t):
            k = (x + y) / (2 * t)
            px[x, y] = tuple(round(TOPO[i] + (BASE[i] - TOPO[i]) * k) for i in range(3)) + (255,)
    mask = Image.new("L", (t, t), 0)
    ImageDraw.Draw(mask).rounded_rectangle((0, 0, t - 1, t - 1), radius=round(t * 0.22), fill=255)
    out = Image.new("RGBA", (t, t), (0, 0, 0, 0))
    out.paste(grad, (0, 0), mask)
    return out


def vortice(t: int) -> Image.Image:
    """Vórtice: braços em faixas contínuas que afinam para o centro (desenho próprio)."""
    img = Image.new("RGBA", (t, t), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    cx, cy = t * 0.45, t * 0.45
    braços, n = 6, 160
    for b in range(braços):
        fase = 2 * math.pi * b / braços
        esq, dir_ = [], []
        for i in range(n):
            s = i / (n - 1)
            r = t * (0.035 + 0.335 * s)
            ang = fase + 1.55 * math.pi * s
            x, y = cx + r * math.cos(ang), cy + r * math.sin(ang)
            # tangente aproximada -> normal
            dr, da = t * 0.335, 1.55 * math.pi
            tx = dr * math.cos(ang) - r * da * math.sin(ang)
            ty = dr * math.sin(ang) + r * da * math.cos(ang)
            norma = math.hypot(tx, ty)
            nx, ny = -ty / norma, tx / norma
            meia = t * (0.004 + 0.034 * math.sin(math.pi * min(1, s * 1.15)) ** 0.8)
            esq.append((x + nx * meia, y + ny * meia))
            dir_.append((x - nx * meia, y - ny * meia))
        cor = BRANCO if b % 2 == 0 else CIANO
        d.polygon(esq + dir_[::-1], fill=cor + (255,))
    d.ellipse((cx - t * 0.05, cy - t * 0.05, cx + t * 0.05, cy + t * 0.05), fill=BRANCO + (255,))
    return img


def selo(t: int) -> Image.Image:
    img = Image.new("RGBA", (t, t), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    diam = t * 0.40
    x0, y0 = t - diam - t * 0.04, t - diam - t * 0.04
    borda = t * 0.022
    d.ellipse((x0 - borda, y0 - borda, x0 + diam + borda, y0 + diam + borda), fill=TOPO + (255,))
    d.ellipse((x0, y0, x0 + diam, y0 + diam), fill=SELO + (255,))
    # linha de pulso (monitor)
    m = lambda fx, fy: (x0 + diam * fx, y0 + diam * fy)
    linha = [m(0.14, 0.55), m(0.36, 0.55), m(0.46, 0.30), m(0.58, 0.78), m(0.68, 0.48), m(0.86, 0.48)]
    d.line(linha, fill=PULSO + (255,), width=round(t * 0.032), joint="curve")
    return img


def icone() -> Image.Image:
    t = LADO * SS
    img = bloco(t)
    v = vortice(t)
    sombra = Image.new("RGBA", (t, t), (0, 0, 0, 0))
    sombra.putalpha(v.getchannel("A").point(lambda a: a // 3))
    img.alpha_composite(sombra.filter(ImageFilter.GaussianBlur(t * 0.01)), (round(t * 0.008), round(t * 0.012)))
    img.alpha_composite(v)
    img.alpha_composite(selo(t))
    return img.resize((LADO, LADO), Image.LANCZOS)


if __name__ == "__main__":
    ico = icone()
    ico.save(PASTA / "VorticisMonitor-512.png")
    ico.resize((256, 256), Image.LANCZOS).save(PASTA / "VorticisMonitor-256.png")
    ico.save(PASTA.parent / "vorticis.ico", sizes=[(16, 16), (20, 20), (24, 24), (32, 32), (40, 40), (48, 48), (64, 64), (128, 128), (256, 256)])
    print("ícone gerado")
