// Package api expõe a API REST do Vorticis Monitor e serve o painel web.
package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"vorticis.com.br/monitor/internal/cadastro"
	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/relatorios"
	"vorticis.com.br/monitor/internal/sefaz"
	"vorticis.com.br/monitor/internal/servicos"
	"vorticis.com.br/monitor/internal/store"
)

const Versao = "1.5.0"

type Server struct {
	M      *servicos.Monitor
	Static fs.FS
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	auth := s.autenticar()
	fiscalOuAdmin := s.autenticar("FISCAL", "INTEGRACAO")
	admin := s.autenticar("ADMIN")

	mux.HandleFunc("GET /api/v1/saude", s.saude)
	mux.HandleFunc("GET /api/v1/auth/estado", s.authEstado)
	mux.HandleFunc("POST /api/v1/auth/setup", s.authSetup)
	mux.HandleFunc("POST /api/v1/auth/login", s.authLogin)

	mux.HandleFunc("GET /api/v1/eventos/stream", auth(s.stream))
	mux.HandleFunc("GET /api/v1/dashboard/resumo", auth(s.resumo))

	mux.HandleFunc("GET /api/v1/empresas", auth(s.listarEmpresas))
	mux.HandleFunc("POST /api/v1/empresas", admin(s.salvarEmpresa))
	mux.HandleFunc("GET /api/v1/empresas/{id}", auth(s.obterEmpresa))
	mux.HandleFunc("PUT /api/v1/empresas/{id}", admin(s.salvarEmpresa))
	mux.HandleFunc("POST /api/v1/empresas/{id}/certificado", admin(s.enviarCertificado))
	mux.HandleFunc("GET /api/v1/empresas/{id}/certificado", auth(s.obterCertificado))
	mux.HandleFunc("DELETE /api/v1/empresas/{id}/certificado", admin(s.removerCertificado))

	mux.HandleFunc("GET /api/v1/documentos", auth(s.listarDocumentos))
	mux.HandleFunc("POST /api/v1/documentos/importar", fiscalOuAdmin(s.importar))
	mux.HandleFunc("GET /api/v1/documentos/{chave}", auth(s.obterDocumento))
	mux.HandleFunc("GET /api/v1/documentos/{chave}/xml", auth(s.baixarXML))
	mux.HandleFunc("GET /api/v1/documentos/{chave}/pdf", auth(s.pdf))
	mux.HandleFunc("GET /api/v1/documentos/{chave}/danfe", auth(s.pdf))
	mux.HandleFunc("POST /api/v1/documentos/exportar", auth(s.exportar))
	mux.HandleFunc("GET /api/v1/cadastro/cnpj/{cnpj}", auth(s.consultarCNPJ))
	mux.HandleFunc("POST /api/v1/cadastro/certificado", admin(s.consultarPorCertificado))
	mux.HandleFunc("GET /api/v1/relatorios", auth(func(w http.ResponseWriter, r *http.Request) { jsonOK(w, relatorios.Catalogo) }))
	mux.HandleFunc("GET /api/v1/relatorios/{id}", auth(s.relatorio))
	mux.HandleFunc("GET /api/v1/certificados/instalados", admin(s.certificadosInstalados))
	mux.HandleFunc("GET /api/v1/documentos/{chave}/ocorrencias", auth(s.ocorrencias))
	mux.HandleFunc("GET /api/v1/documentos/{chave}/eventos", auth(s.eventos))
	mux.HandleFunc("GET /api/v1/documentos/{chave}/correcoes", auth(s.correcoes))
	mux.HandleFunc("POST /api/v1/documentos/{chave}/consultar-sefaz", fiscalOuAdmin(s.consultarSefaz))
	mux.HandleFunc("POST /api/v1/documentos/{chave}/revalidar", fiscalOuAdmin(s.revalidar))
	mux.HandleFunc("POST /api/v1/documentos/{chave}/manifestacao", s.autenticar("FISCAL")(s.manifestar))
	mux.HandleFunc("POST /api/v1/documentos/{chave}/conciliar", fiscalOuAdmin(s.conciliar))

	mux.HandleFunc("POST /api/v1/validacao/pre-emissao", fiscalOuAdmin(s.preEmissao))
	mux.HandleFunc("GET /api/v1/regras", auth(s.listarRegras))

	mux.HandleFunc("GET /api/v1/captura/status", auth(s.capturaStatus))
	mux.HandleFunc("POST /api/v1/captura/{id}/nfse", fiscalOuAdmin(s.capturaNFSe))
	mux.HandleFunc("GET /api/v1/procuradores", auth(s.listarProcuradores))
	mux.HandleFunc("POST /api/v1/procuradores", admin(s.incluirProcurador))
	mux.HandleFunc("DELETE /api/v1/procuradores/{id}", admin(s.removerProcurador))
	mux.HandleFunc("PUT /api/v1/procuradores/{id}", admin(s.atualizarProcurador))
	mux.HandleFunc("POST /api/v1/procuradores/{id}/nfe", fiscalOuAdmin(s.capturaNFeProcurador))
	mux.HandleFunc("POST /api/v1/captura/{id}/executar", fiscalOuAdmin(s.capturaExecutar))
	mux.HandleFunc("GET /api/v1/sefaz/status", auth(s.sefazStatus))

	mux.HandleFunc("GET /api/v1/pedidos-compra", auth(s.listarPedidos))
	mux.HandleFunc("POST /api/v1/pedidos-compra", fiscalOuAdmin(s.salvarPedido))
	mux.HandleFunc("GET /api/v1/conciliacoes", auth(s.listarConciliacoes))

	mux.HandleFunc("GET /api/v1/parametros/uf", auth(s.listarParamUF))
	mux.HandleFunc("PUT /api/v1/parametros/uf", s.autenticar("FISCAL")(s.salvarParamUF))
	mux.HandleFunc("GET /api/v1/parametros/mva", auth(s.listarMVA))
	mux.HandleFunc("POST /api/v1/parametros/mva", s.autenticar("FISCAL")(s.salvarMVA))
	mux.HandleFunc("DELETE /api/v1/parametros/mva/{id}", s.autenticar("FISCAL")(s.removerMVA))
	mux.HandleFunc("POST /api/v1/parametros/ncm", s.autenticar("FISCAL")(s.importarNCM))

	mux.HandleFunc("GET /api/v1/integracoes/webhooks", admin(s.listarWebhooks))
	mux.HandleFunc("POST /api/v1/integracoes/webhooks", admin(s.salvarWebhook))
	mux.HandleFunc("DELETE /api/v1/integracoes/webhooks/{id}", admin(s.removerWebhook))

	mux.HandleFunc("GET /api/v1/usuarios", admin(s.listarUsuarios))
	mux.HandleFunc("POST /api/v1/usuarios", admin(s.criarUsuario))
	mux.HandleFunc("GET /api/v1/auditoria", admin(s.auditoria))
	mux.HandleFunc("GET /api/v1/config", admin(s.obterConfig))
	mux.HandleFunc("GET /api/v1/sistema/pastas", admin(s.navegarPastas))
	mux.HandleFunc("GET /api/v1/backup", admin(func(w http.ResponseWriter, r *http.Request) { jsonOK(w, s.M.EstadoDoBackup()) }))
	mux.HandleFunc("POST /api/v1/backup/executar", admin(func(w http.ResponseWriter, r *http.Request) {
		e, err := s.M.FazerBackup()
		if err != nil {
			erro(w, 400, err.Error())
			return
		}
		jsonOK(w, e)
	}))
	mux.HandleFunc("POST /api/v1/sistema/pastas", admin(s.criarPasta))
	mux.HandleFunc("PUT /api/v1/config", admin(s.salvarConfig))

	if s.Static != nil {
		arquivos := http.FileServerFS(s.Static)
		// Sem cache: a janela do programa (WebView2) guarda cache entre versões e
		// poderia combinar a tela nova com scripts antigos.
		mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			arquivos.ServeHTTP(w, r)
		}))
	}
	return seguranca(mux)
}

