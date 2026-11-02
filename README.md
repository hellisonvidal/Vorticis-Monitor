# Vorticis Monitor

Programa para Windows de **consulta e monitoramento de documentos fiscais** — **NF-e, NFC-e, CT-e, MDF-e e NFS-e (ABRASF e Padrão Nacional)** — com janela própria, instalado sem exigir administrador, e integração com o ERP por API local.

Arquitetura completa (ERD, motor de regras, máquina de estados, endpoints): [`docs/ARQUITETURA.md`](docs/ARQUITETURA.md).

## Funcionalidades
- **Menu por tipo de documento** — NF-e/NFC-e, NFS-e e Transporte (CT-e/MDF-e) —, cada um com o seletor destacado **Emitidos | Recebidos | Todos** e a quantidade do período.
- **Consulta de documentos** com **filtro de período (De/Até)** em todas as telas, atalhos (hoje, 7/30 dias, mês, mês anterior, ano), busca por número/chave/CNPJ/nome, modelo e situação.
- **Relatórios** (relação de documentos, resumo de tributos, por fornecedor/cliente, por CFOP, manifestação das NF-e recebidas) exportados em **PDF, Excel XLSX, Excel 97-2003 XLS e CSV**, com escolha dos **campos exibidos** (botão Campos; a escolha fica salva por relatório).
- **Salvar senha e login automático** (opcional, na tela de entrada): o computador guarda só uma credencial assinada de 180 dias, invalidada se a senha for trocada.
- **Exportação selecionada:** marque as notas (ou todas do filtro) e exporte em **XML, PDF ou XML + PDF** (ZIP organizado por modelo e movimento). PDF do DANFE/DACTE/DAMDFE/DANFSe com código de barras.
- **Recebidos:** busca automática na SEFAZ (Distribuição DF-e), manifestação do destinatário assinada e download do XML completo. Convive com outros monitores de notas que usam o mesmo CNPJ: respeita a regra de 1 hora da SEFAZ, detecta o bloqueio 656 causado por outro sistema e amplia o intervalo automaticamente.
- **NFS-e Nacional (ADN):** busca automática das NFS-e prestadas e tomadas no Ambiente de Dados Nacional (`GET /contribuintes/DFe/{NSU}?lote=true&cnpjConsulta=`), com o certificado da própria empresa ou por **procuração eletrônica** — o contador cadastra o próprio certificado em Configurações › Procuradores e escolhe, em cada empresa, "Procuração eletrônica".
- **NF-e pelo certificado do contador:** a SEFAZ não aceita procuração para NF-e; com o certificado do procurador o monitor baixa as NF-e em que ele consta como **autorizado a obter o XML** (`autXML`) e importa só as das empresas cadastradas (Configurações › Procuradores, com a UF do procurador).
- **Backup automático** (Configurações › Backup): diário ou semanal para pendrive, rede ou pasta da nuvem — cópia consistente do banco em ZIP (mantém as últimas N) e espelho incremental dos XMLs, que nunca são apagados no destino. As notas ficam guardadas no monitor sem prazo, mesmo depois dos 90 dias de disponibilidade na SEFAZ.
- **Situação na SEFAZ** e **conferência fiscal** de cada documento (chave, CNPJ/IE, CFOP, NCM/CEST, ICMS, ST, DIFAL, IPI, PIS/COFINS, retenções, contingência).
- **Cadastro da empresa só pelo CNPJ:** dados da Receita Federal (BrasilAPI, CNPJ.ws, ReceitaWS) e Inscrição Estadual pelo Sintegra (via CNPJ.ws) ou pela **SEFAZ da UF com o certificado digital** — inclusive cadastrando direto a partir do arquivo do certificado.
- **Certificado digital** escolhido entre os **instalados no Windows** ou pelo arquivo .pfx. Chave exportável → cópia criptografada no cofre (AES-256-GCM + DPAPI); não exportável → o monitor assina pelo próprio Windows (CNG), sem copiar a chave.

## Downloads (Releases / pasta `release/`)
São **duas versões separadas** — escolha uma:

| Versão | Arquivo | Para quê |
|---|---|---|
| **Instalador** | `instalador/VorticisMonitor-Setup-1.5.0.exe` | Instala o programa para o usuário atual — **não exige administrador** —, com atalhos, inicialização automática opcional e desinstalador. Dados em `%LOCALAPPDATA%\Vorticis\Monitor`. |
| **Portátil** | `portatil/VorticisMonitor-Portatil-1.5.0.exe` | Arquivo único, sem instalação: coloque numa pasta ou pendrive e abra. Dados na pasta `dados` criada ao lado do arquivo. |

`SHA256SUMS.txt` traz os hashes para conferência da integridade.

O Vorticis Monitor abre em **janela própria do Windows** (WebView2), sem usar o navegador. Requisito: Microsoft Edge WebView2 Runtime, já incluído no Windows 10/11 atualizados. No primeiro uso, crie o administrador do programa, cadastre a empresa e **inclua o certificado** (instalado no Windows ou arquivo .pfx). Fechar a janela encerra o programa; abrir o atalho de novo traz a janela já aberta.

Dados: `%LOCALAPPDATA%\Vorticis\Monitor` (instalado) ou `dados\` ao lado do executável (portátil). Integração com ERP pela API local `http://127.0.0.1:8765/api/v1` (`X-API-Key` em Configurações › Integração) ou gravando XMLs na pasta `importar`.

Linha de comando: `VorticisMonitor.exe run | reset-senha LOGIN SENHA | version` · opções `--portatil`, `--minimizado`, `--sem-janela` (somente a API, sem interface). Opcional, como administrador: `service install|uninstall|start|stop` para rodar como serviço do Windows.

**Modo portátil** é ativado quando o nome do executável contém "portatil", quando existe o arquivo `vorticis.portatil` ao lado dele ou com `--portatil`. Nesse modo a chave que cifra o certificado fica na própria pasta (sem DPAPI), permitindo levá-la para outro computador — proteja a pasta.

## Integração pela rede
Para ERPs em outros computadores, marque **"Integração com ERP pela rede"** no instalador (a API passa a escutar em `0.0.0.0:8765`; o firewall pode exigir liberação da porta). HTTPS: informe `tls_cert` e `tls_key` em `config.json`.

## Compilar
Requer Go 1.24+ e NSIS 3 (`apt install nsis`):
```bash
./build.sh
```
Gera `dist/instalador/VorticisMonitor-Setup-<versão>.exe`, `dist/portatil/VorticisMonitor-Portatil-<versão>.exe` e `dist/SHA256SUMS.txt`.

## Testes e amostras
`go test ./...` · `python3 samples/gerar_amostras.py` gera XMLs fictícios (DIFAL correto, ST com erros, contingência SVC incorreta, devolução inválida, entrada para conciliação, NFS-e com retenções, CT-e).

## Aviso
A conferência fiscal é de apoio; tabelas de alíquotas, MVA e URLs de webservices são parâmetros de referência a revisar conforme a legislação vigente.
