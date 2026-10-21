# -*- mode: python ; coding: utf-8 -*-
# Gera o executavel portatil (onefile, sem console):
#   pyinstaller --noconfirm MonitorNotas.spec
# Saida: dist/MonitorNotas.exe (no Windows). Use build.bat para montar a pasta portatil.

a = Analysis(
    ["run_monitor.py"],
    pathex=["."],
    binaries=[],
    datas=[],
    hiddenimports=[
        "monitor_notas.gui",
        "monitor_notas.exportar",
        "openpyxl",
        "lxml._elementpath",
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
    name="MonitorNotas",
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