func seguranca(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		h.ServeHTTP(w, r)
	})
}

// ---------- helpers ----------

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func erro(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"erro": msg})
}

func ler(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 10<<20)).Decode(v)
}

func idPath(r *http.Request, nome string) int64 {
	n, _ := strconv.ParseInt(r.PathValue(nome), 10, 64)
	return n
}

func filtro(r *http.Request) store.FiltroDocumentos {
	q := r.URL.Query()
	f := store.FiltroDocumentos{Status: q.Get("status"), Modelo: q.Get("modelo"), Direcao: q.Get("direcao"), De: q.Get("de"), Ate: q.Get("ate"), Busca: q.Get("q"), SoComErro: q.Get("com_erro") == "1"}
	f.EmpresaID, _ = strconv.ParseInt(q.Get("empresa_id"), 10, 64)
	f.Pagina, _ = strconv.Atoi(q.Get("pagina"))
	f.PorPagina, _ = strconv.Atoi(q.Get("por_pagina"))
	if p := q.Get("periodo"); p != "" {
		if t, err := time.Parse("2006-01", p); err == nil {
			f.De, f.Ate = t.Format("2006-01-02"), t.AddDate(0, 1, -1).Format("2006-01-02")
		}
	}
	return f
}

// ---------- auth / saúde ----------

func (s *Server) saude(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{"status": "ok", "versao": Versao, "hora": time.Now().Format(time.RFC3339)})
}

func (s *Server) authEstado(w http.ResponseWriter, r *http.Request) {
	emps, _ := s.M.Store.ListarEmpresas()
	semCert := 0
	for _, e := range emps {
		if !e.TemCertificado {
			semCert++
		}
	}
	jsonOK(w, map[string]any{"precisa_setup": s.M.Store.ContarUsuarios() == 0, "versao": Versao, "empresas": len(emps), "empresas_sem_certificado": semCert})
}

func (s *Server) authSetup(w http.ResponseWriter, r *http.Request) {
	if s.M.Store.ContarUsuarios() > 0 {
		erro(w, http.StatusConflict, "configuração inicial já realizada")
		return
	}
	var in struct{ Login, Nome, Senha string }
	if err := ler(r, &in); err != nil || in.Login == "" || len(in.Senha) < 8 {
		erro(w, http.StatusBadRequest, "informe login e senha com ao menos 8 caracteres")
		return
	}
	h, _ := HashSenha(in.Senha)
	u := &store.Usuario{Login: in.Login, Nome: in.Nome, Perfil: "ADMIN", SenhaHash: h}
	if err := s.M.Store.CriarUsuario(u); err != nil {
		erro(w, http.StatusBadRequest, err.Error())
		return
	}
	s.M.Store.Auditar(u.Login, "SETUP", "usuario", u.Login, "administrador criado")
	jsonOK(w, map[string]any{"token": s.emitirToken(u.Login, u.Perfil), "usuario": u})
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Login, Senha string
		Lembrar      bool
		Credencial   string
	}
	ler(r, &in)
	var u *store.Usuario
	var err error
	if in.Credencial != "" { // login com a senha salva neste computador
		ss, e := s.lerAssinado(in.Credencial)
		if e == nil && ss.Tipo == "c" {
			u, err = s.M.Store.UsuarioPorLogin(ss.Login)
			if err == nil && (!u.Ativo || resumoSenha(u.SenhaHash) != ss.Chk) {
				err = errors.New("senha alterada")
			}
		} else {
			err = e
		}
		if err != nil {
			erro(w, http.StatusUnauthorized, "a senha salva não vale mais (expirou ou foi alterada) — digite a senha novamente")
			return
		}
	} else {
		u, err = s.M.Store.UsuarioPorLogin(in.Login)
		if err != nil || !u.Ativo || !conferirSenha(in.Senha, u.SenhaHash) {
			time.Sleep(400 * time.Millisecond)
			erro(w, http.StatusUnauthorized, "usuário ou senha inválidos")
			return
		}
	}
	s.M.Store.Auditar(u.Login, "LOGIN", "usuario", u.Login, r.RemoteAddr)
	out := map[string]any{"token": s.emitirToken(u.Login, u.Perfil), "usuario": u}
	if in.Lembrar || in.Credencial != "" {
		out["credencial"] = s.emitirCredencial(u.Login, u.SenhaHash)
	}
	jsonOK(w, out)
}

