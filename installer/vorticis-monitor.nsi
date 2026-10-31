; Instalador do Vorticis Monitor (NSIS 3) — instalação por usuário, sem privilégio de administrador
; Compilar: makensis -DVERSAO=1.0.0 installer/vorticis-monitor.nsi
Unicode true
!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"

!ifndef VERSAO
  !define VERSAO "1.0.0"
!endif
!define APP       "Vorticis Monitor"
!define EXE       "VorticisMonitor.exe"
!define PORTA     "8765"
!define DADOS     "$LOCALAPPDATA\Vorticis\Monitor"
!define REGUNINST "Software\Microsoft\Windows\CurrentVersion\Uninstall\VorticisMonitor"

Name "${APP} ${VERSAO}"
OutFile "..\dist\VorticisMonitor-Setup-${VERSAO}.exe"
InstallDir "$LOCALAPPDATA\Programs\Vorticis\Monitor"
InstallDirRegKey HKCU "Software\Vorticis\Monitor" "InstallDir"
RequestExecutionLevel user
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
!define MUI_WELCOMEPAGE_TEXT "Monitor fiscal em tempo real para NF-e, NFC-e, CT-e, MDF-e, NFS-e e SPED.$\r$\n$\r$\nA instalação é feita apenas para o seu usuário e não exige permissão de administrador.$\r$\n$\r$\nO programa abre em janela própria (não usa o navegador). Após a instalação, cadastre a empresa e inclua o certificado digital A1 (.pfx).$\r$\n$\r$\nRequisito: Microsoft Edge WebView2 Runtime (já incluído no Windows 10 e 11 atualizados)."
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Abrir o ${APP} agora"
!define MUI_FINISHPAGE_RUN_FUNCTION Iniciar

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
  SetShellVarContext current
FunctionEnd

Function Iniciar
  Exec '"$INSTDIR\${EXE}"'
FunctionEnd

Section "!${APP} (obrigatório)" SecApp
  SectionIn RO
  ; Atualização: encerra a instância do próprio usuário antes de substituir o executável
  nsExec::Exec 'taskkill /IM ${EXE} /F'
  Sleep 1000
  SetOutPath "$INSTDIR"
  File "..\dist\${EXE}"
  File "vorticis.ico"
  File "LICENCA.txt"
  File "..\README.md"
  SetOutPath "$INSTDIR\docs"
  File "..\docs\ARQUITETURA.md"
  SetOutPath "$INSTDIR\exemplos"
  File "..\samples\*.xml"

  ; Dados (banco, XMLs, logs, chave-mestra) no perfil do usuário
  CreateDirectory "${DADOS}"
  CreateDirectory "${DADOS}\importar"

  WriteUninstaller "$INSTDIR\Desinstalar.exe"
  WriteRegStr HKCU "Software\Vorticis\Monitor" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${REGUNINST}" "DisplayName" "${APP}"
  WriteRegStr HKCU "${REGUNINST}" "DisplayVersion" "${VERSAO}"
  WriteRegStr HKCU "${REGUNINST}" "Publisher" "Vorticis"
  WriteRegStr HKCU "${REGUNINST}" "DisplayIcon" "$INSTDIR\vorticis.ico"
  WriteRegStr HKCU "${REGUNINST}" "UninstallString" '"$INSTDIR\Desinstalar.exe"'
  WriteRegStr HKCU "${REGUNINST}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${REGUNINST}" "URLInfoAbout" "http://localhost:${PORTA}/"
  WriteRegDWORD HKCU "${REGUNINST}" "NoModify" 1
  WriteRegDWORD HKCU "${REGUNINST}" "NoRepair" 1
SectionEnd

Section "Atalhos no Menu Iniciar e Área de Trabalho" SecAtalhos
  CreateDirectory "$SMPROGRAMS\Vorticis Monitor"
  CreateShortcut "$SMPROGRAMS\Vorticis Monitor\Vorticis Monitor.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\vorticis.ico"
  CreateShortcut "$SMPROGRAMS\Vorticis Monitor\Pasta de importação de XML.lnk" "${DADOS}\importar"
  CreateShortcut "$SMPROGRAMS\Vorticis Monitor\Desinstalar.lnk" "$INSTDIR\Desinstalar.exe"
  CreateShortcut "$DESKTOP\Vorticis Monitor.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\vorticis.ico"
SectionEnd

Section "Iniciar automaticamente ao entrar no Windows" SecInicio
  ; Sem serviço: inicia minimizado no logon do usuário (captura DF-e, alertas e consultas em segundo plano)
  CreateShortcut "$SMSTARTUP\Vorticis Monitor.lnk" "$INSTDIR\${EXE}" "--minimizado" "$INSTDIR\vorticis.ico" 0 SW_SHOWMINIMIZED
SectionEnd

Section /o "Integração com ERP pela rede (API em outros computadores)" SecRede
  ; Escuta em todas as interfaces. O Windows pode pedir para liberar a porta no firewall.
  ${IfNot} ${FileExists} "${DADOS}\config.json"
    FileOpen $0 "${DADOS}\config.json" w
    FileWrite $0 '{"host":"0.0.0.0","porta":${PORTA}}'
    FileClose $0
  ${EndIf}
SectionEnd

LangString DESC_App ${LANG_PORTUGUESEBR} "Executável, painel web, documentação e XMLs de exemplo."
LangString DESC_Atalhos ${LANG_PORTUGUESEBR} "Atalhos para abrir o programa."
LangString DESC_Inicio ${LANG_PORTUGUESEBR} "Abre o programa minimizado sempre que você entrar no Windows, mantendo a captura de XML e os alertas ativos."
LangString DESC_Rede ${LANG_PORTUGUESEBR} "Libera a API do monitor para ERPs em outros computadores (porta ${PORTA}). Se o firewall bloquear, peça ao suporte para liberar a porta."
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecApp} $(DESC_App)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecAtalhos} $(DESC_Atalhos)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecInicio} $(DESC_Inicio)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecRede} $(DESC_Rede)
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Function un.onInit
  SetShellVarContext current
FunctionEnd

Section "Uninstall"
  nsExec::Exec 'taskkill /IM ${EXE} /F'
  Sleep 1000
  Delete "$DESKTOP\Vorticis Monitor.lnk"
  Delete "$SMSTARTUP\Vorticis Monitor.lnk"
  RMDir /r "$SMPROGRAMS\Vorticis Monitor"
  RMDir /r "$INSTDIR\docs"
  RMDir /r "$INSTDIR\exemplos"
  Delete "$INSTDIR\${EXE}"
  Delete "$INSTDIR\vorticis.ico"
  Delete "$INSTDIR\LICENCA.txt"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\Desinstalar.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "${REGUNINST}"
  DeleteRegKey HKCU "Software\Vorticis\Monitor"
  MessageBox MB_YESNO|MB_ICONQUESTION "Remover também os dados (banco, XMLs armazenados, certificados e logs)?$\r$\n$\r$\nATENÇÃO: os XMLs fiscais devem ser guardados por 5 anos. Faça backup de$\r$\n${DADOS} antes de remover." /SD IDNO IDNO manter
    RMDir /r "${DADOS}"
  manter:
SectionEnd
