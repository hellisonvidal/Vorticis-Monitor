package servicos

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"vorticis.com.br/monitor/internal/config"
	"vorticis.com.br/monitor/internal/store"
)

func TestBackup(t *testing.T) {
	dados, destino := t.TempDir(), t.TempDir()
	st, err := store.Open(filepath.Join(dados, "vorticis.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	os.MkdirAll(filepath.Join(dados, "xml", "202610"), 0o755)
	os.WriteFile(filepath.Join(dados, "xml", "202610", "a.xml"), []byte("<NFe/>"), 0o644)
	os.WriteFile(filepath.Join(dados, "config.json"), []byte("{}"), 0o600)
	cfg := &config.Config{DataDir: dados, BackupPasta: destino, BackupFrequencia: "diario", BackupManter: 2}
	m := &Monitor{Store: st, Cfg: cfg, Hub: NovoHub(st), clientes: map[int64]*clienteCache{}}
	e, err := m.FazerBackup()
	if err != nil || e.XMLCopiados != 1 || e.EmAndamento || e.Proximo == "" {
		t.Fatalf("backup: %+v %v", e, err)
	}
	if e, _ = m.FazerBackup(); e.XMLCopiados != 0 || e.XMLTotal != 1 {
		t.Fatalf("o espelho de XML deve ser incremental: %+v", e)
	}
	z, err := zip.OpenReader(e.Arquivo)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	nomes := map[string]bool{}
	for _, f := range z.File {
		nomes[f.Name] = true
	}
	if !nomes["vorticis.db"] || !nomes["LEIA-ME.txt"] || !nomes["config.json"] {
		t.Fatalf("conteúdo do ZIP: %v", nomes)
	}
	if _, err := os.Stat(filepath.Join(destino, "VorticisMonitor-Backup", "xml", "202610", "a.xml")); err != nil {
		t.Fatal("XML não copiado")
	}
	cfg.BackupPasta = filepath.Join(dados, "sub")
	os.MkdirAll(cfg.BackupPasta, 0o755)
	if _, err := m.FazerBackup(); err == nil {
		t.Fatal("deve recusar backup dentro da pasta de dados")
	}
}
