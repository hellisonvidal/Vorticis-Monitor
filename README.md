# Vorticis Monitor

Monitor fiscal em tempo real para **NF-e, NFC-e, CT-e, MDF-e, NFS-e (ABRASF e Padrão Nacional)** e pré-escrituração **SPED**. Um único executável (Go) que roda como **serviço do Windows**, como **servidor web** (Linux/Docker) e se integra a ERPs e sistemas de escrituração via API REST.

Arquitetura completa (ERD, motor de regras, máquina de estados, endpoints): [`docs/ARQUITETURA.md`](docs/ARQUITETURA.md).

## Funcionalidades
- **Entradas:** captura pela Distribuição DF-e (NSU, regra de 1 h), manifestação do destinatário assinada (XMLDSig), validação de chave/CNPJ/IE, conciliação XML × pedido de compra × ordem de recebimento, devoluções x documento original.
- **Saídas:** pré-validação antes da SEFAZ (`POST /api/v1/validacao/pre-emissao` → 422 se houver erro), CFOP/NCM/CEST/CST/CSOSN, auditoria de contingência (SVC-AN, SVC-RS, FS-DA, off-line).
- **Serviços/Transporte:** ISS (alíquota, local de incidência LC 116 art. 3º), retenções IRRF/PIS/COFINS/CSLL/INSS, CT-e e MDF-e.
- **Impostos complexos:** DIFAL (base única/dupla, FCP, entrada uso/consumo/ativo), ICMS-ST (MVA ajustada, MVA defasada), IPI, PIS/COFINS por regime, IBS/CBS 2026.
- **Painel:** grid em tempo real (SSE), central de rejeições em linguagem clara com correções sugeridas, alertas de prazos (cancelamento, manifestação, contingência, certificado, MDF-e), score fiscal.
- **Relatórios:** livros por CFOP/CST, apuração ICMS/ST/DIFAL por UF, PIS/COFINS, retenções (Reinf), inconsistências, rejeições, manifestação, contingência, conciliação, EFD ICMS/IPI, EFD-Contribuições, ZIP de XML + DANFE.
- **Certificado digital A1** solicitado no primeiro uso, armazenado cifrado (AES-256-GCM + DPAPI).

## Instalação no Windows
1. Execute `VorticisMonitor-Setup-1.0.0.exe` (64 bits, como administrador).
2. Acesse `http://localhost:8765`, crie o administrador, cadastre a empresa e **inclua o certificado .pfx**.
3. Integre o ERP pela API (`X-API-Key` em Configurações › Integração) ou gravando XMLs em `C:\ProgramData\Vorticis\Monitor\importar`.

Linha de comando: `vorticis-monitor.exe run | service install|uninstall|start|stop | reset-senha LOGIN SENHA | version`.

## Modo servidor web
```bash
docker compose -f deploy/docker-compose.yml up -d
```
ou `VORTICIS_HOST=0.0.0.0 ./vorticis-monitor-linux-amd64 run` (ver `deploy/` para systemd e nginx).

## Compilar
Requer Go 1.24+ e NSIS 3 (`apt install nsis`):
```bash
./build.sh
```
Gera `dist/vorticis-monitor.exe`, `dist/vorticis-monitor-linux-amd64` e `dist/VorticisMonitor-Setup-<versão>.exe`.

## Testes e amostras
`go test ./...` · `python3 samples/gerar_amostras.py` gera XMLs fictícios (DIFAL correto, ST com erros, contingência SVC incorreta, devolução inválida, entrada para conciliação, NFS-e com retenções, CT-e).

## Aviso
Validações e arquivos SPED são de conferência/pré-escrituração; tabelas de alíquotas, MVA e URLs de webservices são parâmetros de referência a revisar conforme a legislação vigente.
