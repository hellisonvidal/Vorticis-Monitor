@echo off
chcp 65001 >nul
setlocal EnableExtensions

rem Cria (ou remove, com o parametro "remover") a tarefa no Agendador de Tarefas
rem que roda "MonitorNotas.exe executar" a cada 60 minutos para o usuario atual.
rem O caminho e absoluto (via %~dp0). O programa resolve config, banco e certificados
rem a partir da pasta do proprio .exe, entao nao precisa de "Iniciar em".

set "NOME=MonitorNotas - consulta"
set "EXE=%~dp0MonitorNotas.exe"

if /i "%~1"=="remover" goto remover

if not exist "%EXE%" (
    echo Nao encontrei "%EXE%".
    echo Coloque este arquivo na mesma pasta do MonitorNotas.exe.
    goto fim_erro
)

echo Criando a tarefa "%NOME%" para rodar a cada 60 minutos...
schtasks /Create /F /TN "%NOME%" /SC MINUTE /MO 60 /TR "\"%EXE%\" executar"
if errorlevel 1 goto fim_erro

echo.
echo Tarefa criada. A consulta roda de hora em hora, mesmo com o programa fechado.
echo Para remover depois: agendar_consulta.bat remover
goto fim

:remover
echo Removendo a tarefa "%NOME%"...
schtasks /Delete /F /TN "%NOME%"
if errorlevel 1 goto fim_erro
echo Tarefa removida.
goto fim

:fim_erro
echo.
echo *** Nao foi possivel concluir. Veja a mensagem acima. ***
pause
exit /b 1

:fim
pause
exit /b 0