// ---------- tempo real (SSE) ----------

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		erro(w, 500, "streaming não suportado")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	c := s.M.Hub.Assinar()
	defer s.M.Hub.Cancelar(c)
	fmt.Fprint(w, "event: conectado\ndata: {}\n\n")
	fl.Flush()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case e := <-c:
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Tipo, b)
			fl.Flush()
		}
	}
}

func (s *Server) resumo(w http.ResponseWriter, r *http.Request) {
	res, err := s.M.Store.Resumo(filtro(r))
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	jsonOK(w, res)
}

// ---------- empresas e certificado ----------

func (s *Server) listarEmpresas(w http.ResponseWriter, r *http.Request) {
	e, err := s.M.Store.ListarEmpresas()
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	jsonOK(w, e)
}

func (s *Server) obterEmpresa(w http.ResponseWriter, r *http.Request) {
	e, err := s.M.Store.ObterEmpresa(idPath(r, "id"))
	if err != nil {
		erro(w, 404, "empresa não encontrada")
		return
	}
	jsonOK(w, e)
}

func (s *Server) salvarEmpresa(w http.ResponseWriter, r *http.Request) {
	var e store.Empresa
	if err := ler(r, &e); err != nil {
		erro(w, 400, "JSON inválido")
		return
	}
	if id := idPath(r, "id"); id > 0 {
		e.ID = id
	}
	if !fiscal.ValidaCNPJ(e.CNPJ) && !fiscal.ValidaCPF(e.CNPJ) {
		erro(w, 400, "CNPJ inválido")
		return
	}
	if _, ok := fiscal.CodigoPorUF[strings.ToUpper(e.UF)]; !ok {
		erro(w, 400, "UF inválida")
		return
	}
	if ok, _ := fiscal.ValidaIE(e.IE, e.UF); e.IE != "" && !ok {
		erro(w, 400, "Inscrição Estadual inválida para "+e.UF)
		return
	}
	switch e.Regime {
	case "", "SN", "MEI", "LP", "LR":
	default:
		erro(w, 400, "regime deve ser SN, MEI, LP ou LR")
		return
	}
	if e.NFSeModo == "PROCURACAO" {
		if _, err := s.M.Store.ObterProcurador(e.NFSeProcuradorID); err != nil {
			erro(w, 400, "escolha o procurador (certificado com procuração eletrônica) para consultar a NFS-e")
			return
		}
	}
	if err := s.M.Store.SalvarEmpresa(&e); err != nil {
		erro(w, 400, err.Error())
		return
	}
	s.M.Store.SalvarNFSeEmpresa(e.ID, e.NFSeModo, e.NFSeProcuradorID)
	s.M.InvalidarCliente(e.ID)
	s.M.Store.Auditar(usuario(r), "SALVAR_EMPRESA", "empresa", strconv.FormatInt(e.ID, 10), e.CNPJ)
	jsonOK(w, e)
}

func (s *Server) enviarCertificado(w http.ResponseWriter, r *http.Request) {
	id := idPath(r, "id")
	emp, err := s.M.Store.ObterEmpresa(id)
	if err != nil {
		erro(w, 404, "empresa não encontrada")
		return
	}
	cert, pfx, senha, tipo, err := lerCertificado(r)
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	if !cert.Valido() {
		erro(w, 400, "certificado fora da validade ("+cert.Validade.Format("02/01/2006")+")")
		return
	}
	if cert.CNPJ != "" && len(cert.CNPJ) == 14 && cert.CNPJ[:8] != emp.CNPJ[:8] {
		erro(w, 400, "CNPJ-base do certificado ("+cert.CNPJ+") difere da empresa ("+emp.CNPJ+") — rejeição 213 na SEFAZ")
		return
	}
	pc, err := s.M.Cofre.Cifrar(pfx)
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	sc, _ := s.M.Cofre.Cifrar([]byte(senha))
	c := &store.Certificado{EmpresaID: id, Tipo: tipo, Titular: cert.Titular, CNPJ: cert.CNPJ, Serial: cert.Serial, Emissor: cert.Emissor,
		ValidoDe: cert.ValidoDe.Format(time.RFC3339), ValidoAte: cert.Validade.Format(time.RFC3339), PFX: pc, Senha: sc}
	if err := s.M.Store.SalvarCertificado(c); err != nil {
		erro(w, 500, err.Error())
		return
	}
	s.M.InvalidarCliente(id)
	s.M.Store.Auditar(usuario(r), "CERTIFICADO_INCLUIDO", "empresa", strconv.FormatInt(id, 10), cert.Titular+" válido até "+c.ValidoAte)
	c.DiasRestantes = int(time.Until(cert.Validade).Hours() / 24)
	jsonOK(w, c)
}

func (s *Server) obterCertificado(w http.ResponseWriter, r *http.Request) {
	c, err := s.M.Store.CertificadoAtivo(idPath(r, "id"))
	if err != nil {
		erro(w, 404, "nenhum certificado cadastrado")
		return
	}
	jsonOK(w, c)
}

func (s *Server) removerCertificado(w http.ResponseWriter, r *http.Request) {
	id := idPath(r, "id")
	s.M.Store.RemoverCertificado(id)
	s.M.InvalidarCliente(id)
	s.M.Store.Auditar(usuario(r), "CERTIFICADO_REMOVIDO", "empresa", strconv.FormatInt(id, 10), "")
	jsonOK(w, map[string]bool{"ok": true})
}

