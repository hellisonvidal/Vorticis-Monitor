// Package store implementa a persistência do Vorticis Monitor.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"vorticis.com.br/monitor/internal/fiscal"
)

type Store struct {
	DB *sql.DB
	mu sync.Mutex // serializa escritas (SQLite)

	ncmCarregado bool
}

// Open abre (ou cria) o banco e aplica o esquema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("aplicando esquema: %w", err)
	}
	// Migrações simples (colunas novas em bancos já existentes; erro de coluna duplicada é ignorado).
	for _, m := range []string{
		`ALTER TABLE nsu_controle ADD COLUMN ultima_consulta TEXT`,
		`ALTER TABLE nsu_controle ADD COLUMN bloqueios INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE nsu_controle ADD COLUMN concorrente INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE empresa ADD COLUMN nfse_modo TEXT NOT NULL DEFAULT 'PROPRIO'`,
		`ALTER TABLE empresa ADD COLUMN nfse_procurador_id INTEGER`,
		`CREATE TABLE IF NOT EXISTS procurador (
		  id INTEGER PRIMARY KEY AUTOINCREMENT,
		  nome TEXT NOT NULL, documento TEXT, tipo TEXT NOT NULL DEFAULT 'A1',
		  titular TEXT, emissor TEXT, valido_ate TEXT,
		  pfx_cifrado BLOB NOT NULL, senha_cifrada BLOB NOT NULL,
		  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`ALTER TABLE procurador ADD COLUMN uf TEXT`,
		`ALTER TABLE procurador ADD COLUMN nfe_autxml INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE empresa ADD COLUMN usar_cert_matriz INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE usuario ADD COLUMN ultimo_acesso TEXT`,
	} {
		db.Exec(m)
	}
	s := &Store{DB: db}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM param_uf`).Scan(&n)
	if n == 0 {
		for _, p := range fiscal.ParametrosUFPadrao {
			s.SalvarParamUF(p)
		}
	}
	db.QueryRow(`SELECT COUNT(*) FROM param_ncm`).Scan(&n)
	s.ncmCarregado = n > 0
	return s, nil
}

func agora() string { return time.Now().Format(time.RFC3339) }

// ---------------- Empresa ----------------

type Empresa struct {
	ID                int64  `json:"id"`
	CNPJ              string `json:"cnpj"`
	RazaoSocial       string `json:"razao_social"`
	UF                string `json:"uf"`
	IE                string `json:"ie"`
	CodMun            string `json:"cod_mun"`
	Regime            string `json:"regime"`
	Ambiente          int    `json:"ambiente"`
	CapturaAutomatica bool   `json:"captura_automatica"`
	CienciaAutomatica bool   `json:"ciencia_automatica"`
	Ativo             bool   `json:"ativo"`
	TemCertificado    bool   `json:"tem_certificado"`
	// NFS-e Nacional (ADN): PROPRIO = certificado da empresa; PROCURACAO = certificado
	// do procurador (contador) com procuração eletrônica da empresa.
	NFSeModo         string `json:"nfse_modo"`
	NFSeProcuradorID int64  `json:"nfse_procurador_id"`
	// Filiais: sem certificado próprio, usam o certificado de outra empresa da
	// mesma raiz de CNPJ (de preferência a matriz, /0001).
	UsarCertMatriz bool   `json:"usar_cert_matriz"`
	CertProprio    bool   `json:"cert_proprio"`
	CertEmpresaID  int64  `json:"cert_empresa_id,omitempty"`
	CertEmpresa    string `json:"cert_empresa,omitempty"`
	Matriz         bool   `json:"matriz"`
}

const empCols = `id, cnpj, razao_social, uf, COALESCE(ie,''), COALESCE(cod_mun,''), regime, ambiente, captura_automatica, ciencia_automatica, ativo,
 EXISTS(SELECT 1 FROM certificado c WHERE c.empresa_id = empresa.id AND c.ativo = 1),
 COALESCE(nfse_modo,'PROPRIO'), COALESCE(nfse_procurador_id,0), COALESCE(usar_cert_matriz,1),
 COALESCE((SELECT e2.id FROM empresa e2 WHERE substr(e2.cnpj,1,8) = substr(empresa.cnpj,1,8) AND e2.id <> empresa.id AND length(empresa.cnpj) = 14
   AND EXISTS(SELECT 1 FROM certificado c2 WHERE c2.empresa_id = e2.id AND c2.ativo = 1)
   ORDER BY substr(e2.cnpj,9,4) = '0001' DESC, e2.id LIMIT 1), 0)`

func scanEmpresa(r interface{ Scan(...any) error }) (*Empresa, error) {
	e := &Empresa{}
	var raiz int64
	err := r.Scan(&e.ID, &e.CNPJ, &e.RazaoSocial, &e.UF, &e.IE, &e.CodMun, &e.Regime, &e.Ambiente, &e.CapturaAutomatica, &e.CienciaAutomatica, &e.Ativo, &e.TemCertificado, &e.NFSeModo, &e.NFSeProcuradorID, &e.UsarCertMatriz, &raiz)
	e.CertProprio = e.TemCertificado
	e.Matriz = len(e.CNPJ) == 14 && e.CNPJ[8:12] == "0001"
	if !e.CertProprio && e.UsarCertMatriz && raiz > 0 {
		e.TemCertificado, e.CertEmpresaID = true, raiz
	}
	return e, err
}

// completarNomeCert preenche o nome da empresa cujo certificado é usado (filial).
func (s *Store) completarNomeCert(e *Empresa) {
	if e.CertEmpresaID > 0 {
		s.DB.QueryRow(`SELECT razao_social || ' (' || cnpj || ')' FROM empresa WHERE id = ?`, e.CertEmpresaID).Scan(&e.CertEmpresa)
	}
}

