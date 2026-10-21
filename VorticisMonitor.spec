# -*- mode: python ; coding: utf-8 -*-
# Gera o executavel portatil (onefile, sem console):
#   pyinstaller --noconfirm VorticisMonitor.spec
# Saida: dist/VorticisMonitor.exe (no Windows). Use build.bat para montar a pasta portatil.

from PyInstaller.utils.hooks import collect_data_files, collect_submodules

a = Analysis(
    ["run_monitor.py"],
    pathex=["."],
    binaries=[],
    datas=[
        # Recursos da interface (icones, logos). A GUI procura em
        # Path(sys._MEIPASS) / "monitor_notas" / "recursos".
        ("monitor_notas/recursos", "monitor_notas/recursos"),
        *collect_data_files("fpdf"),
        *collect_data_files("brazilfiscalreport"),
    ],
    hiddenimports=[
        "monitor_notas.gui",
        "monitor_notas.exportar",
        "monitor_notas.pdf",
        "monitor_notas.winstore",
        "openpyxl",
        "lxml._elementpath",
        *collect_submodules("brazilfiscalreport"),
    ],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[],
    noarchive=False,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.datas,
    [],
    name="VorticisMonitor",
    icon="monitor_notas/recursos/icone.ico",
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=False,
    upx_exclude=[],
    runtime_tmpdir=None,
    console=False,
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)
