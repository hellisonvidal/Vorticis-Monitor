package store

// Esquema relacional do Vorticis Monitor. SQL padrão (SQLite no modo desktop;
// portável para PostgreSQL no modo web multiempresa — ver docs/ARQUITETURA.md).
const schema = `
CREATE TABLE IF NOT EXISTS empresa (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cnpj TEXT NOT NULL UNIQUE,
  razao_social TEXT NOT NULL,
  uf TEXT NOT NULL,
  ie TEXT,
  cod_mun TEXT,
  regime TEXT NOT NULL DEFAULT 'LR',        -- SN | MEI | LP | LR
  ambiente INTEGER NOT NULL DEFAULT 2,      -- 1 produção | 2 homologação
  captura_automatica INTEGER NOT NULL DEFAULT 1,
  ciencia_automatica INTEGER NOT NULL DEFAULT 0,
  ativo INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS certificado (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  empresa_id INTEGER NOT NULL REFERENCES empresa(id),
  tipo TEXT NOT NULL DEFAULT 'A1',
  titular TEXT, cnpj TEXT, serial TEXT, emissor TEXT,
  valido_de TEXT, valido_ate TEXT,
  pfx_cifrado BLOB NOT NULL,
  senha_cifrada BLOB NOT NULL,
  ativo INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS documento (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  empresa_id INTEGER REFERENCES empresa(id),
  chave TEXT NOT NULL UNIQUE,
  modelo TEXT NOT NULL,                     -- NFE | NFCE | CTE | MDFE | NFSE
  direcao TEXT NOT NULL,                    -- E | S
  tipo_operacao TEXT,                       -- VENDA | COMPRA | DEVOLUCAO | TRANSFERENCIA | SERVICO | TRANSPORTE | OUTROS
  serie TEXT, numero TEXT,
  dh_emissao TEXT, dh_autorizacao TEXT,
  emit_doc TEXT, emit_nome TEXT, emit_uf TEXT,
  dest_doc TEXT, dest_nome TEXT, dest_uf TEXT,
  natureza TEXT, cfop_principal TEXT,
  valor_total REAL NOT NULL DEFAULT 0,
  valor_icms REAL DEFAULT 0, valor_st REAL DEFAULT 0, valor_ipi REAL DEFAULT 0,
  valor_pis REAL DEFAULT 0, valor_cofins REAL DEFAULT 0, valor_difal REAL DEFAULT 0,
  valor_iss REAL DEFAULT 0, valor_retencoes REAL DEFAULT 0,
  status TEXT NOT NULL,                     -- ver docs: máquina de estados
  c_stat INTEGER, x_motivo TEXT, protocolo TEXT,
  tp_emis TEXT, fin_nfe TEXT,
  origem TEXT NOT NULL DEFAULT 'IMPORTACAO', -- IMPORTACAO | DISTDFE | ERP | NFSE_NACIONAL | PASTA
  resumo INTEGER NOT NULL DEFAULT 0,        -- 1 = apenas resNFe (aguardando XML completo)
  manifestacao TEXT,                        -- CIENCIA | CONFIRMADA | DESCONHECIDA | NAO_REALIZADA
  dh_manifestacao TEXT,
  conciliacao TEXT,                         -- PENDENTE | CONCILIADO | DIVERGENTE | SEM_PEDIDO
  score INTEGER, erros INTEGER DEFAULT 0, alertas INTEGER DEFAULT 0,
  xml TEXT,
  hash TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS ix_doc_emp_status ON documento(empresa_id, status);
CREATE INDEX IF NOT EXISTS ix_doc_emissao ON documento(dh_emissao);

CREATE TABLE IF NOT EXISTS documento_item (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  documento_id INTEGER NOT NULL REFERENCES documento(id) ON DELETE CASCADE,
  n_item INTEGER NOT NULL,
  cod_prod TEXT, descricao TEXT, ncm TEXT, cest TEXT, cfop TEXT, unidade TEXT,
  quantidade REAL, valor_unit REAL, valor_total REAL, valor_desconto REAL,
  cst_icms TEXT, v_bc REAL, p_icms REAL, v_icms REAL,
  v_bc_st REAL, p_mva REAL, p_icms_st REAL, v_icms_st REAL,
  cst_ipi TEXT, v_ipi REAL, cst_pis TEXT, v_pis REAL, cst_cofins TEXT, v_cofins REAL,
  v_difal REAL, v_fcp_dest REAL,
  ped_compra TEXT, item_ped TEXT
);
CREATE INDEX IF NOT EXISTS ix_item_doc ON documento_item(documento_id);

CREATE TABLE IF NOT EXISTS documento_referencia (
  documento_id INTEGER NOT NULL REFERENCES documento(id) ON DELETE CASCADE,
  chave_referenciada TEXT NOT NULL,
  tipo TEXT NOT NULL DEFAULT 'NFREF',       -- NFREF | CTE_NFE | DEVOLUCAO
  PRIMARY KEY (documento_id, chave_referenciada)
);

CREATE TABLE IF NOT EXISTS documento_evento (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  documento_id INTEGER REFERENCES documento(id) ON DELETE CASCADE,
  chave TEXT NOT NULL,
  tp_evento TEXT NOT NULL, descricao TEXT, n_seq INTEGER,
  dh_evento TEXT, protocolo TEXT, c_stat INTEGER, x_motivo TEXT,
  xml TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (chave, tp_evento, n_seq)
);

CREATE TABLE IF NOT EXISTS ocorrencia (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  documento_id INTEGER NOT NULL REFERENCES documento(id) ON DELETE CASCADE,
  regra TEXT, grupo TEXT, severidade TEXT, n_item INTEGER, campo TEXT,
  mensagem TEXT, sugestao TEXT, esperado TEXT, informado TEXT,
  cod_rejeicao INTEGER, acao_correcao TEXT,
  resolvida INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS ix_oc_doc ON ocorrencia(documento_id);

CREATE TABLE IF NOT EXISTS alerta (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  empresa_id INTEGER,
  documento_id INTEGER,
  tipo TEXT NOT NULL,
  severidade TEXT NOT NULL,
  titulo TEXT NOT NULL, mensagem TEXT,
  vence_em TEXT,
  lido INTEGER NOT NULL DEFAULT 0,
  chave_unica TEXT UNIQUE,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS nsu_controle (
  empresa_id INTEGER NOT NULL,
  servico TEXT NOT NULL,                    -- NFE_DISTDFE | CTE_DISTDFE | NFSE_ADN
  ult_nsu TEXT NOT NULL DEFAULT '000000000000000',
  max_nsu TEXT,
  proxima_consulta TEXT,
  ultimo_c_stat INTEGER, ultimo_motivo TEXT,
  updated_at TEXT,
  PRIMARY KEY (empresa_id, servico)
);

CREATE TABLE IF NOT EXISTS pedido_compra (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  empresa_id INTEGER NOT NULL REFERENCES empresa(id),
  numero TEXT NOT NULL,
  fornecedor_cnpj TEXT NOT NULL,
  data_pedido TEXT,
  status TEXT NOT NULL DEFAULT 'ABERTO',    -- ABERTO | PARCIAL | ATENDIDO | CANCELADO
  UNIQUE (empresa_id, numero)
);

CREATE TABLE IF NOT EXISTS pedido_item (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  pedido_id INTEGER NOT NULL REFERENCES pedido_compra(id) ON DELETE CASCADE,
  item TEXT NOT NULL,
  cod_produto TEXT, cod_produto_fornecedor TEXT, descricao TEXT, ncm TEXT,
  quantidade REAL NOT NULL, valor_unitario REAL NOT NULL,
  quantidade_recebida REAL NOT NULL DEFAULT 0,
  finalidade TEXT NOT NULL DEFAULT 'REVENDA' -- REVENDA | INDUSTRIALIZACAO | USO_CONSUMO | ATIVO
);

CREATE TABLE IF NOT EXISTS conciliacao (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  documento_id INTEGER NOT NULL REFERENCES documento(id) ON DELETE CASCADE,
  pedido_id INTEGER REFERENCES pedido_compra(id),
  ordem_recebimento TEXT,
  status TEXT NOT NULL,
  divergencias TEXT,                        -- JSON
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS param_uf (
  uf TEXT PRIMARY KEY, aliq_interna REAL NOT NULL, fcp REAL NOT NULL DEFAULT 0,
  difal_metodo TEXT NOT NULL DEFAULT 'BASE_DUPLA', updated_at TEXT
);

CREATE TABLE IF NOT EXISTS param_mva (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ncm TEXT NOT NULL, cest TEXT, uf_destino TEXT NOT NULL,
  mva_original REAL NOT NULL,
  vigencia_ini TEXT NOT NULL, vigencia_fim TEXT,
  fundamento TEXT
);
CREATE INDEX IF NOT EXISTS ix_mva ON param_mva(ncm, uf_destino);

CREATE TABLE IF NOT EXISTS param_ncm (
  ncm TEXT PRIMARY KEY, descricao TEXT, aliq_ipi REAL, vigencia_fim TEXT
);

CREATE TABLE IF NOT EXISTS usuario (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  login TEXT NOT NULL UNIQUE,
  nome TEXT,
  senha_hash TEXT NOT NULL,
  perfil TEXT NOT NULL DEFAULT 'ADMIN',     -- ADMIN | FISCAL | CONSULTA | INTEGRACAO
  ativo INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS webhook (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  url TEXT NOT NULL, eventos TEXT NOT NULL, segredo TEXT, ativo INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS auditoria (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  usuario TEXT, acao TEXT NOT NULL, entidade TEXT, entidade_id TEXT, detalhe TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
