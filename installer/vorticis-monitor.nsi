; Instalador do Vorticis Monitor (NSIS 3)
; Compilar: makensis -DVERSAO=1.0.0 installer/vorticis-monitor.nsi
Unicode true
!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"

!ifndef VERSAO
  !define VERSAO "1.0.0"
!endif
!define APP      "Vorticis Monitor"
!define SERVICO  "VorticisMonitor"
!define EXE      "vorticis-monitor.exe"
!define PORTA    "8765"
!define REGUNINST "Software\Microsoft\Windows\CurrentVersion\Uninstall\VorticisMonitor"

Name "${APP} ${VERSAO}"
OutFile "..\dist\VorticisMonitor-Setup-${VERSAO}.exe"
InstallDir "$PROGRAMFILES64\Vorticis\Monitor"
InstallDirRegKey HKLM "Software\Vorticis\Monitor" "InstallDir"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
BrandingText "Vorticis Monitor Fiscal"
VIProductVersion "${VERSAO}.0"
VIAddVersionKey "ProductName" "${APP}"
VIAddVersionKey "FileDescription" "Instalador do Vorticis Monitor Fiscal"
VIAddVersionKey "FileVersion" "${VERSAO}"
VIAddVersionKey "ProductVersion" "${VERSAO}"
VIAddVersionKey "CompanyName" "Vorticis"
VIAddVersionKey "LegalCopyright" "(c) 2026 Vorticis"

!define MUI_ICON "vorticis.ico"
!define MUI_UNICON "vorticis.ico"
!define MUI_ABORTWARNING
!define MUI_WELCOMEPAGE_TITLE "Bem-vindo ao ${APP}"
!define MUI_WELCOMEPAGE_TEXT "Monitor fiscal em tempo real para NF-e, NFC-e, CT-e, MDF-e, NFS-e e SPED.$\r$\n$\r$\nO ${APP} será instalado como serviço do Windows e o painel ficará disponível no navegador em http://localhost:${PORTA}.$\r$\n$\r$\nApós a instalação, cadastre a empresa e inclua o certificado digital A1 (.pfx) para habilitar as consultas à SEFAZ."
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Abrir o painel do ${APP} (cadastro da empresa e do certificado digital)"
!define MUI_FINISHPAGE_RUN_FUNCTION AbrirPainel

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "LICENCA.txt"
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "PortugueseBR"

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP "O ${APP} requer Windows 64 bits."
    Abort
  ${EndIf}
  SetRegView 64
FunctionEnd

Function AbrirPainel
  ExecShell "open" "http://localhost:${PORTA}/"
FunctionEnd

Section "!${APP} (obrigatório)" SecApp
  SectionIn RO
  ; Atualização: para o serviço antes de substituir o executável
  nsExec::Exec '"$INSTDIR\${EXE}" service stop'
  Sleep 2000
  SetOutPath "$INSTDIR"
  File "..\dist\${EXE}"
  File "vorticis.ico"
  File "LICENCA.txt"
  File "..\README.md"
  SetOutPath "$INSTDIR\docs"
  File "..\docs\ARQUITETURA.md"
  SetOutPath "$INSTDIR\exemplos"
  File "..\samples\*.xml"

  ; Diretório de dados (banco, XMLs, logs, chave-mestra) restrito a SYSTEM e Administradores
  CreateDirectory "$COMMONPROGRAMDATA\Vorticis\Monitor"
  CreateDirectory "$COMMONPROGRAMDATA\Vorticis\Monitor\importar"
  nsExec::Exec 'icacls "$COMMONPROGRAMDATA\Vorticis\Monitor" /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F"'

  WriteUninstaller "$INSTDIR\Desinstalar.exe"
  WriteRegStr HKLM "Software\Vorticis\Monitor" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "${REGUNINST}" "DisplayName" "${APP}"
  WriteRegStr HKLM "${REGUNINST}" "DisplayVersion" "${VERSAO}"
  WriteRegStr HKLM "${REGUNINST}" "Publisher" "Vorticis"
  WriteRegStr HKLM "${REGUNINST}" "DisplayIcon" "$INSTDIR\vorticis.ico"
  WriteRegStr HKLM "${REGUNINST}" "UninstallString" '"$INSTDIR\Desinstalar.exe"'
  WriteRegStr HKLM "${REGUNINST}" "URLInfoAbout" "http://localhost:${PORTA}/"
  WriteRegDWORD HKLM "${REGUNINST}" "NoModify" 1
  WriteRegDWORD HKLM "${REGUNINST}" "NoRepair" 1
SectionEnd

