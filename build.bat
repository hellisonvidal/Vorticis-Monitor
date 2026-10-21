@echo off
chcp 65001 >nul
setlocal EnableExtensions
cd /d "%~dp0"

echo ==============================================
echo  Vorticis Monitor - gerar pacote Windows
echo ==============================================
echo.

set "PY="
where py >nul 2>nul
if not errorlevel 1 set "PY=py -3"
if defined PY goto achou_python
where python >nul 2>nul
if not errorlevel 1 set "PY=python"
if not defined PY goto sem_python
:achou_python

echo [1/5] Preparando ambiente virtual .venv...
if not exist ".venv\Scripts\python.exe" (
    %PY% -m venv .venv
    if errorlevel 1 goto erro
)

echo [2/5] Instalando dependencias e PyInstaller...
".venv\Scripts\python.exe" -m pip install --upgrade pip
if errorlevel 1 goto erro
".venv\Scripts\python.exe" -m pip install -r requirements.txt pyinstaller
if errorlevel 1 goto erro

echo [3/5] Gerando VorticisMonitor.exe com PyInstaller...
".venv\Scripts\pyinstaller.exe" --noconfirm VorticisMonitor.spec
if errorlevel 1 goto erro
if not exist "dist\VorticisMonitor.exe" goto erro

echo [4/5] Montando a pasta portatil dist\VorticisMonitor...
set "PASTA=dist\VorticisMonitor"
if exist "%PASTA%" rmdir /s /q "%PASTA%"
mkdir "%PASTA%\certificados"
if errorlevel 1 goto erro
copy /y "dist\VorticisMonitor.exe" "%PASTA%\" >nul
if errorlevel 1 goto erro
copy /y "config.exemplo.toml" "%PASTA%\" >nul
if errorlevel 1 goto erro
copy /y "agendar_consulta.bat" "%PASTA%\" >nul
if errorlevel 1 goto erro

> "%PASTA%\certificados\LEIA-ME.txt" (
echo CERTIFICADOS DIGITAIS A1 ^(.pfx^)
echo.
echo Coloque aqui o arquivo .pfx de cada empresa, por exemplo empresa.pfx
echo No config.toml, aponte para ele no campo certificado:
echo    certificado = "certificados/empresa.pfx"
echo.
echo Alternativa: instale o certificado no Windows e cadastre-o pela interface.
echo.
echo Nao compartilhe esta pasta: ela contem a chave privada do certificado.
)

> "%PASTA%\LEIA-ME.txt" (
echo VORTICIS MONITOR - USO
echo ======================
echo.
echo 1. Copie config.exemplo.toml para config.toml (nesta mesma pasta) e ajuste os dados.
echo 2. Abra o VorticisMonitor.exe:
echo    - aba Empresas: cadastre a empresa pelo certificado instalado no Windows
echo      ou pelo arquivo .pfx ^(pasta certificados^);
echo    - Consultar: busca os documentos da empresa selecionada;
echo    - Consultas: filtre, selecione e exporte para XML, ZIP, PDF ou Excel.
echo 3. Consulta automatica com o programa fechado: execute agendar_consulta.bat
echo    ^(roda a cada 60 minutos^). Para remover a tarefa: agendar_consulta.bat remover
echo.
echo Dados ficam nesta pasta: dados\monitor.db ^(banco^), dados\xml ^(XMLs^) e dados\monitor.log.
echo Faca backup desta pasta. Para usar em outro computador ou usuario, copie tudo e
echo digite a senha do certificado novamente.
echo.
echo Este monitor apenas consulta e extrai documentos. Ele nao manifesta notas.
)

echo [5/5] Concluido.
echo.
echo ==============================================
echo Pacote pronto em: %CD%\%PASTA%
echo Copie a pasta VorticisMonitor para onde quiser.
echo ==============================================
pause
exit /b 0

:erro
echo.
echo *** Falhou. Veja a mensagem de erro acima. ***
pause
exit /b 1

:sem_python
echo Python nao encontrado. Instale o Python 3.11 ou superior em python.org
echo e marque a opcao "Add python.exe to PATH" durante a instalacao.
pause
exit /b 1
