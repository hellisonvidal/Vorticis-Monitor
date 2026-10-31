# Vorticis Monitor

Programa para Windows de **consulta e monitoramento de documentos fiscais** — **NF-e, NFC-e, CT-e, MDF-e e NFS-e (ABRASF e Padrão Nacional)** — com janela própria, instalado sem exigir administrador, e integração com o ERP por API local.

Arquitetura completa (ERD, motor de regras, máquina de estados, endpoints): [`docs/ARQUITETURA.md`](docs/ARQUITETURA.md).

## Funcionalidades
- **Consulta de documentos** emitidos e recebidos com **filtro de período (De/Até)** em todas as telas, atalhos (hoje, 7/30 dias, mês, mês anterior, ano), busca por número/chave/CNPJ/nome, modelo, movimento e situação.
- **Exportação selecionada:** marque as notas (ou todas do filtro) e exporte em **XML, PDF ou XML + PDF** (ZIP organizado por modelo e movimento). PDF do DANFE/DACTE/DAMDFE/DANFSe com código de barras.
- **Recebidos:** busca automática na SEFAZ (Distribuição DF-e), manifestação do destinatário assinada e download do XML completo.
- **Situação na SEFAZ** e **conferência fiscal** de cada documento (chave, CNPJ/IE, CFOP, NCM/CEST, ICMS, ST, DIFAL, IPI, PIS/COFINS, retenções, contingência).
- **Cadastro da empresa só pelo CNPJ:** dados da Receita Federal (BrasilAPI, CNPJ.ws, ReceitaWS) e Inscrição Estadual pelo Sintegra (via CNPJ.ws) ou pela **SEFAZ da UF com o certificado digital** — inclusive cadastrando direto a partir do arquivo do certificado.
- **Certificado digital A1** guardado criptografado (AES-256-GCM + DPAPI).

## Downloads (Releases / pasta `release/`)
| Arquivo | Para quê |
|---|---|
| `VorticisMonitor-Setup-1.0.0.exe` | **Instalador** (recomendado): instala o programa no computador para o usuário atual — **não exige administrador** —, cria atalhos, inicialização automática opcional e desinstalador. |
| `VorticisMonitor-Portatil-1.0.0.zip` | **Portátil**: extraia em qualquer pasta ou pendrive e abra `VorticisMonitor-Portatil.exe`. Sem instalação; dados na subpasta `dados`. |
| `VorticisMonitor.exe` | Executável avulso (o mesmo programa, sem instalador). |
| `SHA256SUMS.txt` | Hashes para conferência da integridade. |

O Vorticis Monitor abre em **janela própria do Windows** (WebView2), sem usar o navegador. Requisito: Microsoft Edge WebView2 Runtime, já incluído no Windows 10/11 atualizados. No primeiro uso, crie o administrador do programa, cadastre a empresa e **inclua o certificado .pfx**. Fechar a janela encerra o programa; abrir o atalho de novo traz a janela já aberta.

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
Gera em `dist/`: `VorticisMonitor.exe`, `VorticisMonitor-Portatil-<versão>.zip`, `VorticisMonitor-Setup-<versão>.exe` e `SHA256SUMS.txt`.

## Testes e amostras
`go test ./...` · `python3 samples/gerar_amostras.py` gera XMLs fictícios (DIFAL correto, ST com erros, contingência SVC incorreta, devolução inválida, entrada para conciliação, NFS-e com retenções, CT-e).

## Aviso
A conferência fiscal é de apoio; tabelas de alíquotas, MVA e URLs de webservices são parâmetros de referência a revisar conforme a legislação vigente.