// lerCertificado obtém o certificado do pedido: arquivo .pfx (campos "arquivo" e
// "senha") ou certificado instalado no Windows (campo "impressao" = thumbprint).
// Instalado com chave exportável → vira .pfx no cofre (tipo A1); caso contrário
// fica no Windows e o cofre guarda só o thumbprint (tipo REPOSITORIO).
func lerCertificado(r *http.Request) (cert *sefaz.Certificado, pfx []byte, senha, tipo string, err error) {
	if e := r.ParseMultipartForm(10 << 20); e != nil {
		r.ParseForm()
	}
	if imp := strings.TrimSpace(r.FormValue("impressao")); imp != "" {
		senha = gerarSenha()
		if pfx, err = sefaz.ExportarRepositorio(imp, senha); err == nil {
			if cert, err = sefaz.CarregarPFX(pfx, senha); err == nil {
				return cert, pfx, senha, "A1", nil
			}
		}
		if cert, err = sefaz.CarregarRepositorio(imp); err != nil {
			return nil, nil, "", "", err
		}
		return cert, []byte(cert.Impressao), "", "REPOSITORIO", nil
	}
	f, _, e := r.FormFile("arquivo")
	if e != nil {
		return nil, nil, "", "", errors.New("escolha um certificado instalado no Windows ou envie o arquivo .pfx/.p12 com a senha")
	}
	defer f.Close()
	pfx, _ = io.ReadAll(io.LimitReader(f, 10<<20))
	senha = r.FormValue("senha")
	cert, err = sefaz.CarregarPFX(pfx, senha)
	return cert, pfx, senha, "A1", err
}

func gerarSenha() string {
	b := make([]byte, 18)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *Server) certificadosInstalados(w http.ResponseWriter, r *http.Request) {
	l, err := sefaz.ListarRepositorio()
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	jsonOK(w, l)
}

// relatorio gera um relatório do período (?formato=json|pdf|xlsx|xls|csv).
func (s *Server) relatorio(w http.ResponseWriter, r *http.Request) {
	f := filtro(r)
	f.Todos = true
	id := r.PathValue("id")
	if id == "manifestacao" {
		f.Direcao, f.Modelo = "E", "NFE"
	}
	docs, _, err := s.M.Store.ListarDocumentos(f)
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	data := func(iso string) string {
		if t, err := time.Parse("2006-01-02", iso); err == nil {
			return t.Format("02/01/2006")
		}
		return "…"
	}
	partes := []string{"Período: " + data(f.De) + " a " + data(f.Ate)}
	if f.EmpresaID > 0 {
		if e, err := s.M.Store.ObterEmpresa(f.EmpresaID); err == nil {
			partes = append(partes, "Empresa: "+e.RazaoSocial+" ("+relatorios.FormatarDoc(e.CNPJ)+")")
		}
	} else {
		partes = append(partes, "Todas as empresas")
	}
	if f.Direcao != "" && id != "manifestacao" {
		partes = append(partes, map[string]string{"S": "Emitidos", "E": "Recebidos"}[f.Direcao])
	}
	if f.Modelo != "" && id != "manifestacao" {
		partes = append(partes, "Modelo: "+strings.ReplaceAll(f.Modelo, ",", ", "))
	}
	if f.Status != "" {
		partes = append(partes, "Situação: "+strings.ReplaceAll(f.Status, ",", ", "))
	}
	itens := func(id int64) []store.Item { l, _ := s.M.Store.Itens(id); return l }
	rel, err := relatorios.Gerar(id, docs, itens, strings.Join(partes, " · "))
	if err != nil {
		erro(w, 404, err.Error())
		return
	}
	disponiveis := append([]relatorios.Coluna(nil), rel.Colunas...)
	var sel []int
	for _, x := range strings.Split(r.URL.Query().Get("colunas"), ",") {
		if i, err := strconv.Atoi(strings.TrimSpace(x)); err == nil {
			sel = append(sel, i)
		}
	}
	rel.Selecionar(sel)
	formato := r.URL.Query().Get("formato")
	if formato == "" || formato == "json" {
		jsonOK(w, map[string]any{"relatorio": rel, "disponiveis": disponiveis, "selecionadas": rel.Indices})
		return
	}
	ct, ok := relatorios.Formatos[formato]
	if !ok {
		erro(w, 400, "formato inválido: use pdf, xlsx, xls ou csv")
		return
	}
	var buf bytes.Buffer
	if err := rel.Escrever(&buf, formato); err != nil {
		erro(w, 400, err.Error())
		return
	}
	nome := fmt.Sprintf("%s_%s_a_%s.%s", id, f.De, f.Ate, formato)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`"`)
	w.Write(buf.Bytes())
}

// ---------- documentos ----------

func (s *Server) listarDocumentos(w http.ResponseWriter, r *http.Request) {
	f := filtro(r)
	docs, total, err := s.M.Store.ListarDocumentos(f)
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]any{"total": total, "pagina": max(1, f.Pagina), "itens": docs})
}

func (s *Server) importar(w http.ResponseWriter, r *http.Request) {
	origem := r.URL.Query().Get("origem")
	if origem == "" {
		origem = "IMPORTACAO"
		if usuario(r) == "api" {
			origem = "ERP"
		}
	}
	var res []servicos.ResultadoImportacao
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/") {
		if err := r.ParseMultipartForm(200 << 20); err != nil {
			erro(w, 400, err.Error())
			return
		}
		for _, fhs := range r.MultipartForm.File {
			for _, fh := range fhs {
				f, err := fh.Open()
				if err != nil {
					continue
				}
				b, _ := io.ReadAll(io.LimitReader(f, 200<<20))
				f.Close()
				res = append(res, s.M.ImportarArquivo(fh.Filename, b, origem)...)
			}
		}
	} else {
		b, _ := io.ReadAll(io.LimitReader(r.Body, 200<<20))
		res = s.M.ImportarArquivo(r.URL.Query().Get("nome"), b, origem)
	}
	jsonOK(w, map[string]any{"processados": len(res), "resultados": res})
}

