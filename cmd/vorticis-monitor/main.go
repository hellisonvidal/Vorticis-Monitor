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

func main() {
	if executandoComoServico() {
		executarServico()
		return
	}
	cmd := "run"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "run":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		if err := executar(ctx); err != nil {
			log.Fatal(err)
		}
	case "service":
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

func executar(ctx context.Context) error {
	cfg, st, cofre, err := abrir()
	if err != nil {
		return err
	}
	defer st.DB.Close()
	mon := servicos.Novo(st, cfg, cofre)
	go mon.Iniciar(ctx)
	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Porta)),
		Handler:           (&api.Server{M: mon, Static: web.Static()}).Handler(),
		ReadHeaderTimeout: 15 * time.Second,
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
	log.Printf("Vorticis Monitor %s — painel em %s://%s (dados: %s)", api.Versao, esquema, srv.Addr, cfg.DataDir)
	if cfg.TLSCert != "" {
		err = srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
	} else {
		err = srv.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
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
