// Package api expõe a API REST do Vorticis Monitor e serve o painel web.
package api

import (
	"bytes"
	"context"
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

	"vorticis.com.br/monitor/internal/fiscal"
	"vorticis.com.br/monitor/internal/regras"
	"vorticis.com.br/monitor/internal/relatorios"
	"vorticis.com.br/monitor/internal/sefaz"
	"vorticis.com.br/monitor/internal/servicos"
	"vorticis.com.br/monitor/internal/store"
)

const Versao = "1.0.0"

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
	mux.HandleFunc("GET /api/v1/documentos/{chave}/danfe", auth(s.danfe))
	mux.HandleFunc("GET /api/v1/documentos/{chave}/ocorrencias", auth(s.ocorrencias))
	mux.HandleFunc("GET /api/v1/documentos/{chave}/eventos", auth(s.eventos))
	mux.HandleFunc("GET /api/v1/documentos/{chave}/correcoes", auth(s.correcoes))
	mux.HandleFunc("POST /api/v1/documentos/{chave}/consultar-sefaz", fiscalOuAdmin(s.consultarSefaz))
	mux.HandleFunc("POST /api/v1/documentos/{chave}/revalidar", fiscalOuAdmin(s.revalidar))
	mux.HandleFunc("POST /api/v1/documentos/{chave}/manifestacao", s.autenticar("FISCAL")(s.manifestar))
	mux.HandleFunc("POST /api/v1/documentos/{chave}/conciliar", fiscalOuAdmin(s.conciliar))

	mux.HandleFunc("POST /api/v1/validacao/pre-emissao", fiscalOuAdmin(s.preEmissao))
	mux.HandleFunc("POST /api/v1/simulacao/difal", auth(s.simDIFAL))
	mux.HandleFunc("POST /api/v1/simulacao/icms-st", auth(s.simST))
	mux.HandleFunc("POST /api/v1/simulacao/retencoes", auth(s.simRetencoes))
	mux.HandleFunc("GET /api/v1/regras", auth(s.listarRegras))

	mux.HandleFunc("GET /api/v1/rejeicoes", auth(s.rejeicoes))
	mux.HandleFunc("GET /api/v1/rejeicoes/central", auth(s.centralRejeicoes))
	mux.HandleFunc("GET /api/v1/rejeicoes/{codigo}", auth(s.rejeicao))

	mux.HandleFunc("GET /api/v1/alertas", auth(s.alertas))
	mux.HandleFunc("POST /api/v1/alertas/{id}/lido", auth(s.alertaLido))

	mux.HandleFunc("GET /api/v1/captura/status", auth(s.capturaStatus))
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

	mux.HandleFunc("GET /api/v1/relatorios", auth(s.catalogoRelatorios))
	mux.HandleFunc("GET /api/v1/relatorios/{id}", auth(s.relatorio))
	mux.HandleFunc("GET /api/v1/escrituracao/lancamentos", auth(s.lancamentos))

	mux.HandleFunc("GET /api/v1/integracoes/webhooks", admin(s.listarWebhooks))
	mux.HandleFunc("POST /api/v1/integracoes/webhooks", admin(s.salvarWebhook))
	mux.HandleFunc("DELETE /api/v1/integracoes/webhooks/{id}", admin(s.removerWebhook))

	mux.HandleFunc("GET /api/v1/usuarios", admin(s.listarUsuarios))
	mux.HandleFunc("POST /api/v1/usuarios", admin(s.criarUsuario))
	mux.HandleFunc("GET /api/v1/auditoria", admin(s.auditoria))
	mux.HandleFunc("GET /api/v1/config", admin(s.obterConfig))
	mux.HandleFunc("PUT /api/v1/config", admin(s.salvarConfig))

	if s.Static != nil {
		mux.Handle("GET /", http.FileServerFS(s.Static))
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
		if pr, err := relatorios.ParsePeriodo(p); err == nil {
			f.De, f.Ate = pr.Ini.Format("2006-01-02"), pr.Fim.Format("2006-01-02")
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
	var in struct{ Login, Senha string }
	ler(r, &in)
	u, err := s.M.Store.UsuarioPorLogin(in.Login)
	if err != nil || !u.Ativo || !conferirSenha(in.Senha, u.SenhaHash) {
		time.Sleep(400 * time.Millisecond)
		erro(w, http.StatusUnauthorized, "usuário ou senha inválidos")
		return
	}
	s.M.Store.Auditar(u.Login, "LOGIN", "usuario", u.Login, r.RemoteAddr)
	jsonOK(w, map[string]any{"token": s.emitirToken(u.Login, u.Perfil), "usuario": u})
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
	if err := s.M.Store.SalvarEmpresa(&e); err != nil {
		erro(w, 400, err.Error())
		return
	}
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
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		erro(w, 400, "envie multipart/form-data com os campos 'arquivo' (.pfx) e 'senha'")
		return
	}
	f, _, err := r.FormFile("arquivo")
	if err != nil {
		erro(w, 400, "arquivo .pfx/.p12 não enviado")
		return
	}
	defer f.Close()
	pfx, _ := io.ReadAll(io.LimitReader(f, 10<<20))
	senha := r.FormValue("senha")
	cert, err := sefaz.CarregarPFX(pfx, senha)
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
	c := &store.Certificado{EmpresaID: id, Tipo: "A1", Titular: cert.Titular, CNPJ: cert.CNPJ, Serial: cert.Serial, Emissor: cert.Emissor,
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

func (s *Server) danfe(w http.ResponseWriter, r *http.Request) {
	d := s.doc(w, r)
	if d == nil {
		return
	}
	its, _ := s.M.Store.Itens(d.ID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	relatorios.DANFE(w, d, its)
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

func (s *Server) simDIFAL(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Base            float64 `json:"base"`
		UFOrigem        string  `json:"uf_origem"`
		UFDestino       string  `json:"uf_destino"`
		OrigemMerc      string  `json:"origem_mercadoria"`
		AliqInterna     float64 `json:"aliq_interna"`
		FCP             *float64 `json:"fcp"`
		Metodo          string  `json:"metodo"`
	}
	if err := ler(r, &in); err != nil {
		erro(w, 400, "JSON inválido")
		return
	}
	p, _ := s.M.Store.UF(strings.ToUpper(in.UFDestino))
	if in.AliqInterna == 0 {
		in.AliqInterna = p.AliqInterna
	}
	fcp := p.FCP
	if in.FCP != nil {
		fcp = *in.FCP
	}
	if in.Metodo == "" {
		in.Metodo = p.DifalMetodo
	}
	inter := fiscal.AliquotaInterestadual(strings.ToUpper(in.UFOrigem), strings.ToUpper(in.UFDestino), in.OrigemMerc)
	jsonOK(w, regras.CalculaDIFAL(in.Base, inter, in.AliqInterna, fcp, in.Metodo))
}

func (s *Server) simST(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ValorProdutos float64 `json:"valor_produtos"`
		Frete, Seguro, Outras, Desconto, IPI float64
		MVAOriginal   float64 `json:"mva_original"`
		UFOrigem      string  `json:"uf_origem"`
		UFDestino     string  `json:"uf_destino"`
		OrigemMerc    string  `json:"origem_mercadoria"`
		AliqIntra     float64 `json:"aliq_intra"`
		RedBC         float64 `json:"reducao_bc"`
		AjustarMVA    bool    `json:"ajustar_mva"`
		NCM           string  `json:"ncm"`
	}
	if err := ler(r, &in); err != nil {
		erro(w, 400, "JSON inválido")
		return
	}
	ufo, ufd := strings.ToUpper(in.UFOrigem), strings.ToUpper(in.UFDestino)
	if in.AliqIntra == 0 {
		p, _ := s.M.Store.UF(ufd)
		in.AliqIntra = p.AliqInterna
	}
	if in.MVAOriginal == 0 && in.NCM != "" {
		in.MVAOriginal, _ = s.M.Store.MVA(fiscal.SoDigitos(in.NCM), "", ufd, time.Now())
	}
	inter := in.AliqIntra
	if ufo != ufd {
		inter = fiscal.AliquotaInterestadual(ufo, ufd, in.OrigemMerc)
	}
	mva := in.MVAOriginal
	if in.AjustarMVA && ufo != ufd {
		mva = regras.MVAAjustada(in.MVAOriginal, inter, in.AliqIntra)
	}
	base := in.ValorProdutos + in.Frete + in.Seguro + in.Outras - in.Desconto
	proprio := base * inter / 100
	res := regras.CalculaICMSST(base+in.IPI, mva, in.RedBC, in.AliqIntra, proprio)
	jsonOK(w, map[string]any{"aliq_interestadual": inter, "mva_original": in.MVAOriginal, "mva_aplicada": mva, "resultado": res})
}

func (s *Server) simRetencoes(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Base     float64 `json:"base"`
		Servico  string  `json:"servico"`
		Simples  bool    `json:"prestador_simples"`
	}
	ler(r, &in)
	si := fiscal.CodigoServicoNormalizado(in.Servico)
	res := regras.CalculaRetencoes(in.Base, si, fiscal.LC116RetencaoIR1Pct[si], fiscal.LC116CessaoMaoObra[si])
	if in.Simples {
		res.IRRF, res.PIS, res.COFINS, res.CSLL = 0, 0, 0, 0
	}
	jsonOK(w, map[string]any{"servico": si, "iss_local_prestacao": fiscal.LC116LocalPrestacao[si], "retencoes": res})
}

func (s *Server) listarRegras(w http.ResponseWriter, r *http.Request) {
	var out []map[string]string
	for _, rg := range s.M.Motor.Regras() {
		out = append(out, map[string]string{"id": rg.ID, "grupo": rg.Grupo, "descricao": rg.Descricao})
	}
	jsonOK(w, out)
}

// ---------- rejeições / alertas ----------

func (s *Server) rejeicoes(w http.ResponseWriter, r *http.Request) { jsonOK(w, fiscal.ListaRejeicoes()) }

func (s *Server) rejeicao(w http.ResponseWriter, r *http.Request) {
	c, _ := strconv.Atoi(r.PathValue("codigo"))
	jsonOK(w, fiscal.DescreveRejeicao(c))
}

func (s *Server) centralRejeicoes(w http.ResponseWriter, r *http.Request) {
	f := filtro(r)
	f.Status = "REJEITADA,DENEGADA,BLOQUEADA"
	if f.PorPagina == 0 {
		f.PorPagina = 500
	}
	docs, _, err := s.M.Store.ListarDocumentos(f)
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	type grupo struct {
		Rejeicao   fiscal.Rejeicao     `json:"rejeicao"`
		Documentos []*store.Documento  `json:"documentos"`
	}
	ag := map[int]*grupo{}
	var ordem []int
	for _, d := range docs {
		c := d.CStat
		if d.Status == servicos.StValidacao {
			c = -1
		}
		if ag[c] == nil {
			rj := fiscal.DescreveRejeicao(c)
			if c == -1 {
				rj = fiscal.Rejeicao{Codigo: 0, Categoria: "pre-validacao", Oficial: "Bloqueado na pré-validação do Monitor", Explicacao: "O documento foi reprovado pelo motor de regras antes da transmissão.", Sugestao: "Veja as ocorrências do documento e aplique as correções sugeridas."}
			}
			ag[c] = &grupo{Rejeicao: rj}
			ordem = append(ordem, c)
		}
		ag[c].Documentos = append(ag[c].Documentos, d)
	}
	out := []*grupo{}
	for _, c := range ordem {
		out = append(out, ag[c])
	}
	jsonOK(w, out)
}

func (s *Server) alertas(w http.ResponseWriter, r *http.Request) {
	a, err := s.M.Store.ListarAlertas(r.URL.Query().Get("abertos") == "1", 500)
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	jsonOK(w, a)
}

func (s *Server) alertaLido(w http.ResponseWriter, r *http.Request) {
	s.M.Store.MarcarAlertaLido(idPath(r, "id"))
	jsonOK(w, map[string]bool{"ok": true})
}

// ---------- captura / SEFAZ ----------

func (s *Server) capturaStatus(w http.ResponseWriter, r *http.Request) {
	n, _ := s.M.Store.ListarNSU()
	jsonOK(w, n)
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

func (s *Server) catalogoRelatorios(w http.ResponseWriter, r *http.Request) { jsonOK(w, relatorios.Catalogo) }

func (s *Server) relatorio(w http.ResponseWriter, r *http.Request) {
	f := filtro(r)
	id := r.PathValue("id")
	nome := id + "_" + time.Now().Format("20060102_1504")
	setCSV := func() {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+nome+`.csv"`)
	}
	var err error
	switch id {
	case "livro-entradas-saidas":
		setCSV()
		err = relatorios.LivroEntradasSaidas(w, s.M.Store, f)
	case "apuracao-icms":
		setCSV()
		err = relatorios.ApuracaoICMS(w, s.M.Store, f)
	case "pis-cofins":
		setCSV()
		err = relatorios.PISCOFINS(w, s.M.Store, f)
	case "retencoes":
		setCSV()
		err = relatorios.Retencoes(w, s.M.Store, f)
	case "inconsistencias":
		setCSV()
		err = relatorios.Inconsistencias(w, s.M.Store, f)
	case "rejeicoes":
		setCSV()
		f.Status = "REJEITADA,DENEGADA,BLOQUEADA"
		err = relatorios.Documentos(w, s.M.Store, f, true)
	case "manifestacao":
		setCSV()
		f.Direcao, f.Modelo = "E", "NFE"
		f.Status = "AUTORIZADA,RESUMO"
		err = relatorios.Documentos(w, s.M.Store, f, false)
	case "contingencia":
		setCSV()
		f.Status = "CONTINGENCIA"
		err = relatorios.Documentos(w, s.M.Store, f, false)
	case "conciliacao":
		setCSV()
		err = relatorios.ConciliacaoCSV(w, s.M.Store)
	case "efd-icms-ipi", "efd-contribuicoes":
		per, e := relatorios.ParsePeriodo(r.URL.Query().Get("periodo"))
		if e != nil || f.EmpresaID == 0 {
			erro(w, 400, "informe empresa_id e periodo=AAAA-MM")
			return
		}
		var buf bytes.Buffer
		if id == "efd-icms-ipi" {
			err = relatorios.EFDICMSIPI(&buf, s.M.Store, f.EmpresaID, per, r.URL.Query().Get("leiaute"))
		} else {
			err = relatorios.EFDContribuicoes(&buf, s.M.Store, f.EmpresaID, per)
		}
		if err == nil {
			w.Header().Set("Content-Type", "text/plain; charset=iso-8859-1")
			w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ToUpper(id)+"_"+per.Ini.Format("200601")+`.txt"`)
			w.Write(paraLatin1(buf.Bytes()))
		}
	case "xml-lote":
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="xml_`+nome+`.zip"`)
		_, err = relatorios.LoteZIP(w, s.M.Store, f, r.URL.Query().Get("danfe") != "0")
	default:
		erro(w, 404, "relatório desconhecido")
		return
	}
	if err != nil {
		erro(w, 500, err.Error())
	}
}

// paraLatin1 converte UTF-8 para ISO-8859-1 (codificação exigida pelo SPED).
func paraLatin1(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for _, r := range string(b) {
		if r < 256 {
			out = append(out, byte(r))
		} else {
			out = append(out, '?')
		}
	}
	return out
}

// lancamentos é o feed para sistemas de escrituração fiscal (adaptador genérico).
func (s *Server) lancamentos(w http.ResponseWriter, r *http.Request) {
	f := filtro(r)
	f.PorPagina = 5000
	docs, _, err := s.M.Store.ListarDocumentos(f)
	if err != nil {
		erro(w, 500, err.Error())
		return
	}
	type lanc struct {
		*store.Documento
		Itens []store.Item `json:"itens"`
		CodSitSPED string  `json:"cod_sit_sped"`
	}
	out := []lanc{}
	for _, d := range docs {
		if d.Resumo || d.Status == servicos.StValidacao || d.Status == servicos.StPendente || d.Status == servicos.StRejeitada {
			continue
		}
		its, _ := s.M.Store.Itens(d.ID)
		cs := map[string]string{servicos.StCancelada: "02", servicos.StDenegada: "04", servicos.StInutilizada: "05"}[d.Status]
		if cs == "" {
			cs = "00"
		}
		out = append(out, lanc{Documento: d, Itens: its, CodSitSPED: cs})
	}
	jsonOK(w, map[string]any{"versao_layout": "vorticis-escrituracao-1", "quantidade": len(out), "lancamentos": out})
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