func (s *Server) doc(w http.ResponseWriter, r *http.Request) *store.Documento {
	d, err := s.M.Store.ObterDocumento(r.PathValue("chave"))
	if err != nil {
		erro(w, 404, "documento não encontrado")
		return nil
	}
	return d
}

func (s *Server) obterDocumento(w http.ResponseWriter, r *http.Request) {
	d := s.doc(w, r)
	if d == nil {
		return
	}
	its, _ := s.M.Store.Itens(d.ID)
	ocs, _ := s.M.Store.Ocorrencias(d.ID)
	evs, _ := s.M.Store.Eventos(d.Chave)
	var rej *fiscal.Rejeicao
	if d.CStat >= 200 || d.Status == servicos.StDenegada {
		x := fiscal.DescreveRejeicao(d.CStat)
		rej = &x
	}
	jsonOK(w, map[string]any{"documento": d, "itens": its, "ocorrencias": ocs, "eventos": evs, "rejeicao": rej, "chave_info": fiscal.AnalisaChave(d.Chave)})
}

func (s *Server) baixarXML(w http.ResponseWriter, r *http.Request) {
	d := s.doc(w, r)
	if d == nil {
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+d.Chave+`.xml"`)
	io.WriteString(w, d.XML)
}

func (s *Server) ocorrencias(w http.ResponseWriter, r *http.Request) {
	d := s.doc(w, r)
	if d == nil {
		return
	}
	o, _ := s.M.Store.Ocorrencias(d.ID)
	jsonOK(w, o)
}

func (s *Server) eventos(w http.ResponseWriter, r *http.Request) {
	e, _ := s.M.Store.Eventos(r.PathValue("chave"))
	jsonOK(w, e)
}

// correcoes devolve as correções sugeridas (patches) que o ERP pode aplicar antes de reemitir.
func (s *Server) correcoes(w http.ResponseWriter, r *http.Request) {
	d := s.doc(w, r)
	if d == nil {
		return
	}
	ocs, _ := s.M.Store.Ocorrencias(d.ID)
	type patch struct {
		Regra    string `json:"regra"`
		Item     int    `json:"item,omitempty"`
		Campo    string `json:"campo"`
		Atual    string `json:"valor_atual"`
		Sugerido string `json:"valor_sugerido"`
		Acao     string `json:"acao"`
		Motivo   string `json:"motivo"`
	}
	out := []patch{}
	for _, o := range ocs {
		if o.Severidade == "INFO" {
			continue
		}
		if o.Esperado != "" || o.AcaoCorrecao != "" {
			out = append(out, patch{Regra: o.Regra, Item: o.NItem, Campo: o.Campo, Atual: o.Informado, Sugerido: o.Esperado, Acao: o.AcaoCorrecao, Motivo: o.Mensagem})
		}
	}
	if d.CStat >= 200 {
		rj := fiscal.DescreveRejeicao(d.CStat)
		out = append(out, patch{Regra: fmt.Sprintf("SEFAZ-%d", d.CStat), Campo: "-", Acao: rj.AcaoAutomatica, Motivo: rj.Sugestao})
	}
	jsonOK(w, map[string]any{"chave": d.Chave, "correcoes": out, "observacao": "Aplicar no ERP e reassinar o XML; o Monitor não altera documentos assinados."})
}

func (s *Server) consultarSefaz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	res, err := s.M.ConsultarSituacao(ctx, r.PathValue("chave"))
	if err != nil {
		code := 502
		if errors.Is(err, servicos.ErrSemCertificado) {
			code = 412
		}
		erro(w, code, err.Error())
		return
	}
	jsonOK(w, res)
}

func (s *Server) revalidar(w http.ResponseWriter, r *http.Request) {
	res, err := s.M.Revalidar(r.PathValue("chave"), nil)
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	jsonOK(w, res)
}

func (s *Server) manifestar(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Tipo          string `json:"tipo"` // 210210 | 210200 | 210220 | 210240 (ou CIENCIA, CONFIRMACAO, DESCONHECIMENTO, NAO_REALIZADA)
		Justificativa string `json:"justificativa"`
	}
	ler(r, &in)
	tipos := map[string]string{"CIENCIA": "210210", "CONFIRMACAO": "210200", "DESCONHECIMENTO": "210220", "NAO_REALIZADA": "210240"}
	if t, ok := tipos[strings.ToUpper(in.Tipo)]; ok {
		in.Tipo = t
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	res, err := s.M.Manifestar(ctx, r.PathValue("chave"), in.Tipo, in.Justificativa)
	if err != nil {
		code := 502
		if errors.Is(err, servicos.ErrSemCertificado) {
			code = 412
		}
		erro(w, code, err.Error())
		return
	}
	s.M.Store.Auditar(usuario(r), "MANIFESTACAO", "documento", r.PathValue("chave"), in.Tipo)
	jsonOK(w, res)
}

func (s *Server) conciliar(w http.ResponseWriter, r *http.Request) {
	var in struct {
		OrdemRecebimento string `json:"ordem_recebimento"`
	}
	ler(r, &in)
	c, err := s.M.Conciliar(r.PathValue("chave"), in.OrdemRecebimento)
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	jsonOK(w, c)
}

// ---------- validação e simulação ----------

func (s *Server) preEmissao(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(io.LimitReader(r.Body, 20<<20))
	d, res, err := s.M.Validar(b)
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	out := map[string]any{"tipo": d.Tipo, "aprovado": !res.Bloqueante, "resultado": res}
	if d.NFe != nil {
		out["chave"] = d.NFe.Chave()
	}
	if res.Bloqueante {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(out)
		return
	}
	jsonOK(w, out)
}

func (s *Server) listarRegras(w http.ResponseWriter, r *http.Request) {
	var out []map[string]string
	for _, rg := range s.M.Motor.Regras() {
		out = append(out, map[string]string{"id": rg.ID, "grupo": rg.Grupo, "descricao": rg.Descricao})
	}
	jsonOK(w, out)
}

