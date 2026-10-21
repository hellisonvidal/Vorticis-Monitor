# monitor-notas

Monitor gratuito de documentos fiscais eletrônicos, feito para rodar sozinho e guardar tudo localmente (SQLite + XMLs em disco).

- **NF-e recebidas**: consulta o serviço `NFeDistribuicaoDFe` do Ambiente Nacional, usando o certificado A1 da empresa.
- **NF-e emitidas**: por **importação de pasta** com os XMLs do seu emissor. A SEFAZ não devolve ao emitente o XML completo das próprias notas, então não há como baixá-las.
- **NFS-e emitidas e tomadas**: via API de distribuição do ADN (NFS-e Padrão Nacional).

## Instalação

Requer Python 3.11+.

```bash
python3 -m venv .venv
source .venv/bin/activate        # Windows: .venv\Scripts\activate
pip install -r requirements.txt
```

## Configuração

```bash
cp config.exemplo.toml config.toml
```

Edite `config.toml`. Pontos principais:

- `[geral]`: `ambiente` (`"homologacao"` ou `"producao"`), `banco`, `pasta_xml`, `intervalo_minutos` (mínimo 60; valores menores são elevados a 60) e `verificar_tls`.
- Uma seção `[[empresa]]` por CNPJ, com `uf`, `certificado` (arquivo `.pfx`), `consultar_nfe`, `consultar_nfse` e `manifestar_ciencia`.
- Senha do certificado: use `senha = "env:NOME_DA_VARIAVEL"` e defina a variável no ambiente, em vez de gravar a senha no arquivo.

```bash
export SENHA_CERT_EMPRESA='...'      # Windows (PowerShell): $env:SENHA_CERT_EMPRESA='...'
```

## Comandos

```bash
python -m monitor_notas executar          # um ciclo para todas as empresas
python -m monitor_notas servico           # roda em loop (intervalo do config)
python -m monitor_notas importar PASTA    # importa XMLs de NF-e emitidas (do seu emissor)
python -m monitor_notas manifestar CNPJ CHAVE [--evento 210200] [--justificativa ...]
python -m monitor_notas listar [--cnpj CNPJ] [--limite 50]
python -m monitor_notas certificados      # mostra titular e validade
```

Use `--config caminho.toml` (antes do subcomando) para outro arquivo de configuração. Comece por `certificados` para conferir senha e validade.

## Onde ficam os dados

- **SQLite**: arquivo definido em `banco` (padrão `dados/monitor.db`), com os documentos, metadados e o último NSU de cada consulta.
- **XMLs**: em `pasta_xml` (padrão `dados/xml`), organizados assim:

```
dados/xml/<CNPJ>/<nfe|nfse>/<recebida|emitida>/<AAAA-MM>/<chave>.xml
```

Resumos de NF-e levam o sufixo `-resumo` e eventos `-evento<código>`. Notas sem data de emissão vão para `sem-data`. Faça backup das duas coisas (banco e pasta).

## Rodar como serviço

### Linux (systemd)

`/etc/systemd/system/monitor-notas.service`:

```ini
[Unit]
Description=Monitor de NF-e / NFS-e
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=monitor
WorkingDirectory=/opt/monitor-notas
Environment=SENHA_CERT_EMPRESA=troque-esta-senha
ExecStart=/opt/monitor-notas/.venv/bin/python -m monitor_notas servico
Restart=on-failure
RestartSec=60

[Install]
WantedBy=multi-user.target
```

Prefira `EnvironmentFile=/etc/monitor-notas.env` (com permissão `600`) a deixar a senha na unit.

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now monitor-notas
journalctl -u monitor-notas -f
```

### Windows (Agendador de Tarefas)

Crie uma tarefa que rode a cada hora chamando o comando `executar` (um ciclo e sai):

```bat
schtasks /Create /TN "MonitorNotas" /SC HOURLY /ST 00:00 ^
  /TR "cmd /c cd /d C:\monitor-notas && .venv\Scripts\python -m monitor_notas executar >> monitor.log 2>&1"
```

A variável de senha precisa existir para o usuário da tarefa (`setx SENHA_CERT_EMPRESA "..."`).

## Convivendo com o Auditto

- Os dois sistemas podem consultar o mesmo CNPJ: cada um guarda o **próprio NSU**, então um não atrapalha o outro.
- O limite de consumo da SEFAZ é **por CNPJ**, somando os dois sistemas. Por isso o intervalo mínimo é de 60 minutos e o monitor respeita os retornos `cStat` 137 (nenhum documento) e 656 (consumo indevido), esperando 1 hora antes de tentar de novo.
- Deixe `manifestar_ciencia = false` se o Auditto já manifesta. Só um sistema deve manifestar: se os dois enviarem, a SEFAZ rejeita o segundo por duplicidade (573). O monitor trata isso como sucesso, mas são chamadas desnecessárias que contam no consumo.

## Certificado A1 x A3

- **A1 (`.pfx`)**: suportado.
- **A3 (token/cartão)**: ainda não suportado. Exigiria PKCS#11 e o token conectado à máquina.
- Recomendação: use um A1 para o monitor (ele pode coexistir com o A3 usado em outros sistemas) ou aguarde a fase 2.

## Limitações / validar antes de produção

- Rode primeiro com `ambiente = "homologacao"`.
- Os nomes dos campos JSON do ADN (`StatusProcessamento`, `LoteDFe`, `ArquivoXml`...) e os parâmetros `lote` e `cnpjConsulta` vêm de integrações de terceiros. O Swagger oficial é <https://adn.nfse.gov.br/contribuintes/docs/index.html>; confirme tudo na produção restrita.
- Municípios que ainda não compartilham dados com o ADN não aparecem. O compartilhamento é obrigatório desde 01/01/2026 (LC 214/2025, art. 62).
- NF-e emitidas dependem da importação de pasta; nada é baixado automaticamente.
- Se houver erro de TLS com a SEFAZ, aponte `verificar_tls` para um arquivo `.pem` com a cadeia ICP-Brasil.

## Próximos passos

- Painel web
- Alertas por e-mail
- Suporte a certificado A3
- Download do XML completo por chave após a manifestação
