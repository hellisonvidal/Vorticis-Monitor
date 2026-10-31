# Vorticis Monitor

Monitor fiscal em tempo real para **NF-e, NFC-e, CT-e, MDF-e, NFS-e (ABRASF e Padrão Nacional)** e pré-escrituração **SPED**. Programa para Windows com janela própria, instalado sem exigir administrador, que se integra a ERPs e sistemas de escrituração via API REST.

Arquitetura completa (ERD, motor de regras, máquina de estados, endpoints): [`docs/ARQUITETURA.md`](docs/ARQUITETURA.md).

## Funcionalidades
- **Entradas:** captura pela Distribuição DF-e (NSU, regra de 1 h), manifestação do destinatário assinada (XMLDSig), validação de chave/CNPJ/IE, conciliação XML × pedido de compra × ordem de recebimento, devoluções x documento original.
- **Saídas:** pré-validação antes da SEFAZ (`POST /api/v1/validacao/pre-emissao` → 422 se houver erro), CFOP/NCM/CEST/CST/CSOSN, auditoria de contingência (SVC-AN, SVC-RS, FS-DA, off-line).
- **Serviços/Transporte:** ISS (alíquota, local de incidência LC 116 art. 3º), retenções IRRF/PIS/COFINS/CSLL/INSS, CT-e e MDF-e.
- **Impostos complexos:** DIFAL (base única/dupla, FCP, entrada uso/consumo/ativo), ICMS-ST (MVA ajustada, MVA defasada), IPI, PIS/COFINS por regime, IBS/CBS 2026.
- **Painel:** grid em tempo real (SSE), central de rejeições em linguagem clara com correções sugeridas, alertas de prazos (cancelamento, manifestação, contingência, certificado, MDF-e), score fiscal.
- **Relatórios:** livros por CFOP/CST, apuração ICMS/ST/DIFAL por UF, PIS/COFINS, retenções (Reinf), inconsistências, rejeições, manifestação, contingência, conciliação, EFD ICMS/IPI, EFD-Contribuições, ZIP de XML + DANFE.
- **Certificado digital A1** solicitado no primeiro uso, armazenado cifrado (AES-256-GCM + DPAPI).

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
Validações e arquivos SPED são de conferência/pré-escrituração; tabelas de alíquotas, MVA e URLs de webservices são parâmetros de referência a revisar conforme a legislação vigente.
