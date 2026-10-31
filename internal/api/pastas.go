package api

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// navegarPastas lista subpastas (e arquivos com as extensões pedidas) para o
// navegador de pastas das Configurações. caminho="*" lista as unidades no Windows.
func (s *Server) navegarPastas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c := strings.TrimSpace(q.Get("caminho"))
	if c == "" {
		c = s.M.Cfg.DataDir
	}
	var exts []string
	for _, e := range strings.Split(strings.ToLower(q.Get("ext")), ",") {
		if e = strings.TrimSpace(e); e != "" {
			exts = append(exts, e)
		}
	}
	type item struct {
		Nome    string `json:"nome"`
		Caminho string `json:"caminho"`
	}
	out := map[string]any{"pastas": []item{}, "arquivos": []item{}}
	home, _ := os.UserHomeDir()
	atalhos := []item{{"Dados do monitor", s.M.Cfg.DataDir}}
	for _, a := range []struct{ n, p string }{{"Área de trabalho", "Desktop"}, {"Documentos", "Documents"}, {"Downloads", "Downloads"}} {
		if p := filepath.Join(home, a.p); existeDir(p) {
			atalhos = append(atalhos, item{a.n, p})
		}
	}
	if runtime.GOOS == "windows" {
		atalhos = append(atalhos, item{"Este computador (unidades)", "*"})
	}
	out["atalhos"] = atalhos
	if c == "*" && runtime.GOOS == "windows" {
		var us []item
		for l := 'A'; l <= 'Z'; l++ {
			if p := string(l) + `:\`; existeDir(p) {
				us = append(us, item{p, p})
			}
		}
		out["caminho"], out["pai"], out["pastas"] = "*", "", us
		jsonOK(w, out)
		return
	}
	abs, err := filepath.Abs(c)
	if err != nil || !existeDir(abs) {
		erro(w, 400, "pasta não encontrada: "+c)
		return
	}
	es, err := os.ReadDir(abs)
	if err != nil {
		erro(w, 400, "sem permissão para abrir a pasta: "+abs)
		return
	}
	pastas, arqs := []item{}, []item{}
	for _, e := range es {
		n := e.Name()
		if strings.HasPrefix(n, ".") || strings.HasPrefix(n, "$") {
			continue
		}
		p := filepath.Join(abs, n)
		if e.IsDir() || (e.Type()&os.ModeSymlink != 0 && existeDir(p)) {
			pastas = append(pastas, item{n, p})
		} else if len(exts) > 0 {
			for _, x := range exts {
				if strings.HasSuffix(strings.ToLower(n), x) {
					arqs = append(arqs, item{n, p})
					break
				}
			}
		}
	}
	ord := func(l []item) {
		sort.Slice(l, func(i, j int) bool { return strings.ToLower(l[i].Nome) < strings.ToLower(l[j].Nome) })
	}
	ord(pastas)
	ord(arqs)
	pai := filepath.Dir(abs)
	if pai == abs {
		pai = ""
		if runtime.GOOS == "windows" {
			pai = "*"
		}
	}
	out["caminho"], out["pai"], out["pastas"], out["arquivos"] = abs, pai, pastas, arqs
	jsonOK(w, out)
}

// criarPasta cria uma subpasta (botão "Nova pasta" do navegador).
func (s *Server) criarPasta(w http.ResponseWriter, r *http.Request) {
	var in struct{ Caminho, Nome string }
	ler(r, &in)
	n := strings.TrimSpace(in.Nome)
	if n == "" || strings.ContainsAny(n, `\/:*?"<>|`) || !existeDir(in.Caminho) {
		erro(w, 400, "nome de pasta inválido")
		return
	}
	p := filepath.Join(in.Caminho, n)
	if err := os.MkdirAll(p, 0o755); err != nil {
		erro(w, 400, err.Error())
		return
	}
	jsonOK(w, map[string]string{"caminho": p})
}

func existeDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