// ---------- rejeições / alertas ----------

// ---------- captura / SEFAZ ----------

func (s *Server) capturaStatus(w http.ResponseWriter, r *http.Request) {
	n, _ := s.M.Store.ListarNSU()
	jsonOK(w, n)
}

func (s *Server) capturaNFSe(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	res, err := s.M.CapturarNFSe(ctx, idPath(r, "id"), r.URL.Query().Get("forcar") == "1")
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	jsonOK(w, res)
}

func (s *Server) listarProcuradores(w http.ResponseWriter, r *http.Request) {
	l, err := s.M.Store.ListarProcuradores()
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	jsonOK(w, l)
}

// incluirProcurador guarda o certificado (e-CPF/e-CNPJ) de quem tem procuração
// eletrônica das empresas — normalmente o escritório de contabilidade.
func (s *Server) incluirProcurador(w http.ResponseWriter, r *http.Request) {
	cert, pfx, senha, tipo, err := lerCertificado(r)
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	if !cert.Valido() {
		erro(w, 400, "certificado fora da validade ("+cert.Validade.Format("02/01/2006")+")")
		return
	}
	pc, err := s.M.Cofre.Cifrar(pfx)
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	sc, _ := s.M.Cofre.Cifrar([]byte(senha))
	nome := strings.TrimSpace(r.FormValue("nome"))
	if nome == "" {
		nome = strings.TrimSpace(strings.SplitN(cert.Titular, ":", 2)[0])
	}
	p := &store.Procurador{Nome: nome, Documento: cert.CNPJ, Tipo: tipo, Titular: cert.Titular, Emissor: cert.Emissor,
		ValidoAte: cert.Validade.Format(time.RFC3339), PFX: pc, Senha: sc,
		UF: strings.ToUpper(r.FormValue("uf")), NFeAutXML: r.FormValue("nfe_autxml") != ""}
	if p.UF != "" && fiscal.CodigoPorUF[p.UF] == "" {
		erro(w, 400, "UF do procurador inválida")
		return
	}
	if err := s.M.Store.SalvarProcurador(p); err != nil {
		erro(w, 500, err.Error())
		return
	}
	s.M.Store.Auditar(usuario(r), "PROCURADOR_INCLUIDO", "procurador", strconv.FormatInt(p.ID, 10), cert.Titular)
	jsonOK(w, p)
}