func (s *Store) ListarEmpresas() ([]*Empresa, error) {
	rows, err := s.DB.Query(`SELECT ` + empCols + ` FROM empresa ORDER BY razao_social`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Empresa{}
	for rows.Next() {
		e, err := scanEmpresa(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	rows.Close()
	for _, e := range out {
		s.completarNomeCert(e)
	}
	return out, nil
}

func (s *Store) ObterEmpresa(id int64) (*Empresa, error) {
	e, err := scanEmpresa(s.DB.QueryRow(`SELECT `+empCols+` FROM empresa WHERE id = ?`, id))
	if err == nil {
		s.completarNomeCert(e)
	}
	return e, err
}

func (s *Store) EmpresaPorCNPJ(cnpj string) (*Empresa, error) {
	return scanEmpresa(s.DB.QueryRow(`SELECT `+empCols+` FROM empresa WHERE cnpj = ?`, fiscal.NormalizaCNPJ(cnpj)))
}

func (s *Store) SalvarEmpresa(e *Empresa) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.CNPJ = fiscal.NormalizaCNPJ(e.CNPJ)
	if e.Ambiente == 0 {
		e.Ambiente = 2
	}
	if e.Regime == "" {
		e.Regime = "LR"
	}
	if e.ID == 0 {
		r, err := s.DB.Exec(`INSERT INTO empresa (cnpj, razao_social, uf, ie, cod_mun, regime, ambiente, captura_automatica, ciencia_automatica, ativo, usar_cert_matriz) VALUES (?,?,?,?,?,?,?,?,?,1,?)`,
			e.CNPJ, e.RazaoSocial, strings.ToUpper(e.UF), e.IE, e.CodMun, e.Regime, e.Ambiente, e.CapturaAutomatica, e.CienciaAutomatica, e.UsarCertMatriz)
		if err != nil {
			return err
		}
		e.ID, _ = r.LastInsertId()
		e.Ativo = true
		return nil
	}
	_, err := s.DB.Exec(`UPDATE empresa SET razao_social=?, uf=?, ie=?, cod_mun=?, regime=?, ambiente=?, captura_automatica=?, ciencia_automatica=?, ativo=?, usar_cert_matriz=? WHERE id=?`,
		e.RazaoSocial, strings.ToUpper(e.UF), e.IE, e.CodMun, e.Regime, e.Ambiente, e.CapturaAutomatica, e.CienciaAutomatica, e.Ativo, e.UsarCertMatriz, e.ID)
	return err
}

// ContarDocumentosEmpresa devolve quantos documentos estão vinculados à empresa.
func (s *Store) ContarDocumentosEmpresa(id int64) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM documento WHERE empresa_id = ?`, id).Scan(&n)
	return n
}

// ExcluirEmpresa remove a empresa, seus certificados, controle de NSU e pedidos de
// compra. Os documentos são excluídos (apagarDocumentos) ou ficam no monitor sem
// vínculo de empresa.
func (s *Store) ExcluirEmpresa(id int64, apagarDocumentos bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	doc := `UPDATE documento SET empresa_id = NULL WHERE empresa_id = ?`
	if apagarDocumentos {
		doc = `DELETE FROM documento WHERE empresa_id = ?`
	}
	for _, q := range []string{
		doc,
		`DELETE FROM pedido_compra WHERE empresa_id = ?`,
		`DELETE FROM certificado WHERE empresa_id = ?`,
		`DELETE FROM nsu_controle WHERE empresa_id = ?`,
		`DELETE FROM empresa WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SalvarNFSeEmpresa grava como a empresa consulta a NFS-e Nacional.
func (s *Store) SalvarNFSeEmpresa(id int64, modo string, procurador int64) error {
	if modo != "PROCURACAO" {
		modo, procurador = "PROPRIO", 0
	}
	var p any
	if procurador > 0 {
		p = procurador
	}
	_, err := s.DB.Exec(`UPDATE empresa SET nfse_modo = ?, nfse_procurador_id = ? WHERE id = ?`, modo, p, id)
	return err
}

// ---------------- Procurador (certificado de terceiro com procuração) ----------------

type Procurador struct {
	ID        int64  `json:"id"`
	Nome      string `json:"nome"`
	Documento string `json:"documento"`
	Tipo      string `json:"tipo"`
	Titular   string `json:"titular"`
	Emissor   string `json:"emissor"`
	ValidoAte string `json:"valido_ate"`
	Empresas  int    `json:"empresas"`
	// UF do procurador (cUFAutor na Distribuição DF-e) e se busca as NF-e em que
	// ele consta como autorizado a obter o XML (autXML).
	UF        string `json:"uf"`
	NFeAutXML bool   `json:"nfe_autxml"`
	PFX       []byte `json:"-"`
	Senha     []byte `json:"-"`
}

func (s *Store) SalvarProcurador(p *Procurador) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.DB.Exec(`INSERT INTO procurador (nome, documento, tipo, titular, emissor, valido_ate, pfx_cifrado, senha_cifrada, uf, nfe_autxml) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		p.Nome, p.Documento, p.Tipo, p.Titular, p.Emissor, p.ValidoAte, p.PFX, p.Senha, strings.ToUpper(p.UF), p.NFeAutXML)
	if err == nil {
		p.ID, _ = r.LastInsertId()
	}
	return err
}

func (s *Store) ListarProcuradores() ([]*Procurador, error) {
	rows, err := s.DB.Query(`SELECT id, nome, COALESCE(documento,''), tipo, COALESCE(titular,''), COALESCE(emissor,''), COALESCE(valido_ate,''), COALESCE(uf,''), nfe_autxml,
		(SELECT COUNT(*) FROM empresa e WHERE e.nfse_procurador_id = procurador.id AND e.nfse_modo = 'PROCURACAO') FROM procurador ORDER BY nome`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Procurador{}
	for rows.Next() {
		p := &Procurador{}
		rows.Scan(&p.ID, &p.Nome, &p.Documento, &p.Tipo, &p.Titular, &p.Emissor, &p.ValidoAte, &p.UF, &p.NFeAutXML, &p.Empresas)
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) ObterProcurador(id int64) (*Procurador, error) {
	p := &Procurador{}
	err := s.DB.QueryRow(`SELECT id, nome, COALESCE(documento,''), tipo, COALESCE(titular,''), COALESCE(emissor,''), COALESCE(valido_ate,''), COALESCE(uf,''), nfe_autxml, pfx_cifrado, senha_cifrada FROM procurador WHERE id = ?`, id).
		Scan(&p.ID, &p.Nome, &p.Documento, &p.Tipo, &p.Titular, &p.Emissor, &p.ValidoAte, &p.UF, &p.NFeAutXML, &p.PFX, &p.Senha)
	return p, err
}

// AtualizarProcurador altera a UF e a busca de NF-e (autXML) do procurador.
func (s *Store) AtualizarProcurador(id int64, uf string, autxml bool) error {
	_, err := s.DB.Exec(`UPDATE procurador SET uf = ?, nfe_autxml = ? WHERE id = ?`, strings.ToUpper(uf), autxml, id)
	return err
}

func (s *Store) RemoverProcurador(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DB.Exec(`UPDATE empresa SET nfse_modo = 'PROPRIO', nfse_procurador_id = NULL WHERE nfse_procurador_id = ?`, id)
	_, err := s.DB.Exec(`DELETE FROM procurador WHERE id = ?`, id)
	return err
}

// ---------------- Certificado ----------------

type Certificado struct {
	ID            int64  `json:"id"`
	EmpresaID     int64  `json:"empresa_id"`
	Tipo          string `json:"tipo"`
	Titular       string `json:"titular"`
	CNPJ          string `json:"cnpj"`
	Serial        string `json:"serial"`
	Emissor       string `json:"emissor"`
	ValidoDe      string `json:"valido_de"`
	ValidoAte     string `json:"valido_ate"`
	DiasRestantes int    `json:"dias_restantes"`
	// HerdadoDe: empresa (matriz) dona do certificado usado por uma filial.
	HerdadoDe string `json:"herdado_de,omitempty"`
	PFX       []byte `json:"-"`
	Senha     []byte `json:"-"`
}

func (s *Store) SalvarCertificado(c *Certificado) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE certificado SET ativo = 0 WHERE empresa_id = ?`, c.EmpresaID); err != nil {
		return err
	}
	r, err := tx.Exec(`INSERT INTO certificado (empresa_id, tipo, titular, cnpj, serial, emissor, valido_de, valido_ate, pfx_cifrado, senha_cifrada) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		c.EmpresaID, c.Tipo, c.Titular, c.CNPJ, c.Serial, c.Emissor, c.ValidoDe, c.ValidoAte, c.PFX, c.Senha)
	if err != nil {
		return err
	}
	c.ID, _ = r.LastInsertId()
	return tx.Commit()
}

func (s *Store) CertificadoAtivo(empresaID int64) (*Certificado, error) {
	c := &Certificado{}
	err := s.DB.QueryRow(`SELECT id, empresa_id, tipo, COALESCE(titular,''), COALESCE(cnpj,''), COALESCE(serial,''), COALESCE(emissor,''), COALESCE(valido_de,''), COALESCE(valido_ate,''), pfx_cifrado, senha_cifrada
		FROM certificado WHERE empresa_id = ? AND ativo = 1 ORDER BY id DESC LIMIT 1`, empresaID).
		Scan(&c.ID, &c.EmpresaID, &c.Tipo, &c.Titular, &c.CNPJ, &c.Serial, &c.Emissor, &c.ValidoDe, &c.ValidoAte, &c.PFX, &c.Senha)
	if err != nil {
		return nil, err
	}
	if t, err := time.Parse(time.RFC3339, c.ValidoAte); err == nil {
		c.DiasRestantes = int(time.Until(t).Hours() / 24)
	}
	return c, nil
}

// CertificadoEfetivo devolve o certificado próprio da empresa ou, na filial sem
// certificado e com "usar certificado da matriz", o da empresa de mesma raiz.
func (s *Store) CertificadoEfetivo(empresaID int64) (*Certificado, error) {
	if c, err := s.CertificadoAtivo(empresaID); err == nil {
		return c, nil
	}
	e, err := s.ObterEmpresa(empresaID)
	if err != nil {
		return nil, err
	}
	if e.CertEmpresaID == 0 {
		return nil, sql.ErrNoRows
	}
	c, err := s.CertificadoAtivo(e.CertEmpresaID)
	if err == nil {
		c.HerdadoDe = e.CertEmpresa
	}
	return c, err
}

func (s *Store) RemoverCertificado(empresaID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`DELETE FROM certificado WHERE empresa_id = ?`, empresaID)
	return err
}

// ---------------- Documento ----------------

type Documento struct {
	ID             int64   `json:"id"`
	EmpresaID      *int64  `json:"empresa_id"`
	Chave          string  `json:"chave"`
	Modelo         string  `json:"modelo"`
	Direcao        string  `json:"direcao"`
	TipoOperacao   string  `json:"tipo_operacao"`
	Serie          string  `json:"serie"`
	Numero         string  `json:"numero"`
	DhEmissao      string  `json:"dh_emissao"`
	DhAutorizacao  string  `json:"dh_autorizacao"`
	EmitDoc        string  `json:"emit_doc"`
	EmitNome       string  `json:"emit_nome"`
	EmitUF         string  `json:"emit_uf"`
	DestDoc        string  `json:"dest_doc"`
	DestNome       string  `json:"dest_nome"`
	DestUF         string  `json:"dest_uf"`
	Natureza       string  `json:"natureza"`
	CFOPPrincipal  string  `json:"cfop_principal"`
	ValorTotal     float64 `json:"valor_total"`
	ValorICMS      float64 `json:"valor_icms"`
	ValorST        float64 `json:"valor_st"`
	ValorIPI       float64 `json:"valor_ipi"`
	ValorPIS       float64 `json:"valor_pis"`
	ValorCOFINS    float64 `json:"valor_cofins"`
	ValorDIFAL     float64 `json:"valor_difal"`
	ValorISS       float64 `json:"valor_iss"`
	ValorRetencoes float64 `json:"valor_retencoes"`
	Status         string  `json:"status"`
	CStat          int     `json:"c_stat"`
	XMotivo        string  `json:"x_motivo"`
	Protocolo      string  `json:"protocolo"`
	TpEmis         string  `json:"tp_emis"`
	FinNFe         string  `json:"fin_nfe"`
	Origem         string  `json:"origem"`
	Resumo         bool    `json:"resumo"`
	Manifestacao   string  `json:"manifestacao"`
	DhManifestacao string  `json:"dh_manifestacao"`
	Conciliacao    string  `json:"conciliacao"`
	Score          int     `json:"score"`
	Erros          int     `json:"erros"`
	Alertas        int     `json:"alertas"`
	XML            string  `json:"-"`
	Hash           string  `json:"-"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type Item struct {
	NItem         int     `json:"n_item"`
	CodProd       string  `json:"cod_prod"`
	Descricao     string  `json:"descricao"`
	NCM           string  `json:"ncm"`
	CEST          string  `json:"cest"`
	CFOP          string  `json:"cfop"`
	Unidade       string  `json:"unidade"`
	Quantidade    float64 `json:"quantidade"`
	ValorUnit     float64 `json:"valor_unit"`
	ValorTotal    float64 `json:"valor_total"`
	ValorDesconto float64 `json:"valor_desconto"`
	CSTICMS       string  `json:"cst_icms"`
	VBC           float64 `json:"v_bc"`
	PICMS         float64 `json:"p_icms"`
	VICMS         float64 `json:"v_icms"`
	VBCST         float64 `json:"v_bc_st"`
	PMVA          float64 `json:"p_mva"`
	PICMSST       float64 `json:"p_icms_st"`
	VICMSST       float64 `json:"v_icms_st"`
	CSTIPI        string  `json:"cst_ipi"`
	VIPI          float64 `json:"v_ipi"`
	CSTPIS        string  `json:"cst_pis"`
	VPIS          float64 `json:"v_pis"`
	CSTCOFINS     string  `json:"cst_cofins"`
	VCOFINS       float64 `json:"v_cofins"`
	VDIFAL        float64 `json:"v_difal"`
	VFCPDest      float64 `json:"v_fcp_dest"`
	PedCompra     string  `json:"ped_compra"`
	ItemPed       string  `json:"item_ped"`
}

type Ocorrencia struct {
	ID           int64  `json:"id"`
	Regra        string `json:"regra"`
	Grupo        string `json:"grupo"`
	Severidade   string `json:"severidade"`
	NItem        int    `json:"item,omitempty"`
	Campo        string `json:"campo"`
	Mensagem     string `json:"mensagem"`
	Sugestao     string `json:"sugestao"`
	Esperado     string `json:"esperado"`
	Informado    string `json:"informado"`
	CodRejeicao  int    `json:"cod_rejeicao,omitempty"`
	AcaoCorrecao string `json:"acao_correcao,omitempty"`
	Resolvida    bool   `json:"resolvida"`
}

const docCols = `id, empresa_id, chave, modelo, direcao, COALESCE(tipo_operacao,''), COALESCE(serie,''), COALESCE(numero,''), COALESCE(dh_emissao,''), COALESCE(dh_autorizacao,''),
 COALESCE(emit_doc,''), COALESCE(emit_nome,''), COALESCE(emit_uf,''), COALESCE(dest_doc,''), COALESCE(dest_nome,''), COALESCE(dest_uf,''), COALESCE(natureza,''), COALESCE(cfop_principal,''),
 valor_total, COALESCE(valor_icms,0), COALESCE(valor_st,0), COALESCE(valor_ipi,0), COALESCE(valor_pis,0), COALESCE(valor_cofins,0), COALESCE(valor_difal,0), COALESCE(valor_iss,0), COALESCE(valor_retencoes,0),
 status, COALESCE(c_stat,0), COALESCE(x_motivo,''), COALESCE(protocolo,''), COALESCE(tp_emis,''), COALESCE(fin_nfe,''), origem, resumo, COALESCE(manifestacao,''), COALESCE(dh_manifestacao,''),
 COALESCE(conciliacao,''), COALESCE(score,0), COALESCE(erros,0), COALESCE(alertas,0), created_at, updated_at`

func scanDoc(r interface{ Scan(...any) error }) (*Documento, error) {
	d := &Documento{}
	var emp sql.NullInt64
	err := r.Scan(&d.ID, &emp, &d.Chave, &d.Modelo, &d.Direcao, &d.TipoOperacao, &d.Serie, &d.Numero, &d.DhEmissao, &d.DhAutorizacao,
		&d.EmitDoc, &d.EmitNome, &d.EmitUF, &d.DestDoc, &d.DestNome, &d.DestUF, &d.Natureza, &d.CFOPPrincipal,
		&d.ValorTotal, &d.ValorICMS, &d.ValorST, &d.ValorIPI, &d.ValorPIS, &d.ValorCOFINS, &d.ValorDIFAL, &d.ValorISS, &d.ValorRetencoes,
		&d.Status, &d.CStat, &d.XMotivo, &d.Protocolo, &d.TpEmis, &d.FinNFe, &d.Origem, &d.Resumo, &d.Manifestacao, &d.DhManifestacao,
		&d.Conciliacao, &d.Score, &d.Erros, &d.Alertas, &d.CreatedAt, &d.UpdatedAt)
	if emp.Valid {
		d.EmpresaID = &emp.Int64
	}
	return d, err
}

// SalvarDocumento grava (upsert pela chave) o documento, itens, ocorrências e referências.
// Um resumo (resNFe) nunca sobrescreve um XML completo.
func (s *Store) SalvarDocumento(d *Documento, itens []Item, ocs []Ocorrencia, refs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	var resumoExistente bool
	err = tx.QueryRow(`SELECT id, resumo FROM documento WHERE chave = ?`, d.Chave).Scan(&id, &resumoExistente)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if id != 0 && d.Resumo && !resumoExistente {
		d.ID = id
		return nil
	}
	args := []any{d.EmpresaID, d.Chave, d.Modelo, d.Direcao, d.TipoOperacao, d.Serie, d.Numero, d.DhEmissao, d.DhAutorizacao,
		d.EmitDoc, d.EmitNome, d.EmitUF, d.DestDoc, d.DestNome, d.DestUF, d.Natureza, d.CFOPPrincipal,
		d.ValorTotal, d.ValorICMS, d.ValorST, d.ValorIPI, d.ValorPIS, d.ValorCOFINS, d.ValorDIFAL, d.ValorISS, d.ValorRetencoes,
		d.Status, d.CStat, d.XMotivo, d.Protocolo, d.TpEmis, d.FinNFe, d.Origem, d.Resumo, d.Score, d.Erros, d.Alertas, d.XML, d.Hash, agora()}
	if id == 0 {
		r, err := tx.Exec(`INSERT INTO documento (empresa_id, chave, modelo, direcao, tipo_operacao, serie, numero, dh_emissao, dh_autorizacao,
			emit_doc, emit_nome, emit_uf, dest_doc, dest_nome, dest_uf, natureza, cfop_principal,
			valor_total, valor_icms, valor_st, valor_ipi, valor_pis, valor_cofins, valor_difal, valor_iss, valor_retencoes,
			status, c_stat, x_motivo, protocolo, tp_emis, fin_nfe, origem, resumo, score, erros, alertas, xml, hash, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, args...)
		if err != nil {
			return err
		}
		id, _ = r.LastInsertId()
	} else {
		args = append(args, id)
		_, err := tx.Exec(`UPDATE documento SET empresa_id=?, chave=?, modelo=?, direcao=?, tipo_operacao=?, serie=?, numero=?, dh_emissao=?, dh_autorizacao=?,
			emit_doc=?, emit_nome=?, emit_uf=?, dest_doc=?, dest_nome=?, dest_uf=?, natureza=?, cfop_principal=?,
			valor_total=?, valor_icms=?, valor_st=?, valor_ipi=?, valor_pis=?, valor_cofins=?, valor_difal=?, valor_iss=?, valor_retencoes=?,
			status=?, c_stat=?, x_motivo=?, protocolo=?, tp_emis=?, fin_nfe=?, origem=?, resumo=?, score=?, erros=?, alertas=?, xml=?, hash=?, updated_at=? WHERE id=?`, args...)
		if err != nil {
			return err
		}
		for _, t := range []string{"documento_item", "ocorrencia", "documento_referencia"} {
			if _, err := tx.Exec(`DELETE FROM `+t+` WHERE documento_id = ?`, id); err != nil {
				return err
			}
		}
	}
	d.ID = id
	for _, i := range itens {
		if _, err := tx.Exec(`INSERT INTO documento_item (documento_id, n_item, cod_prod, descricao, ncm, cest, cfop, unidade, quantidade, valor_unit, valor_total, valor_desconto,
			cst_icms, v_bc, p_icms, v_icms, v_bc_st, p_mva, p_icms_st, v_icms_st, cst_ipi, v_ipi, cst_pis, v_pis, cst_cofins, v_cofins, v_difal, v_fcp_dest, ped_compra, item_ped)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, i.NItem, i.CodProd, i.Descricao, i.NCM, i.CEST, i.CFOP, i.Unidade, i.Quantidade, i.ValorUnit, i.ValorTotal, i.ValorDesconto,
			i.CSTICMS, i.VBC, i.PICMS, i.VICMS, i.VBCST, i.PMVA, i.PICMSST, i.VICMSST, i.CSTIPI, i.VIPI, i.CSTPIS, i.VPIS, i.CSTCOFINS, i.VCOFINS, i.VDIFAL, i.VFCPDest, i.PedCompra, i.ItemPed); err != nil {
			return err
		}
	}
	for _, o := range ocs {
		if _, err := tx.Exec(`INSERT INTO ocorrencia (documento_id, regra, grupo, severidade, n_item, campo, mensagem, sugestao, esperado, informado, cod_rejeicao, acao_correcao)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, id, o.Regra, o.Grupo, o.Severidade, o.NItem, o.Campo, o.Mensagem, o.Sugestao, o.Esperado, o.Informado, o.CodRejeicao, o.AcaoCorrecao); err != nil {
			return err
		}
	}
	for _, r := range refs {
		tx.Exec(`INSERT OR IGNORE INTO documento_referencia (documento_id, chave_referenciada) VALUES (?,?)`, id, r)
	}
	return tx.Commit()
}

// FiltroDocumentos parametriza a listagem do grid.
type FiltroDocumentos struct {
	EmpresaID int64
	Status    string
	Modelo    string
	Direcao   string
	De, Ate   string
	Todos     bool // sem paginação (relatórios)
	Busca     string
	Pagina    int
	PorPagina int
	SoComErro bool
}

func (f FiltroDocumentos) where() (string, []any) {
	w := []string{"1=1"}
	var a []any
	if f.EmpresaID > 0 {
		w, a = append(w, "empresa_id = ?"), append(a, f.EmpresaID)
	}
	if f.Status != "" {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(strings.Split(f.Status, ","))), ",")
		w = append(w, "status IN ("+ph+")")
		for _, s := range strings.Split(f.Status, ",") {
			a = append(a, s)
		}
	}
	if f.Modelo != "" {
		ms := strings.Split(f.Modelo, ",")
		w = append(w, "modelo IN ("+strings.TrimSuffix(strings.Repeat("?,", len(ms)), ",")+")")
		for _, m := range ms {
			a = append(a, m)
		}
	}
	if f.Direcao != "" {
		w, a = append(w, "direcao = ?"), append(a, f.Direcao)
	}
	if f.De != "" {
		w, a = append(w, "substr(dh_emissao,1,10) >= ?"), append(a, f.De)
	}
	if f.Ate != "" {
		w, a = append(w, "substr(dh_emissao,1,10) <= ?"), append(a, f.Ate)
	}
	if f.Busca != "" {
		q := "%" + f.Busca + "%"
		w = append(w, "(chave LIKE ? OR numero LIKE ? OR emit_nome LIKE ? OR dest_nome LIKE ? OR emit_doc LIKE ? OR dest_doc LIKE ?)")
		a = append(a, q, q, q, q, q, q)
	}
	if f.SoComErro {
		w = append(w, "(erros > 0 OR status IN ('REJEITADA','DENEGADA'))")
	}
	return strings.Join(w, " AND "), a
}

func (s *Store) ListarDocumentos(f FiltroDocumentos) ([]*Documento, int, error) {
	if f.Todos {
		f.Pagina, f.PorPagina = 1, 1000000
	} else if f.PorPagina <= 0 || f.PorPagina > 5000 {
		f.PorPagina = 50
	}
	if f.Pagina <= 0 {
		f.Pagina = 1
	}
	w, a := f.where()
	var total int
	s.DB.QueryRow(`SELECT COUNT(*) FROM documento WHERE `+w, a...).Scan(&total)
	rows, err := s.DB.Query(`SELECT `+docCols+` FROM documento WHERE `+w+` ORDER BY dh_emissao DESC, id DESC LIMIT ? OFFSET ?`,
		append(a, f.PorPagina, (f.Pagina-1)*f.PorPagina)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*Documento{}
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, nil
}

// DocumentosComXML retorna documentos (com XML) para exportação.
func (s *Store) DocumentosComXML(f FiltroDocumentos) ([]*Documento, error) {
	w, a := f.where()
	rows, err := s.DB.Query(`SELECT `+docCols+`, COALESCE(xml,'') FROM documento WHERE `+w+` ORDER BY dh_emissao`, a...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Documento
	for rows.Next() {
		d := &Documento{}
		var emp sql.NullInt64
		if err := rows.Scan(&d.ID, &emp, &d.Chave, &d.Modelo, &d.Direcao, &d.TipoOperacao, &d.Serie, &d.Numero, &d.DhEmissao, &d.DhAutorizacao,
			&d.EmitDoc, &d.EmitNome, &d.EmitUF, &d.DestDoc, &d.DestNome, &d.DestUF, &d.Natureza, &d.CFOPPrincipal,
			&d.ValorTotal, &d.ValorICMS, &d.ValorST, &d.ValorIPI, &d.ValorPIS, &d.ValorCOFINS, &d.ValorDIFAL, &d.ValorISS, &d.ValorRetencoes,
			&d.Status, &d.CStat, &d.XMotivo, &d.Protocolo, &d.TpEmis, &d.FinNFe, &d.Origem, &d.Resumo, &d.Manifestacao, &d.DhManifestacao,
			&d.Conciliacao, &d.Score, &d.Erros, &d.Alertas, &d.CreatedAt, &d.UpdatedAt, &d.XML); err != nil {
			return nil, err
		}
		if emp.Valid {
			d.EmpresaID = &emp.Int64
		}
		out = append(out, d)
	}
	return out, nil
}

func (s *Store) ObterDocumento(chave string) (*Documento, error) {
	d, err := scanDoc(s.DB.QueryRow(`SELECT `+docCols+` FROM documento WHERE chave = ?`, chave))
	if err != nil {
		return nil, err
	}
	s.DB.QueryRow(`SELECT COALESCE(xml,'') FROM documento WHERE id = ?`, d.ID).Scan(&d.XML)
	return d, nil
}

func (s *Store) AtualizarSituacao(chave, status string, cstat int, motivo, protocolo, dhAut string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`UPDATE documento SET status=?, c_stat=?, x_motivo=?, protocolo=COALESCE(NULLIF(?,''),protocolo), dh_autorizacao=COALESCE(NULLIF(?,''),dh_autorizacao), updated_at=? WHERE chave=?`,
		status, cstat, motivo, protocolo, dhAut, agora(), chave)
	return err
}

func (s *Store) AtualizarManifestacao(chave, tipo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`UPDATE documento SET manifestacao=?, dh_manifestacao=?, updated_at=? WHERE chave=?`, tipo, agora(), agora(), chave)
	return err
}

func (s *Store) AtualizarConciliacao(docID int64, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`UPDATE documento SET conciliacao=?, updated_at=? WHERE id=?`, status, agora(), docID)
	return err
}

func (s *Store) Itens(docID int64) ([]Item, error) {
	rows, err := s.DB.Query(`SELECT n_item, COALESCE(cod_prod,''), COALESCE(descricao,''), COALESCE(ncm,''), COALESCE(cest,''), COALESCE(cfop,''), COALESCE(unidade,''),
		COALESCE(quantidade,0), COALESCE(valor_unit,0), COALESCE(valor_total,0), COALESCE(valor_desconto,0), COALESCE(cst_icms,''), COALESCE(v_bc,0), COALESCE(p_icms,0), COALESCE(v_icms,0),
		COALESCE(v_bc_st,0), COALESCE(p_mva,0), COALESCE(p_icms_st,0), COALESCE(v_icms_st,0), COALESCE(cst_ipi,''), COALESCE(v_ipi,0), COALESCE(cst_pis,''), COALESCE(v_pis,0),
		COALESCE(cst_cofins,''), COALESCE(v_cofins,0), COALESCE(v_difal,0), COALESCE(v_fcp_dest,0), COALESCE(ped_compra,''), COALESCE(item_ped,'')
		FROM documento_item WHERE documento_id = ? ORDER BY n_item`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var i Item
		if err := rows.Scan(&i.NItem, &i.CodProd, &i.Descricao, &i.NCM, &i.CEST, &i.CFOP, &i.Unidade, &i.Quantidade, &i.ValorUnit, &i.ValorTotal, &i.ValorDesconto,
			&i.CSTICMS, &i.VBC, &i.PICMS, &i.VICMS, &i.VBCST, &i.PMVA, &i.PICMSST, &i.VICMSST, &i.CSTIPI, &i.VIPI, &i.CSTPIS, &i.VPIS, &i.CSTCOFINS, &i.VCOFINS, &i.VDIFAL, &i.VFCPDest, &i.PedCompra, &i.ItemPed); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, nil
}

func (s *Store) Ocorrencias(docID int64) ([]Ocorrencia, error) {
	rows, err := s.DB.Query(`SELECT id, COALESCE(regra,''), COALESCE(grupo,''), COALESCE(severidade,''), COALESCE(n_item,0), COALESCE(campo,''), COALESCE(mensagem,''), COALESCE(sugestao,''),
		COALESCE(esperado,''), COALESCE(informado,''), COALESCE(cod_rejeicao,0), COALESCE(acao_correcao,''), resolvida FROM ocorrencia WHERE documento_id = ? ORDER BY id`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ocorrencia{}
	for rows.Next() {
		var o Ocorrencia
		if err := rows.Scan(&o.ID, &o.Regra, &o.Grupo, &o.Severidade, &o.NItem, &o.Campo, &o.Mensagem, &o.Sugestao, &o.Esperado, &o.Informado, &o.CodRejeicao, &o.AcaoCorrecao, &o.Resolvida); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

// ---------------- Eventos ----------------

type Evento struct {
	Chave     string `json:"chave"`
	TpEvento  string `json:"tp_evento"`
	Descricao string `json:"descricao"`
	NSeq      int    `json:"n_seq"`
	DhEvento  string `json:"dh_evento"`
	Protocolo string `json:"protocolo"`
	CStat     int    `json:"c_stat"`
	XMotivo   string `json:"x_motivo"`
	XML       string `json:"-"`
}

func (s *Store) SalvarEvento(e Evento) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var docID sql.NullInt64
	s.DB.QueryRow(`SELECT id FROM documento WHERE chave = ?`, e.Chave).Scan(&docID)
	_, err := s.DB.Exec(`INSERT INTO documento_evento (documento_id, chave, tp_evento, descricao, n_seq, dh_evento, protocolo, c_stat, x_motivo, xml) VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (chave, tp_evento, n_seq) DO UPDATE SET protocolo=excluded.protocolo, c_stat=excluded.c_stat, x_motivo=excluded.x_motivo, xml=COALESCE(excluded.xml, documento_evento.xml)`,
		docID, e.Chave, e.TpEvento, e.Descricao, e.NSeq, e.DhEvento, e.Protocolo, e.CStat, e.XMotivo, e.XML)
	return err
}

func (s *Store) Eventos(chave string) ([]Evento, error) {
	rows, err := s.DB.Query(`SELECT chave, tp_evento, COALESCE(descricao,''), COALESCE(n_seq,0), COALESCE(dh_evento,''), COALESCE(protocolo,''), COALESCE(c_stat,0), COALESCE(x_motivo,'')
		FROM documento_evento WHERE chave = ? ORDER BY dh_evento`, chave)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Evento{}
	for rows.Next() {
		var e Evento
		rows.Scan(&e.Chave, &e.TpEvento, &e.Descricao, &e.NSeq, &e.DhEvento, &e.Protocolo, &e.CStat, &e.XMotivo)
		out = append(out, e)
	}
	return out, nil
}

// ---------------- Dashboard ----------------

type Resumo struct {
	PorStatus       map[string]int     `json:"por_status"`
	PorModelo       map[string]int     `json:"por_modelo"`
	ValorPorDir     map[string]float64 `json:"valor_por_direcao"`
	Tributos        map[string]float64 `json:"tributos"`
	ScoreMedio      float64            `json:"score_medio"`
	Total           int                `json:"total"`
	ComErro         int                `json:"com_erro"`
	SemManifestacao int                `json:"sem_manifestacao"`
	Divergentes     int                `json:"conciliacao_divergente"`
	// Painel: emitidos (S) e recebidos (E) separados, e quantidade por dia e movimento.
	PorDirecao map[string]*ResumoDirecao `json:"por_direcao"`
	PorDia     map[string]map[string]int `json:"por_dia"`
}

// ResumoDirecao resume um movimento (emitidos ou recebidos) no período.
type ResumoDirecao struct {
	Total     int                      `json:"total"`
	Valor     float64                  `json:"valor"` // autorizados e em contingência
	PorStatus map[string]int           `json:"por_status"`
	PorModelo map[string]*ResumoModelo `json:"por_modelo"`
}

type ResumoModelo struct {
	Qtd   int     `json:"qtd"`
	Valor float64 `json:"valor"`
}

func (s *Store) Resumo(f FiltroDocumentos) (*Resumo, error) {
	w, a := f.where()
	r := &Resumo{PorStatus: map[string]int{}, PorModelo: map[string]int{}, ValorPorDir: map[string]float64{}, Tributos: map[string]float64{},
		PorDirecao: map[string]*ResumoDirecao{}, PorDia: map[string]map[string]int{}}
	for _, d := range []string{"S", "E"} {
		r.PorDirecao[d] = &ResumoDirecao{PorStatus: map[string]int{}, PorModelo: map[string]*ResumoModelo{}}
	}
	q := func(sqlq string, fn func(*sql.Rows)) {
		rows, err := s.DB.Query(sqlq, a...)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			fn(rows)
		}
	}
	q(`SELECT status, COUNT(*) FROM documento WHERE `+w+` GROUP BY status`, func(rs *sql.Rows) {
		var k string
		var n int
		rs.Scan(&k, &n)
		r.PorStatus[k] = n
		r.Total += n
	})
	q(`SELECT direcao, modelo, status, COUNT(*), COALESCE(SUM(valor_total),0) FROM documento WHERE `+w+` GROUP BY direcao, modelo, status`, func(rs *sql.Rows) {
		var dir, mod, st string
		var n int
		var v float64
		rs.Scan(&dir, &mod, &st, &n, &v)
		rd := r.PorDirecao[dir]
		if rd == nil {
			return
		}
		rd.Total += n
		rd.PorStatus[st] += n
		if rd.PorModelo[mod] == nil {
			rd.PorModelo[mod] = &ResumoModelo{}
		}
		rd.PorModelo[mod].Qtd += n
		if st == "AUTORIZADA" || st == "CONTINGENCIA" {
			rd.Valor += v
			rd.PorModelo[mod].Valor += v
		}
	})
	q(`SELECT substr(dh_emissao,1,10), direcao, COUNT(*) FROM documento WHERE `+w+` GROUP BY 1, 2`, func(rs *sql.Rows) {
		var dia, dir string
		var n int
		rs.Scan(&dia, &dir, &n)
		if r.PorDia[dia] == nil {
			r.PorDia[dia] = map[string]int{}
		}
		r.PorDia[dia][dir] = n
	})
	q(`SELECT modelo, COUNT(*) FROM documento WHERE `+w+` GROUP BY modelo`, func(rs *sql.Rows) {
		var k string
		var n int
		rs.Scan(&k, &n)
		r.PorModelo[k] = n
	})
	q(`SELECT direcao, SUM(valor_total) FROM documento WHERE `+w+` AND status IN ('AUTORIZADA','CONTINGENCIA') GROUP BY direcao`, func(rs *sql.Rows) {
		var k string
		var v float64
		rs.Scan(&k, &v)
		r.ValorPorDir[k] = v
	})
	q(`SELECT SUM(valor_icms), SUM(valor_st), SUM(valor_ipi), SUM(valor_pis), SUM(valor_cofins), SUM(valor_difal), SUM(valor_iss), SUM(valor_retencoes), AVG(score),
		SUM(CASE WHEN erros>0 THEN 1 ELSE 0 END), SUM(CASE WHEN direcao='E' AND modelo='NFE' AND COALESCE(manifestacao,'') IN ('','CIENCIA') THEN 1 ELSE 0 END),
		SUM(CASE WHEN conciliacao='DIVERGENTE' THEN 1 ELSE 0 END) FROM documento WHERE `+w, func(rs *sql.Rows) {
		var v [9]sql.NullFloat64
		var e, m, dv sql.NullInt64
		rs.Scan(&v[0], &v[1], &v[2], &v[3], &v[4], &v[5], &v[6], &v[7], &v[8], &e, &m, &dv)
		for i, k := range []string{"icms", "icms_st", "ipi", "pis", "cofins", "difal", "iss", "retencoes"} {
			r.Tributos[k] = v[i].Float64
		}
		r.ScoreMedio, r.ComErro, r.SemManifestacao, r.Divergentes = v[8].Float64, int(e.Int64), int(m.Int64), int(dv.Int64)
	})
	return r, nil
}

// ---------------- NSU ----------------

type NSU struct {
	EmpresaID       int64  `json:"empresa_id"`
	Servico         string `json:"servico"`
	UltNSU          string `json:"ult_nsu"`
	MaxNSU          string `json:"max_nsu"`
	ProximaConsulta string `json:"proxima_consulta"`
	UltimoCStat     int    `json:"ultimo_c_stat"`
	UltimoMotivo    string `json:"ultimo_motivo"`
	UpdatedAt       string `json:"updated_at"`
	// UltimaConsulta é o horário da última chamada feita por ESTE monitor à SEFAZ.
	UltimaConsulta string `json:"ultima_consulta"`
	// Bloqueios conta as rejeições 656 (consumo indevido) seguidas.
	Bloqueios int `json:"bloqueios"`
	// Concorrente indica que outro sistema (outro monitor de notas, ERP) consulta o mesmo CNPJ.
	Concorrente bool `json:"concorrente"`
}

const colsNSU = `empresa_id, servico, ult_nsu, COALESCE(max_nsu,''), COALESCE(proxima_consulta,''), COALESCE(ultimo_c_stat,0), COALESCE(ultimo_motivo,''), COALESCE(updated_at,''), COALESCE(ultima_consulta,''), bloqueios, concorrente`

func scanNSU(sc interface{ Scan(...any) error }, n *NSU) error {
	return sc.Scan(&n.EmpresaID, &n.Servico, &n.UltNSU, &n.MaxNSU, &n.ProximaConsulta, &n.UltimoCStat, &n.UltimoMotivo, &n.UpdatedAt, &n.UltimaConsulta, &n.Bloqueios, &n.Concorrente)
}

func (s *Store) ObterNSU(empresaID int64, servico string) NSU {
	n := NSU{EmpresaID: empresaID, Servico: servico, UltNSU: "000000000000000"}
	scanNSU(s.DB.QueryRow(`SELECT `+colsNSU+` FROM nsu_controle WHERE empresa_id=? AND servico=?`, empresaID, servico), &n)
	return n
}

func (s *Store) SalvarNSU(n NSU) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`INSERT INTO nsu_controle (empresa_id, servico, ult_nsu, max_nsu, proxima_consulta, ultimo_c_stat, ultimo_motivo, updated_at, ultima_consulta, bloqueios, concorrente) VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (empresa_id, servico) DO UPDATE SET ult_nsu=excluded.ult_nsu, max_nsu=excluded.max_nsu, proxima_consulta=excluded.proxima_consulta,
		ultimo_c_stat=excluded.ultimo_c_stat, ultimo_motivo=excluded.ultimo_motivo, updated_at=excluded.updated_at,
		ultima_consulta=excluded.ultima_consulta, bloqueios=excluded.bloqueios, concorrente=excluded.concorrente`,
		n.EmpresaID, n.Servico, n.UltNSU, n.MaxNSU, n.ProximaConsulta, n.UltimoCStat, n.UltimoMotivo, agora(), n.UltimaConsulta, n.Bloqueios, n.Concorrente)
	return err
}

func (s *Store) ListarNSU() ([]NSU, error) {
	rows, err := s.DB.Query(`SELECT ` + colsNSU + ` FROM nsu_controle`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NSU{}
	for rows.Next() {
		var n NSU
		scanNSU(rows, &n)
		out = append(out, n)
	}
	return out, nil
}

// ---------------- Pedidos de compra / conciliação ----------------

type PedidoItem struct {
	ID                   int64   `json:"id"`
	Item                 string  `json:"item"`
	CodProduto           string  `json:"cod_produto"`
	CodProdutoFornecedor string  `json:"cod_produto_fornecedor"`
	Descricao            string  `json:"descricao"`
	NCM                  string  `json:"ncm"`
	Quantidade           float64 `json:"quantidade"`
	ValorUnitario        float64 `json:"valor_unitario"`
	QuantidadeRecebida   float64 `json:"quantidade_recebida"`
	Finalidade           string  `json:"finalidade"`
}

type Pedido struct {
	ID             int64        `json:"id"`
	EmpresaID      int64        `json:"empresa_id"`
	Numero         string       `json:"numero"`
	FornecedorCNPJ string       `json:"fornecedor_cnpj"`
	DataPedido     string       `json:"data_pedido"`
	Status         string       `json:"status"`
	Itens          []PedidoItem `json:"itens"`
}

func (s *Store) SalvarPedido(p *Pedido) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if p.Status == "" {
		p.Status = "ABERTO"
	}
	p.FornecedorCNPJ = fiscal.NormalizaCNPJ(p.FornecedorCNPJ)
	r, err := tx.Exec(`INSERT INTO pedido_compra (empresa_id, numero, fornecedor_cnpj, data_pedido, status) VALUES (?,?,?,?,?)
		ON CONFLICT (empresa_id, numero) DO UPDATE SET fornecedor_cnpj=excluded.fornecedor_cnpj, data_pedido=excluded.data_pedido, status=excluded.status`,
		p.EmpresaID, p.Numero, p.FornecedorCNPJ, p.DataPedido, p.Status)
	if err != nil {
		return err
	}
	tx.QueryRow(`SELECT id FROM pedido_compra WHERE empresa_id=? AND numero=?`, p.EmpresaID, p.Numero).Scan(&p.ID)
	_ = r
	tx.Exec(`DELETE FROM pedido_item WHERE pedido_id = ?`, p.ID)
	for _, i := range p.Itens {
		if i.Finalidade == "" {
			i.Finalidade = "REVENDA"
		}
		if _, err := tx.Exec(`INSERT INTO pedido_item (pedido_id, item, cod_produto, cod_produto_fornecedor, descricao, ncm, quantidade, valor_unitario, quantidade_recebida, finalidade) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			p.ID, i.Item, i.CodProduto, i.CodProdutoFornecedor, i.Descricao, i.NCM, i.Quantidade, i.ValorUnitario, i.QuantidadeRecebida, i.Finalidade); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) carregarItensPedido(p *Pedido) {
	rows, err := s.DB.Query(`SELECT id, item, COALESCE(cod_produto,''), COALESCE(cod_produto_fornecedor,''), COALESCE(descricao,''), COALESCE(ncm,''), quantidade, valor_unitario, quantidade_recebida, finalidade
		FROM pedido_item WHERE pedido_id = ? ORDER BY id`, p.ID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var i PedidoItem
		rows.Scan(&i.ID, &i.Item, &i.CodProduto, &i.CodProdutoFornecedor, &i.Descricao, &i.NCM, &i.Quantidade, &i.ValorUnitario, &i.QuantidadeRecebida, &i.Finalidade)
		p.Itens = append(p.Itens, i)
	}
}

// Pedidos retorna pedidos filtrados (numero vazio = todos do fornecedor).
func (s *Store) Pedidos(empresaID int64, fornecedor, numero string, soAbertos bool) ([]*Pedido, error) {
	w, a := []string{"empresa_id = ?"}, []any{empresaID}
	if fornecedor != "" {
		w, a = append(w, "fornecedor_cnpj = ?"), append(a, fiscal.NormalizaCNPJ(fornecedor))
	}
	if numero != "" {
		w, a = append(w, "numero = ?"), append(a, numero)
	}
	if soAbertos {
		w = append(w, "status IN ('ABERTO','PARCIAL')")
	}
	rows, err := s.DB.Query(`SELECT id, empresa_id, numero, fornecedor_cnpj, COALESCE(data_pedido,''), status FROM pedido_compra WHERE `+strings.Join(w, " AND ")+` ORDER BY id DESC`, a...)
	if err != nil {
		return nil, err
	}
	var out []*Pedido
	for rows.Next() {
		p := &Pedido{}
		rows.Scan(&p.ID, &p.EmpresaID, &p.Numero, &p.FornecedorCNPJ, &p.DataPedido, &p.Status)
		out = append(out, p)
	}
	rows.Close()
	for _, p := range out {
		s.carregarItensPedido(p)
	}
	return out, nil
}

func (s *Store) RegistrarRecebimento(pedidoItemID int64, qtd float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`UPDATE pedido_item SET quantidade_recebida = quantidade_recebida + ? WHERE id = ?`, qtd, pedidoItemID)
	return err
}

func (s *Store) AtualizarStatusPedido(pedidoID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var total, receb float64
	s.DB.QueryRow(`SELECT SUM(quantidade), SUM(MIN(quantidade_recebida, quantidade)) FROM pedido_item WHERE pedido_id = ?`, pedidoID).Scan(&total, &receb)
	st := "ABERTO"
	if receb >= total-0.0001 && total > 0 {
		st = "ATENDIDO"
	} else if receb > 0 {
		st = "PARCIAL"
	}
	s.DB.Exec(`UPDATE pedido_compra SET status = ? WHERE id = ?`, st, pedidoID)
}

type Conciliacao struct {
	ID               int64  `json:"id"`
	DocumentoID      int64  `json:"documento_id"`
	Chave            string `json:"chave"`
	PedidoID         int64  `json:"pedido_id"`
	PedidoNumero     string `json:"pedido_numero"`
	OrdemRecebimento string `json:"ordem_recebimento"`
	Status           string `json:"status"`
	Divergencias     any    `json:"divergencias"`
	CreatedAt        string `json:"created_at"`
}

func (s *Store) SalvarConciliacao(c *Conciliacao) error {
	b, _ := json.Marshal(c.Divergencias)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DB.Exec(`DELETE FROM conciliacao WHERE documento_id = ?`, c.DocumentoID)
	var ped any
	if c.PedidoID > 0 {
		ped = c.PedidoID
	}
	r, err := s.DB.Exec(`INSERT INTO conciliacao (documento_id, pedido_id, ordem_recebimento, status, divergencias) VALUES (?,?,?,?,?)`, c.DocumentoID, ped, c.OrdemRecebimento, c.Status, string(b))
	if err != nil {
		return err
	}
	c.ID, _ = r.LastInsertId()
	_, err = s.DB.Exec(`UPDATE documento SET conciliacao=? WHERE id=?`, c.Status, c.DocumentoID)
	return err
}

func (s *Store) ListarConciliacoes(limite int) ([]Conciliacao, error) {
	if limite <= 0 {
		limite = 200
	}
	rows, err := s.DB.Query(`SELECT c.id, c.documento_id, d.chave, COALESCE(c.pedido_id,0), COALESCE(p.numero,''), COALESCE(c.ordem_recebimento,''), c.status, COALESCE(c.divergencias,'[]'), c.created_at
		FROM conciliacao c JOIN documento d ON d.id = c.documento_id LEFT JOIN pedido_compra p ON p.id = c.pedido_id ORDER BY c.id DESC LIMIT ?`, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Conciliacao{}
	for rows.Next() {
		var c Conciliacao
		var dv string
		rows.Scan(&c.ID, &c.DocumentoID, &c.Chave, &c.PedidoID, &c.PedidoNumero, &c.OrdemRecebimento, &c.Status, &dv, &c.CreatedAt)
		var x any
		json.Unmarshal([]byte(dv), &x)
		c.Divergencias = x
		out = append(out, c)
	}
	return out, nil
}

// ---------------- Parâmetros (implementa regras.Parametros) ----------------

func (s *Store) UF(uf string) (fiscal.ParametroUF, bool) {
	var p fiscal.ParametroUF
	err := s.DB.QueryRow(`SELECT uf, aliq_interna, fcp, difal_metodo FROM param_uf WHERE uf = ?`, uf).Scan(&p.UF, &p.AliqInterna, &p.FCP, &p.DifalMetodo)
	return p, err == nil
}

func (s *Store) MVA(ncm, cest, uf string, data time.Time) (float64, bool) {
	d := data.Format("2006-01-02")
	if data.IsZero() {
		d = time.Now().Format("2006-01-02")
	}
	var mva float64
	err := s.DB.QueryRow(`SELECT mva_original FROM param_mva WHERE uf_destino = ? AND (? LIKE ncm || '%') AND (cest IS NULL OR cest = '' OR cest = ?)
		AND vigencia_ini <= ? AND (vigencia_fim IS NULL OR vigencia_fim = '' OR vigencia_fim >= ?) ORDER BY LENGTH(ncm) DESC, vigencia_ini DESC LIMIT 1`,
		uf, ncm, cest, d, d).Scan(&mva)
	return mva, err == nil
}

func (s *Store) NCMExiste(ncm string) (bool, bool) {
	if !s.ncmCarregado {
		return false, false
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM param_ncm WHERE ncm = ? AND (vigencia_fim IS NULL OR vigencia_fim = '' OR vigencia_fim >= date('now'))`, ncm).Scan(&n)
	return n > 0, true
}

func (s *Store) SalvarParamUF(p fiscal.ParametroUF) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`INSERT INTO param_uf (uf, aliq_interna, fcp, difal_metodo, updated_at) VALUES (?,?,?,?,?)
		ON CONFLICT (uf) DO UPDATE SET aliq_interna=excluded.aliq_interna, fcp=excluded.fcp, difal_metodo=excluded.difal_metodo, updated_at=excluded.updated_at`,
		strings.ToUpper(p.UF), p.AliqInterna, p.FCP, p.DifalMetodo, agora())
	return err
}

func (s *Store) ListarParamUF() ([]fiscal.ParametroUF, error) {
	rows, err := s.DB.Query(`SELECT uf, aliq_interna, fcp, difal_metodo FROM param_uf ORDER BY uf`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []fiscal.ParametroUF{}
	for rows.Next() {
		var p fiscal.ParametroUF
		rows.Scan(&p.UF, &p.AliqInterna, &p.FCP, &p.DifalMetodo)
		out = append(out, p)
	}
	return out, nil
}

type ParamMVA struct {
	ID          int64   `json:"id"`
	NCM         string  `json:"ncm"`
	CEST        string  `json:"cest"`
	UFDestino   string  `json:"uf_destino"`
	MVAOriginal float64 `json:"mva_original"`
	VigenciaIni string  `json:"vigencia_ini"`
	VigenciaFim string  `json:"vigencia_fim"`
	Fundamento  string  `json:"fundamento"`
}

func (s *Store) SalvarMVA(p *ParamMVA) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.VigenciaIni == "" {
		p.VigenciaIni = "2000-01-01"
	}
	r, err := s.DB.Exec(`INSERT INTO param_mva (ncm, cest, uf_destino, mva_original, vigencia_ini, vigencia_fim, fundamento) VALUES (?,?,?,?,?,?,?)`,
		fiscal.SoDigitos(p.NCM), p.CEST, strings.ToUpper(p.UFDestino), p.MVAOriginal, p.VigenciaIni, p.VigenciaFim, p.Fundamento)
	if err != nil {
		return err
	}
	p.ID, _ = r.LastInsertId()
	return nil
}

func (s *Store) ListarMVA() ([]ParamMVA, error) {
	rows, err := s.DB.Query(`SELECT id, ncm, COALESCE(cest,''), uf_destino, mva_original, vigencia_ini, COALESCE(vigencia_fim,''), COALESCE(fundamento,'') FROM param_mva ORDER BY ncm, uf_destino`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ParamMVA{}
	for rows.Next() {
		var p ParamMVA
		rows.Scan(&p.ID, &p.NCM, &p.CEST, &p.UFDestino, &p.MVAOriginal, &p.VigenciaIni, &p.VigenciaFim, &p.Fundamento)
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) RemoverMVA(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`DELETE FROM param_mva WHERE id = ?`, id)
	return err
}

// ImportarNCM substitui a tabela TIPI (ncm;descricao;aliq_ipi).
func (s *Store) ImportarNCM(linhas [][3]string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM param_ncm`)
	n := 0
	for _, l := range linhas {
		ncm := fiscal.SoDigitos(l[0])
		if len(ncm) != 8 {
			continue
		}
		var ipi float64
		fmt.Sscan(strings.ReplaceAll(l[2], ",", "."), &ipi)
		if _, err := tx.Exec(`INSERT OR REPLACE INTO param_ncm (ncm, descricao, aliq_ipi) VALUES (?,?,?)`, ncm, l[1], ipi); err == nil {
			n++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.ncmCarregado = n > 0
	return n, nil
}

// ---------------- Usuários / auditoria / webhooks ----------------

type Usuario struct {
	ID           int64  `json:"id"`
	Login        string `json:"login"`
	Nome         string `json:"nome"`
	Perfil       string `json:"perfil"`
	Ativo        bool   `json:"ativo"`
	UltimoAcesso string `json:"ultimo_acesso,omitempty"`
	CriadoEm     string `json:"criado_em,omitempty"`
	SenhaHash    string `json:"-"`
}

func (s *Store) ContarUsuarios() int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM usuario`).Scan(&n)
	return n
}

func (s *Store) CriarUsuario(u *Usuario) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.DB.Exec(`INSERT INTO usuario (login, nome, senha_hash, perfil) VALUES (?,?,?,?)`, strings.ToLower(u.Login), u.Nome, u.SenhaHash, u.Perfil)
	if err != nil {
		return err
	}
	u.ID, _ = r.LastInsertId()
	return nil
}

func (s *Store) UsuarioPorLogin(login string) (*Usuario, error) {
	u := &Usuario{}
	err := s.DB.QueryRow(`SELECT id, login, COALESCE(nome,''), perfil, ativo, senha_hash FROM usuario WHERE login = ?`, strings.ToLower(login)).
		Scan(&u.ID, &u.Login, &u.Nome, &u.Perfil, &u.Ativo, &u.SenhaHash)
	return u, err
}

func (s *Store) ListarUsuarios() ([]Usuario, error) {
	rows, err := s.DB.Query(`SELECT id, login, COALESCE(nome,''), perfil, ativo, COALESCE(ultimo_acesso,''), COALESCE(created_at,'') FROM usuario ORDER BY login`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Usuario{}
	for rows.Next() {
		var u Usuario
		rows.Scan(&u.ID, &u.Login, &u.Nome, &u.Perfil, &u.Ativo, &u.UltimoAcesso, &u.CriadoEm)
		out = append(out, u)
	}
	return out, nil
}

func (s *Store) UsuarioPorID(id int64) (*Usuario, error) {
	u := &Usuario{}
	err := s.DB.QueryRow(`SELECT id, login, COALESCE(nome,''), perfil, ativo, senha_hash FROM usuario WHERE id = ?`, id).
		Scan(&u.ID, &u.Login, &u.Nome, &u.Perfil, &u.Ativo, &u.SenhaHash)
	return u, err
}

// AtualizarUsuario altera nome, perfil e situação (ativo) do usuário.
func (s *Store) AtualizarUsuario(u *Usuario) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`UPDATE usuario SET nome = ?, perfil = ?, ativo = ? WHERE id = ?`, u.Nome, u.Perfil, u.Ativo, u.ID)
	return err
}

// AlterarSenha grava o novo hash (invalida as senhas salvas nos computadores).
func (s *Store) AlterarSenha(id int64, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`UPDATE usuario SET senha_hash = ? WHERE id = ?`, hash, id)
	return err
}

func (s *Store) RemoverUsuario(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`DELETE FROM usuario WHERE id = ?`, id)
	return err
}

// ContarAdminsAtivos conta os administradores ativos, exceto o informado.
func (s *Store) ContarAdminsAtivos(exceto int64) int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM usuario WHERE perfil = 'ADMIN' AND ativo = 1 AND id <> ?`, exceto).Scan(&n)
	return n
}

func (s *Store) RegistrarAcesso(id int64) {
	s.DB.Exec(`UPDATE usuario SET ultimo_acesso = ? WHERE id = ?`, agora(), id)
}

func (s *Store) Auditar(usuario, acao, entidade, id, detalhe string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DB.Exec(`INSERT INTO auditoria (usuario, acao, entidade, entidade_id, detalhe) VALUES (?,?,?,?,?)`, usuario, acao, entidade, id, detalhe)
}

func (s *Store) ListarAuditoria(limite int) ([]map[string]any, error) {
	rows, err := s.DB.Query(`SELECT id, COALESCE(usuario,''), acao, COALESCE(entidade,''), COALESCE(entidade_id,''), COALESCE(detalhe,''), created_at FROM auditoria ORDER BY id DESC LIMIT ?`, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var u, a, e, ei, d, c string
		rows.Scan(&id, &u, &a, &e, &ei, &d, &c)
		out = append(out, map[string]any{"id": id, "usuario": u, "acao": a, "entidade": e, "entidade_id": ei, "detalhe": d, "created_at": c})
	}
	return out, nil
}

type Webhook struct {
	ID      int64  `json:"id"`
	URL     string `json:"url"`
	Eventos string `json:"eventos"` // csv: documento.novo,documento.status,captura.ciclo
	Segredo string `json:"segredo,omitempty"`
	Ativo   bool   `json:"ativo"`
}

func (s *Store) SalvarWebhook(w *Webhook) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.DB.Exec(`INSERT INTO webhook (url, eventos, segredo, ativo) VALUES (?,?,?,1)`, w.URL, w.Eventos, w.Segredo)
	if err != nil {
		return err
	}
	w.ID, _ = r.LastInsertId()
	w.Ativo = true
	return nil
}

func (s *Store) ListarWebhooks() ([]Webhook, error) {
	rows, err := s.DB.Query(`SELECT id, url, eventos, COALESCE(segredo,''), ativo FROM webhook WHERE ativo = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Webhook{}
	for rows.Next() {
		var w Webhook
		rows.Scan(&w.ID, &w.URL, &w.Eventos, &w.Segredo, &w.Ativo)
		out = append(out, w)
	}
	return out, nil
}

func (s *Store) RemoverWebhook(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`DELETE FROM webhook WHERE id = ?`, id)
	return err
}
