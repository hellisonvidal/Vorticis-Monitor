# Vorticis Monitor

Programa portátil para Windows que consulta e extrai seus documentos fiscais eletrônicos (NF-e e NFS-e) e guarda tudo localmente (SQLite + XMLs em disco). Somente consulta e extração: **não manifesta** notas.

## O que faz

- **NF-e recebidas**: consulta a distribuição DFe da SEFAZ. Sem manifestação, chegam apenas o **resumo** da nota. Se o Auditto manifestar a nota, o XML completo também chega.
- **NF-e emitidas**: por **Importar XMLs de uma pasta**, com os XMLs que o seu emissor gerou. A SEFAZ não devolve ao emitente o XML completo das próprias notas.
- **NFS-e emitidas e tomadas**: via API de distribuição do ADN (NFS-e Padrão Nacional).

## Download e uso

1. Baixe o zip `VorticisMonitor-windows`: na aba **Actions** do repositório (último build) ou em **Releases** (versões com tag `v*`).
2. Extraia em qualquer pasta. Não precisa instalar nada.
3. Abra `VorticisMonitor.exe`.
4. Na aba **Empresas**, cadastre a empresa (veja **Certificado** abaixo).
5. No cabeçalho, escolha a **empresa** e o **tipo de consulta** (NF-e ou NFS-e) e clique em **Consultar**.

### Cabeçalho

- **Consultar**: busca novos documentos, respeitando a espera de 1 hora entre consultas.
- **Forçar atualização**: ignora a espera de 1 hora. **Cuidado:** se a SEFAZ/ADN considerar o acesso como consumo indevido (cStat 656), o CNPJ fica bloqueado por 1 hora, inclusive para o Auditto, que usa o mesmo CNPJ.

## Consultas e exportação

- A lista não tem limite de registros. Selecione um ou mais documentos (ou use **Todos da lista**).
- Exporte para a **pasta padrão** (definida em **Configurações**):
  - **XML**: um arquivo por documento;
  - **XML em lote (ZIP)**;
  - **PDF**: a DANFE é gerada localmente a partir do XML completo; a DANFSe oficial é baixada do ADN;
  - **Planilha (Excel)**.
- Resumos de NF-e (sem XML completo) não geram PDF nem XML completo.

## Configurações

Menu superior **Configurações**:

- ambiente (homologação ou produção);
- pasta padrão de exportação;
- consulta automática com a janela aberta (gravada no `config.toml`) e intervalo, mínimo 60 minutos.

## Consulta automática

- **Com o programa aberto**: ative em **Configurações**; a consulta roda no intervalo configurado.
- **Mesmo com o programa fechado**: execute `agendar_consulta.bat`. Ele cria a tarefa **"Vorticis Monitor - consulta"** no Agendador de Tarefas, que roda de hora em hora para o seu usuário. Para remover: `agendar_consulta.bat remover`.

## Estrutura da pasta portátil

```
VorticisMonitor\
  VorticisMonitor.exe       programa
  config.exemplo.toml       modelo de configuração (copie para config.toml)
  config.toml               sua configuração (criada por você)
  agendar_consulta.bat      cria/remove a consulta automática
  LEIA-ME.txt               instruções de uso
  certificados\             arquivos .pfx (não compartilhe)
  dados\                    criada na primeira execução
    monitor.db              banco de dados
    xml\                    XMLs organizados por CNPJ, tipo e mês
    monitor.log             log de execução
  exportacoes\              pasta padrão de exportação
```

Caminhos relativos no `config.toml` são relativos a esta pasta.

## Linha de comando

```
VorticisMonitor.exe executar              um ciclo de consulta para todas as empresas
VorticisMonitor.exe importar PASTA        importa XMLs de NF-e emitidas
VorticisMonitor.exe listar [--cnpj CNPJ] [--limite 50]
VorticisMonitor.exe certificados          mostra titular e validade de cada certificado
```

Sem argumentos, abre a interface gráfica. Use `--config outro.toml` (antes do comando) para outro arquivo de configuração. Comece por `certificados` para conferir senha e validade.

## Certificado

- **Certificado instalado no Windows**: na aba **Empresas**, use **Adicionar a partir de certificado instalado...** para escolher o certificado em uma janela nativa, igual à do navegador. Razão social, CNPJ e UF são lidos do próprio certificado.
- **Importar todos os certificados instalados**: cadastra de uma vez os certificados encontrados no Windows.
- **A1 instalado e A3 (token/cartão) são suportados**: o TLS é feito pelo Windows (WinHTTP), sem exportar a chave privada. Se o certificado estiver em token, o Windows pode pedir o PIN.
- **Arquivo `.pfx`** continua aceito: use **Procurar...** na aba Empresas e informe a senha.

## Convivendo com o Auditto

- Cada sistema guarda o **próprio NSU**, então um não atrapalha o outro na consulta do mesmo CNPJ.
- O limite de consumo da SEFAZ é **por CNPJ, somando os dois sistemas**. Por isso o intervalo mínimo é de 60 minutos, e o monitor respeita os retornos 137 (nenhum documento) e 656 (consumo indevido), esperando 1 hora antes de tentar de novo.
- Este monitor não manifesta. Se o Auditto já manifesta, nada precisa ser configurado.

## Segurança

- A senha de um `.pfx` digitada na interface é gravada no `config.toml` como `dpapi:...`, protegida pelo Windows (DPAPI). Só o mesmo usuário, no mesmo computador, consegue descriptografar.
- Certificados instalados no Windows não têm senha guardada pelo programa.
- Ao copiar a pasta para outro computador ou usuário, digite a senha de novo (ou cadastre o certificado instalado).
- Não compartilhe a pasta `certificados` nem o `config.toml` com senha. Eles dão acesso ao certificado.
- Alternativa: `senha = "env:NOME"` lê a senha de uma variável de ambiente.

## Limitações

- **Valide antes de produção**: use `ambiente = "homologacao"` primeiro e só mude para `"producao"` depois de conferir os dados.
- Os nomes dos campos JSON do ADN e os parâmetros da API vêm de integrações de terceiros. A referência oficial é o Swagger: <https://adn.nfse.gov.br/contribuintes/docs/index.html>. Confirme na produção restrita.
- O endpoint `/danfse` do ADN (DANFSe oficial) não está confirmado oficialmente. Valide na homologação.
- Municípios que ainda não compartilham dados com o ADN não aparecem na consulta de NFS-e.
- NF-e emitidas dependem da importação de pasta. Nada é baixado automaticamente.
- Erro de TLS com a SEFAZ: em `[geral]`, aponte `verificar_tls` para um arquivo `.pem` com a cadeia ICP-Brasil.

## Compilar

**Opção 1 - GitHub Actions (sem Windows local):** cada push gera o zip como artefato. Uma tag `v*` (ex.: `git tag v1.0.0 && git push --tags`) também publica o zip em um Release.

**Opção 2 - no seu Windows:** com Python 3.11+ instalado, execute `build.bat`. Ele cria `.venv`, instala as dependências, gera o `.exe` com PyInstaller e monta a pasta `dist\VorticisMonitor\`.

Arquivos envolvidos: `run_monitor.py` (entrada), `VorticisMonitor.spec` (PyInstaller, onefile e sem console; os recursos da interface ficam embutidos no `.exe`).

## Desenvolvimento

```bash
python3 -m venv .venv
source .venv/bin/activate              # Windows: .venv\Scripts\activate
pip install -r requirements.txt
python -m monitor_notas                # abre a interface gráfica
python -m monitor_notas executar       # ciclo de consulta pelo terminal
python -m pytest -q                    # testes
```