func (s *Server) atualizarProcurador(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UF        string `json:"uf"`
		NFeAutXML bool   `json:"nfe_autxml"`
	}
	ler(r, &in)
	if in.UF != "" && fiscal.CodigoPorUF[strings.ToUpper(in.UF)] == "" {
		erro(w, 400, "UF inválida")
		return
	}
	if in.NFeAutXML && in.UF == "" {
		erro(w, 400, "informe a UF do procurador para buscar as NF-e")
		return
	}
	if err := s.M.Store.AtualizarProcurador(idPath(r, "id"), in.UF, in.NFeAutXML); err != nil {
		erro(w, 500, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

func (s *Server) capturaNFeProcurador(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	res, err := s.M.CapturarNFeProcurador(ctx, idPath(r, "id"), r.URL.Query().Get("forcar") == "1")
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	jsonOK(w, res)
}

func (s *Server) removerProcurador(w http.ResponseWriter, r *http.Request) {
	id := idPath(r, "id")
	s.M.Store.RemoverProcurador(id)
	s.M.InvalidarProcurador(id)
	s.M.Store.Auditar(usuario(r), "PROCURADOR_REMOVIDO", "procurador", strconv.FormatInt(id, 10), "")
	jsonOK(w, map[string]bool{"ok": true})
}

func (s *Server) capturaExecutar(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	res, err := s.M.CapturarDistDFe(ctx, idPath(r, "id"), r.URL.Query().Get("forcar") == "1")
	if err != nil {
		code := 502
		if errors.Is(err, servicos.ErrSemCertificado) {
			code = 412
		}
		erro(w, code, err.Error())
		return
	}
	jsonOK(w, res)
}

func (s *Server) sefazStatus(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("empresa_id"), 10, 64)
	cli, emp, err := s.M.ClienteSefaz(id)
	if err != nil {
		code := 502
		if errors.Is(err, servicos.ErrSemCertificado) {
			code = 412
		}
		erro(w, code, err.Error())
		return
	}
	uf := strings.ToUpper(r.URL.Query().Get("uf"))
	if uf == "" {
		uf = emp.UF
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out := map[string]any{}
	for _, tp := range []string{"1", fiscal.SVCPorUF[uf]} {
		res, err := cli.StatusServico(ctx, uf, tp)
		if err != nil {
			out[sefaz.Autorizador(uf, tp)] = map[string]any{"online": false, "erro": err.Error()}
			continue
		}
		out[res.Autorizador] = res
	}
	jsonOK(w, out)
}

// ---------- compras / conciliação ----------

func (s *Server) listarPedidos(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.URL.Query().Get("empresa_id"), 10, 64)
	p, err := s.M.Store.Pedidos(id, r.URL.Query().Get("fornecedor"), r.URL.Query().Get("numero"), r.URL.Query().Get("abertos") == "1")
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	if p == nil {
		p = []*store.Pedido{}
	}
	jsonOK(w, p)
}

func (s *Server) salvarPedido(w http.ResponseWriter, r *http.Request) {
	var p store.Pedido
	if err := ler(r, &p); err != nil || p.Numero == "" || p.EmpresaID == 0 {
		erro(w, 400, "informe empresa_id, numero, fornecedor_cnpj e itens")
		return
	}
	if err := s.M.Store.SalvarPedido(&p); err != nil {
		erro(w, 400, err.Error())
		return
	}
	jsonOK(w, p)
}

func (s *Server) listarConciliacoes(w http.ResponseWriter, r *http.Request) {
	c, err := s.M.Store.ListarConciliacoes(500)
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	jsonOK(w, c)
}

// ---------- parâmetros ----------

func (s *Server) listarParamUF(w http.ResponseWriter, r *http.Request) {
	p, _ := s.M.Store.ListarParamUF()
	jsonOK(w, p)
}

func (s *Server) salvarParamUF(w http.ResponseWriter, r *http.Request) {
	var ps []fiscal.ParametroUF
	if err := ler(r, &ps); err != nil {
		erro(w, 400, "envie uma lista de parâmetros por UF")
		return
	}
	for _, p := range ps {
		if p.DifalMetodo != "BASE_UNICA" {
			p.DifalMetodo = "BASE_DUPLA"
		}
		s.M.Store.SalvarParamUF(p)
	}
	s.M.Store.Auditar(usuario(r), "PARAM_UF", "param_uf", "", fmt.Sprintf("%d UFs", len(ps)))
	jsonOK(w, map[string]int{"atualizadas": len(ps)})
}

func (s *Server) listarMVA(w http.ResponseWriter, r *http.Request) {
	p, _ := s.M.Store.ListarMVA()
	jsonOK(w, p)
}

func (s *Server) salvarMVA(w http.ResponseWriter, r *http.Request) {
	var p store.ParamMVA
	if err := ler(r, &p); err != nil || p.NCM == "" || p.UFDestino == "" {
		erro(w, 400, "informe ncm, uf_destino e mva_original")
		return
	}
	if err := s.M.Store.SalvarMVA(&p); err != nil {
		erro(w, 400, err.Error())
		return
	}
	s.M.Store.Auditar(usuario(r), "PARAM_MVA", "param_mva", strconv.FormatInt(p.ID, 10), p.NCM+"/"+p.UFDestino)
	jsonOK(w, p)
}

func (s *Server) removerMVA(w http.ResponseWriter, r *http.Request) {
	s.M.Store.RemoverMVA(idPath(r, "id"))
	jsonOK(w, map[string]bool{"ok": true})
}

// importarNCM recebe CSV "ncm;descricao;aliq_ipi" (TIPI vigente).
func (s *Server) importarNCM(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(io.LimitReader(r.Body, 50<<20))
	cr := csv.NewReader(bytes.NewReader(b))
	cr.Comma, cr.FieldsPerRecord, cr.LazyQuotes = ';', -1, true
	var linhas [][3]string
	for {
		rec, err := cr.Read()
		if err != nil {
			break
		}
		var l [3]string
		copy(l[:], rec)
		linhas = append(linhas, l)
	}
	n, err := s.M.Store.ImportarNCM(linhas)
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	jsonOK(w, map[string]int{"ncms": n})
}

// ---------- relatórios / escrituração ----------

// ---------- PDF e exportação ----------

func (s *Server) pdf(w http.ResponseWriter, r *http.Request) {
	d := s.doc(w, r)
	if d == nil {
		return
	}
	its, _ := s.M.Store.Itens(d.ID)
	nome := d.Chave
	if d.Numero != "" {
		nome = d.Modelo + "_" + d.Numero + "_" + d.Chave
	}
	w.Header().Set("Content-Type", "application/pdf")
	disp := "inline"
	if r.URL.Query().Get("baixar") == "1" {
		disp = "attachment"
	}
	w.Header().Set("Content-Disposition", disp+`; filename="`+nome+`.pdf"`)
	if err := relatorios.PDF(w, d, its); err != nil {
		erro(w, 500, err.Error())
	}
}

// exportar gera um ZIP com os documentos selecionados (chaves) em XML e/ou PDF.
// Aceita JSON {"chaves":[...],"formato":"xml|pdf|ambos"} ou formulário com os mesmos campos.
func (s *Server) exportar(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Chaves  []string `json:"chaves"`
		Formato string   `json:"formato"`
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		if err := ler(r, &in); err != nil {
			erro(w, 400, "JSON inválido")
			return
		}
	} else {
		r.ParseForm()
		in.Formato = r.FormValue("formato")
		for _, c := range strings.Split(r.FormValue("chaves"), ",") {
			if c = strings.TrimSpace(c); c != "" {
				in.Chaves = append(in.Chaves, c)
			}
		}
	}
	if len(in.Chaves) == 0 {
		erro(w, 400, "selecione ao menos um documento")
		return
	}
	comXML, comPDF := in.Formato != "pdf", in.Formato != "xml"
	nome := "documentos_" + time.Now().Format("20060102_150405")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`.zip"`)
	n, err := relatorios.ExportarZIP(w, s.M.Store, in.Chaves, comXML, comPDF)
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	s.M.Store.Auditar(usuario(r), "EXPORTAR", "documento", "", fmt.Sprintf("%d documento(s) em %s", n, map[bool]string{true: "XML+PDF", false: in.Formato}[comXML && comPDF]))
}

// ---------- cadastro pelo CNPJ ----------

// consultarCNPJ busca os dados da Receita e as IEs (SEFAZ com o certificado de
// uma empresa já cadastrada de mesmo CNPJ-base, quando houver).
func (s *Server) consultarCNPJ(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	d, err := cadastro.ConsultarCNPJ(ctx, r.PathValue("cnpj"))
	if err != nil {
		erro(w, 502, err.Error())
		return
	}
	emps, _ := s.M.Store.ListarEmpresas()
	for _, e := range emps {
		if e.TemCertificado && len(e.CNPJ) == 14 && e.CNPJ[:8] == d.CNPJ[:8] {
			if cli, _, err := s.M.ClienteSefaz(e.ID); err == nil {
				s.mesclarSEFAZ(ctx, cli, d)
			}
			break
		}
	}
	d.Avisos = append(d.Avisos, "Conferência manual da IE: "+cadastro.URLSintegra(d.UF))
	jsonOK(w, d)
}