Section "Serviço do Windows (iniciar com o sistema)" SecServico
  DetailPrint "Registrando o serviço ${SERVICO}..."
  nsExec::ExecToLog '"$INSTDIR\${EXE}" service install'
  nsExec::ExecToLog '"$INSTDIR\${EXE}" service start'
SectionEnd

Section "Atalhos no Menu Iniciar e Área de Trabalho" SecAtalhos
  WriteINIStr "$INSTDIR\Vorticis Monitor.url" "InternetShortcut" "URL" "http://localhost:${PORTA}/"
  WriteINIStr "$INSTDIR\Vorticis Monitor.url" "InternetShortcut" "IconFile" "$INSTDIR\vorticis.ico"
  WriteINIStr "$INSTDIR\Vorticis Monitor.url" "InternetShortcut" "IconIndex" "0"
  CreateDirectory "$SMPROGRAMS\Vorticis Monitor"
  CreateShortcut "$SMPROGRAMS\Vorticis Monitor\Vorticis Monitor.lnk" "$INSTDIR\Vorticis Monitor.url" "" "$INSTDIR\vorticis.ico"
  CreateShortcut "$SMPROGRAMS\Vorticis Monitor\Pasta de importação de XML.lnk" "$COMMONPROGRAMDATA\Vorticis\Monitor\importar"
  CreateShortcut "$SMPROGRAMS\Vorticis Monitor\Desinstalar.lnk" "$INSTDIR\Desinstalar.exe"
  CreateShortcut "$DESKTOP\Vorticis Monitor.lnk" "$INSTDIR\Vorticis Monitor.url" "" "$INSTDIR\vorticis.ico"
SectionEnd

Section /o "Acesso pela rede/navegador de outros computadores" SecRede
  ; Escuta em todas as interfaces e libera a porta no firewall (modo servidor web)
  ${IfNot} ${FileExists} "$COMMONPROGRAMDATA\Vorticis\Monitor\config.json"
    FileOpen $0 "$COMMONPROGRAMDATA\Vorticis\Monitor\config.json" w
    FileWrite $0 '{"host":"0.0.0.0","porta":${PORTA}}'
    FileClose $0
  ${EndIf}
  nsExec::ExecToLog 'netsh advfirewall firewall add rule name="Vorticis Monitor" dir=in action=allow protocol=TCP localport=${PORTA}'
  nsExec::ExecToLog '"$INSTDIR\${EXE}" service stop'
  Sleep 1500
  nsExec::ExecToLog '"$INSTDIR\${EXE}" service start'
SectionEnd

LangString DESC_App ${LANG_PORTUGUESEBR} "Executável, painel web, documentação e XMLs de exemplo."
LangString DESC_Servico ${LANG_PORTUGUESEBR} "Executa o monitor em segundo plano (captura DF-e, alertas, consultas SEFAZ) mesmo sem usuário logado."
LangString DESC_Atalhos ${LANG_PORTUGUESEBR} "Atalhos para abrir o painel no navegador."
LangString DESC_Rede ${LANG_PORTUGUESEBR} "Permite acessar o painel por http://NOME-DO-SERVIDOR:${PORTA} na rede local (libera a porta no firewall)."
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecApp} $(DESC_App)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecServico} $(DESC_Servico)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecAtalhos} $(DESC_Atalhos)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecRede} $(DESC_Rede)
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Function un.onInit
  SetRegView 64
FunctionEnd

Section "Uninstall"
  nsExec::ExecToLog '"$INSTDIR\${EXE}" service stop'
  Sleep 2000
  nsExec::ExecToLog '"$INSTDIR\${EXE}" service uninstall'
  nsExec::ExecToLog 'netsh advfirewall firewall delete rule name="Vorticis Monitor"'
  Delete "$DESKTOP\Vorticis Monitor.lnk"
  RMDir /r "$SMPROGRAMS\Vorticis Monitor"
  RMDir /r "$INSTDIR\docs"
  RMDir /r "$INSTDIR\exemplos"
  Delete "$INSTDIR\*.*"
  RMDir "$INSTDIR"
  RMDir "$PROGRAMFILES64\Vorticis"
  DeleteRegKey HKLM "${REGUNINST}"
  DeleteRegKey HKLM "Software\Vorticis\Monitor"
  MessageBox MB_YESNO|MB_ICONQUESTION "Remover também os dados (banco, XMLs armazenados, certificados e logs)?$\r$\n$\r$\nATENÇÃO: os XMLs fiscais devem ser guardados por 5 anos. Faça backup de$\r$\n$COMMONPROGRAMDATA\Vorticis\Monitor antes de remover." /SD IDNO IDNO manter
    RMDir /r "$COMMONPROGRAMDATA\Vorticis\Monitor"
  manter:
SectionEnd
