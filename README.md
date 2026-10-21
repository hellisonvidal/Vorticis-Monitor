# Monitor de NF-e e NFS-e

Programa portátil para Windows que consulta e extrai seus documentos fiscais eletrônicos e guarda tudo localmente (SQLite + XMLs em disco). Somente consulta e extração: **não manifesta** notas.

## O que faz

- **NF-e recebidas**: consulta a distribuição DFe da SEFAZ com o certificado A1 da empresa. Sem manifestação, chegam apenas o **resumo** da nota. Se o Auditto manifestar a nota, o XML completo também chega.
- **NF-e emitidas**: por **Importar XMLs de uma pasta**, com os XMLs que o seu emissor gerou. A SEFAZ não devolve ao emitente o XML completo das próprias notas.
- **NFS-e emitidas e tomadas**: via API de distribuição do ADN (NFS-e Padrão Nacional).

## Download e uso

1. Baixe o zip `MonitorNotas-windows`: na aba **Actions** do repositório (último build) ou em **Releases** (versões com tag `v*`).
2. Extraia em qualquer pasta. Não precisa instalar nada.
3. Abra `MonitorNotas.exe`.
4. Na aba **Empresas**, cadastre a empresa: CNPJ, UF, certificado `.pfx` (copie para a pasta `certificados`) e senha.
5. Clique em **Consultar agora**.
6. Filtre e exporte para **Excel**, **CSV** ou **ZIP** (XMLs).

## Consulta automática

- **Com o programa aberto**: a consulta roda sozinha no intervalo configurado.
- **Mesmo com o programa fechado**: execute `agendar_consulta.bat`. Ele cria a tarefa **"MonitorNotas - consulta"** no Agendador de Tarefas, que roda de hora em hora para o seu usuário. Para remover: `agendar_consulta.bat remover`.

## Estrutura da pasta portátil

```
MonitorNotas\
  MonitorNotas.exe          programa
  config.exemplo.toml       modelo de configuração (copie para config.toml)
  config.toml               sua configuração (criada por você)
  agendar_consulta.bat      cria/remove a consulta automática
  LEIA-ME.txt               instruções de uso
  certificados\             arquivos .pfx (não compartilhe)
  dados\                    criada na primeira execução
    monitor.db              banco de dados
    xml\                    XMLs organizados por CNPJ, tipo e mês
    monitor.log             log de execução
```

Caminhos relativos no `config.toml` são relativos a esta pasta.

## Linha de comando

```
MonitorNotas.exe executar              um ciclo de consulta para todas as empresas
MonitorNotas.exe importar PASTA        importa XMLs de NF-e emitidas
MonitorNotas.exe listar [--cnpj CNPJ] [--limite 50]
MonitorNotas.exe certificados          mostra titular e validade de cada certificado
```

Sem argumentos, abre a interface gráfica. Use `--config outro.toml` (antes do comando) para outro arquivo de configuração. Comece por `certificados` para conferir senha e validade.

## Compilar

**Opção 1 - GitHub Actions (sem Windows local):** cada push gera o zip como artefato. Uma tag `v*` (ex.: `git tag v1.0.0 && git push --tags`) também publica o zip em um Release.

**Opção 2 - no seu Windows:** com Python 3.11+ instalado, execute `build.bat`. Ele cria `.venv`, instala as dependências, gera o `.exe` com PyInstaller e monta a pasta `dist\MonitorNotas\`.

Arquivos envolvidos: `run_monitor.py` (entrada), `MonitorNotas.spec` (PyInstaller, onefile e sem console).

## Convivendo com o Auditto

- Cada sistema guarda o **próprio NSU**, então um não atrapalha o outro na consulta do mesmo CNPJ.
- O limite de consumo da SEFAZ é **por CNPJ, somando os dois sistemas**. Por isso o intervalo mínimo é de 60 minutos, e o monitor respeita os retornos 137 (nenhum documento) e 656 (consumo indevido), esperando 1 hora antes de tentar de novo.
- Este monitor não manifesta. Se o Auditto já manifesta, nada precisa ser configurado.

## Certificado

- **A1 (`.pfx`)**: suportado.
- **A3 (token/cartão)**: não suportado nesta versão.

## Segurança

- A senha digitada na interface é gravada no `config.toml` como `dpapi:...`, protegida pelo Windows (DPAPI). Só o mesmo usuário, no mesmo computador, consegue descriptografar.
- Ao copiar a pasta para outro computador ou usuário, digite a senha de novo.
- Não compartilhe a pasta `certificados` nem o `config.toml` com senha. Eles dão acesso ao certificado.
- Alternativa: `senha = "env:NOME"` lê a senha de uma variável de ambiente.

## Limitações

- **Valide antes de produção**: use `ambiente = "homologacao"` primeiro e só mude para `"producao"` depois de conferir os dados.
- Os nomes dos campos JSON do ADN e os parâmetros da API vêm de integrações de terceiros. A referência oficial é o Swagger: <https://adn.nfse.gov.br/contribuintes/docs/index.html>. Confirme na produção restrita.
- Municípios que ainda não compartilham dados com o ADN não aparecem na consulta de NFS-e.
- NF-e emitidas dependem da importação de pasta. Nada é baixado automaticamente.
- Erro de TLS com a SEFAZ: em `[geral]`, aponte `verificar_tls` para um arquivo `.pem` com a cadeia ICP-Brasil.

## Desenvolvimento

```bash
python3 -m venv .venv
source .venv/bin/activate              # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python -m monitor_notas                # abre a interface gráfica
python -m monitor_notas executar       # ciclo de consulta pelo terminal
python -m pytest -q                    # testes
```