// consultarPorCertificado lê o certificado A1 (multipart: arquivo, senha), identifica o
// CNPJ e consulta Receita + SEFAZ da UF. O certificado não é gravado aqui.
func (s *Server) consultarPorCertificado(w http.ResponseWriter, r *http.Request) {
	cert, _, _, _, err := lerCertificado(r)
	if err != nil {
		erro(w, 400, err.Error())
		return
	}
	if !cert.Valido() {
		erro(w, 400, "certificado fora da validade ("+cert.Validade.Format("02/01/2006")+")")
		return
	}
	if !fiscal.ValidaCNPJ(cert.CNPJ) {
		erro(w, 400, "não foi possível identificar o CNPJ no certificado (e-CNPJ)")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	d, err := cadastro.ConsultarCNPJ(ctx, cert.CNPJ)
	if err != nil {
		d = &cadastro.Dados{CNPJ: cert.CNPJ, RazaoSocial: strings.TrimSpace(strings.SplitN(cert.Titular, ":", 2)[0])}
		d.Avisos = append(d.Avisos, "Dados da Receita indisponíveis: "+err.Error())
	}
	if d.UF != "" {
		s.mesclarSEFAZ(ctx, sefaz.NovoCliente(cert, 1, s.M.Cfg.VerificarTLSSefaz, s.M.Cfg.CadeiaICPBrasil), d)
	}
	d.Avisos = append(d.Avisos, "Conferência manual da IE: "+cadastro.URLSintegra(d.UF))
	jsonOK(w, map[string]any{"dados": d, "certificado": map[string]any{"titular": cert.Titular, "cnpj": cert.CNPJ, "emissor": cert.Emissor,
		"valido_ate": cert.Validade.Format(time.RFC3339), "dias_restantes": int(time.Until(cert.Validade).Hours() / 24)}})
}

func (s *Server) mesclarSEFAZ(ctx context.Context, cli *sefaz.Cliente, d *cadastro.Dados) {
	cs, err := cli.ConsultaCadastro(ctx, d.UF, d.CNPJ)
	if err != nil {
		d.Avisos = append(d.Avisos, "Consulta de cadastro na SEFAZ-"+d.UF+": "+err.Error())
		return
	}
	var ies []cadastro.Inscricao
	for _, c := range cs {
		ies = append(ies, cadastro.Inscricao{IE: fiscal.SoDigitos(c.IE), UF: c.UF, Ativa: c.CSit == "1", Fonte: "SEFAZ-" + d.UF + " (certificado)", Regime: c.XRegApur})
		if d.CodMun == "" {
			d.CodMun = c.Ender.CMun
		}
	}
	d.MesclarSEFAZ(d.UF, ies)
}

// ---------- integrações / usuários / config ----------

func (s *Server) listarWebhooks(w http.ResponseWriter, r *http.Request) {
	l, _ := s.M.Store.ListarWebhooks()
	jsonOK(w, l)
}

func (s *Server) salvarWebhook(w http.ResponseWriter, r *http.Request) {
	var wh store.Webhook
	if err := ler(r, &wh); err != nil || !strings.HasPrefix(wh.URL, "http") {
		erro(w, 400, "informe url (http/https) e eventos")
		return
	}
	if wh.Eventos == "" {
		wh.Eventos = "*"
	}
	s.M.Store.SalvarWebhook(&wh)
	jsonOK(w, wh)
}

func (s *Server) removerWebhook(w http.ResponseWriter, r *http.Request) {
	s.M.Store.RemoverWebhook(idPath(r, "id"))
	jsonOK(w, map[string]bool{"ok": true})
}

func (s *Server) listarUsuarios(w http.ResponseWriter, r *http.Request) {
	u, _ := s.M.Store.ListarUsuarios()
	jsonOK(w, u)
}

func (s *Server) criarUsuario(w http.ResponseWriter, r *http.Request) {
	var in struct{ Login, Nome, Senha, Perfil string }
	if err := ler(r, &in); err != nil || in.Login == "" || len(in.Senha) < 8 {
		erro(w, 400, "informe login, senha (mín. 8) e perfil")
		return
	}
	switch in.Perfil {
	case "ADMIN", "FISCAL", "CONSULTA":
	default:
		in.Perfil = "CONSULTA"
	}
	h, _ := HashSenha(in.Senha)
	u := &store.Usuario{Login: in.Login, Nome: in.Nome, Perfil: in.Perfil, SenhaHash: h, Ativo: true}
	if err := s.M.Store.CriarUsuario(u); err != nil {
		erro(w, 400, err.Error())
		return
	}
	s.M.Store.Auditar(usuario(r), "CRIAR_USUARIO", "usuario", u.Login, u.Perfil)
	jsonOK(w, u)
}

func (s *Server) auditoria(w http.ResponseWriter, r *http.Request) {
	a, _ := s.M.Store.ListarAuditoria(500)
	jsonOK(w, a)
}

func (s *Server) obterConfig(w http.ResponseWriter, r *http.Request) {
	c := *s.M.Cfg
	c.SegredoSessao = "********"
	jsonOK(w, map[string]any{"config": c, "diretorio_dados": s.M.Cfg.DataDir})
}

func (s *Server) salvarConfig(w http.ResponseWriter, r *http.Request) {
	c := *s.M.Cfg
	if err := ler(r, &c); err != nil {
		erro(w, 400, "JSON inválido")
		return
	}
	c.SegredoSessao, c.DataDir = s.M.Cfg.SegredoSessao, s.M.Cfg.DataDir
	if r.URL.Query().Get("nova_api_key") == "1" {
		c.APIKey = "vtx_" + strconv.FormatInt(time.Now().UnixNano(), 36) + fmt.Sprintf("%x", time.Now().UnixNano()*7919)
	}
	*s.M.Cfg = c
	if err := s.M.Cfg.Salvar(); err != nil {
		erro(w, 500, err.Error())
		return
	}
	s.M.Store.Auditar(usuario(r), "CONFIG", "config", "", "configuração atualizada")
	jsonOK(w, map[string]any{"ok": true, "reiniciar_para": []string{"host", "porta", "tls_cert", "tls_key"}})
}
