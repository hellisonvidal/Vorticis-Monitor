"""Pasta base portátil: ao lado do .exe (PyInstaller) ou a pasta do projeto em desenvolvimento."""
from __future__ import annotations

import sys
from pathlib import Path


def pasta_base() -> Path:
    if getattr(sys, "frozen", False):
        return Path(sys.executable).resolve().parent
    return Path(__file__).resolve().parent.parent


def resolver(caminho: str | Path) -> Path:
    """Caminhos relativos no config são relativos à pasta base (permite mover a pasta inteira)."""
    p = Path(caminho)
    return p if p.is_absolute() else pasta_base() / p
