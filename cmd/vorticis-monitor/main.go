// Vorticis Monitor — monitor fiscal (NF-e, NFC-e, CT-e, MDF-e, NFS-e, SPED).
//
// Uso:
//
//	vorticis-monitor run                 executa em primeiro plano (console)
//	vorticis-monitor service install     instala o serviço Windows "VorticisMonitor"
//	vorticis-monitor service uninstall   remove o serviço
//	vorticis-monitor service start|stop  controla o serviço
//	vorticis-monitor reset-senha LOGIN   redefine a senha de um usuário
//	vorticis-monitor version
//
// Modo portátil: ativado quando o executável contém "portatil" no nome, quando
// existe o arquivo "vorticis.portatil" ao lado dele ou com a opção --portatil.
// Os dados ficam em <pasta do executável>\dados e nenhum serviço é instalado.
// No Windows o programa abre em janela própria (WebView2), sem navegador.
// Opções: --minimizado (inicia minimizado, usado na inicialização automática) e
// --sem-janela (apenas o servidor interno, para acesso pela rede).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"vorticis.com.br/monitor/internal/api"
	"vorticis.com.br/monitor/internal/config"
	"vorticis.com.br/monitor/internal/keystore"
	"vorticis.com.br/monitor/internal/sefaz"
	"vorticis.com.br/monitor/internal/servicos"
	"vorticis.com.br/monitor/internal/store"
	"vorticis.com.br/monitor/web"
)

const nomeServico = "VorticisMonitor"

var (
	portatil     bool
	minimizado bool
	semJanela  bool
)

// args remove as opções globais (--portatil, --sem-navegador) da linha de comando.
func args() []string {
	var out []string
	for _, a := range os.Args[1:] {
		switch a {
		case "--portatil":
			portatil = true
		case "--minimizado":
			minimizado = true
		case "--sem-janela", "--sem-navegador":
			semJanela = true
		default:
			out = append(out, a)
		}
	}
	return out
}

func detectarPortatil() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	if strings.Contains(strings.ToLower(filepath.Base(exe)), "portatil") {
		portatil = true
	}
	if _, err := os.Stat(filepath.Join(dir, "vorticis.portatil")); err == nil {
		portatil = true
	}
	if portatil {
		if os.Getenv("VORTICIS_DATA") == "" {
			os.Setenv("VORTICIS_DATA", filepath.Join(dir, "dados"))
		}
		// A chave-mestra acompanha a pasta (não fica presa à DPAPI desta máquina).
		keystore.SemProtecaoMaquina = true
	}
}

func main() {
	if executandoComoServico() {
		executarServico()
		return
	}
	a := args()
	detectarPortatil()
	cmd := "run"
	if len(a) > 0 {
		cmd = a[0]
	}
	os.Args = append(os.Args[:1], a...)
	switch cmd {
	case "run":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := executarComJanela(ctx, cancel); err != nil {
			erroFatal(err)
		}
	case "service":
		if portatil {
			log.Fatal("a versão portátil não instala serviço; use o instalador VorticisMonitor-Setup")
		}
		if len(os.Args) < 3 {
			log.Fatal("uso: vorticis-monitor service install|uninstall|start|stop")
		}
		if err := controlarServico(os.Args[2]); err != nil {
			log.Fatal(err)
		}
	case "reset-senha":
		if len(os.Args) < 4 {
			log.Fatal("uso: vorticis-monitor reset-senha LOGIN NOVA_SENHA")
		}
		resetSenha(os.Args[2], os.Args[3])
	case "version", "--version", "-v":
		fmt.Println("Vorticis Monitor", api.Versao)
	default:
		fmt.Println("comandos: run | service install|uninstall|start|stop | reset-senha | version")
	}
}

func abrir() (*config.Config, *store.Store, *keystore.Cofre, error) {
	dir := config.DiretorioDados()
	cfg, err := config.Carregar(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	lf, err := os.OpenFile(filepath.Join(dir, "logs", "vorticis-"+time.Now().Format("200601")+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		log.SetOutput(multiWriter{os.Stderr, lf})
	}
	st, err := store.Open(filepath.Join(dir, "vorticis.db"))
	if err != nil {
		return nil, nil, nil, err
	}
	cofre, err := keystore.Abrir(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := sefaz.CarregarOverrides(dir); err != nil {
		log.Printf("webservices.json inválido: %v", err)
	}
	return cfg, st, cofre, nil
}

type multiWriter []interface{ Write([]byte) (int, error) }

func (m multiWriter) Write(p []byte) (int, error) {
	for _, w := range m {
		w.Write(p)
	}
	return len(p), nil
}

// executar sobe o monitor (rotinas + servidor interno do painel) e bloqueia até ctx
// ser cancelado. aoIniciar recebe o endereço do painel e o diretório de dados.
func executar(ctx context.Context, aoIniciar func(url, dataDir string)) error {
	cfg, st, cofre, err := abrir()
	if err != nil {
		return err
	}
	defer st.DB.Close()
	mon := servicos.Novo(st, cfg, cofre)
	go mon.Iniciar(ctx)
	srv := &http.Server{
		Handler:           (&api.Server{M: mon, Static: web.Static()}).Handler(),
		ReadHeaderTimeout: 15 * time.Second,
	}
	ln, err := escutar(cfg.Host, cfg.Porta, !executandoComoServico())
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		sh, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		srv.Shutdown(sh)
	}()
	esquema := "http"
	if cfg.TLSCert != "" {
		esquema = "https"
	}
	porta := ln.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("%s://localhost:%d/", esquema, porta)
	modo := "instalado"
	if portatil {
		modo = "portátil"
	}
	log.Printf("Vorticis Monitor %s (%s) — painel em %s (escuta %s, dados: %s)", api.Versao, modo, url, ln.Addr(), cfg.DataDir)
	if aoIniciar != nil {
		aoIniciar(url, cfg.DataDir)
	}
	if cfg.TLSCert != "" {
		err = srv.ServeTLS(ln, cfg.TLSCert, cfg.TLSKey)
	} else {
		err = srv.Serve(ln)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// escutar abre a porta configurada; em uso interativo tenta as 10 seguintes se estiver ocupada.
func escutar(host string, porta int, tentarOutras bool) (net.Listener, error) {
	tentativas := 1
	if tentarOutras {
		tentativas = 11
	}
	var ultimo error
	for i := 0; i < tentativas; i++ {
		ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(porta+i)))
		if err == nil {
			if i > 0 {
				log.Printf("porta %d ocupada; usando %d", porta, porta+i)
			}
			return ln, nil
		}
		ultimo = err
	}
	return nil, ultimo
}

func resetSenha(login, senha string) {
	_, st, _, err := abrir()
	if err != nil {
		log.Fatal(err)
	}
	h, _ := api.HashSenha(senha)
	r, err := st.DB.Exec(`UPDATE usuario SET senha_hash = ? WHERE login = ?`, h, login)
	if n, _ := r.RowsAffected(); err != nil || n == 0 {
		log.Fatal("usuário não encontrado")
	}
	fmt.Println("senha redefinida")
}
