package servicos

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Backup automático. Na pasta de destino é criada a estrutura:
//
//	VorticisMonitor-Backup/
//	  banco/vorticis-AAAAMMDD-HHMM.zip   cópia consistente do banco + config + chave do cofre
//	  xml/AAAAMM/<chave>.xml             espelho dos XMLs (copia só os novos/alterados)
//
// As cópias do banco além de BackupManter são apagadas; os XMLs nunca são apagados.

// EstadoBackup é gravado em <dados>/backup/estado.json.
type EstadoBackup struct {
	Ultimo       string `json:"ultimo,omitempty"`
	Arquivo      string `json:"arquivo,omitempty"`
	XMLCopiados  int    `json:"xml_copiados"`
	XMLTotal     int    `json:"xml_total"`
	Erro         string `json:"erro,omitempty"`
	Proximo      string `json:"proximo,omitempty"`
	EmAndamento  bool   `json:"em_andamento"`
	DuracaoSeg   int    `json:"duracao_seg"`
	TamanhoBanco int64  `json:"tamanho_banco"`
}

var backupMu sync.Mutex

func (m *Monitor) arquivoEstadoBackup() string {
	return filepath.Join(m.Cfg.DataDir, "backup", "estado.json")
}

// EstadoDoBackup devolve a situação do último backup e a data do próximo.
func (m *Monitor) EstadoDoBackup() EstadoBackup {
	var e EstadoBackup
	if b, err := os.ReadFile(m.arquivoEstadoBackup()); err == nil {
		json.Unmarshal(b, &e)
	}
	e.EmAndamento = !backupMu.TryLock()
	if !e.EmAndamento {
		backupMu.Unlock()
	}
	if p := m.intervaloBackup(); p > 0 && m.Cfg.BackupPasta != "" {
		t, err := time.Parse(time.RFC3339, e.Ultimo)
		if err != nil || e.Erro != "" {
			t = time.Now().Add(-p)
		}
		e.Proximo = t.Add(p).Format(time.RFC3339)
	} else {
		e.Proximo = ""
	}
	return e
}

func (m *Monitor) intervaloBackup() time.Duration {
	switch m.Cfg.BackupFrequencia {
	case "diario":
		return 24 * time.Hour
	case "semanal":
		return 7 * 24 * time.Hour
	}
	return 0
}

// BackupSeVencido executa o backup agendado quando chegou a hora (chamado pelo agendador).
func (m *Monitor) BackupSeVencido() {
	p := m.intervaloBackup()
	if p == 0 || m.Cfg.BackupPasta == "" {
		return
	}
	e := m.EstadoDoBackup()
	if t, err := time.Parse(time.RFC3339, e.Ultimo); err == nil && time.Since(t) < p && e.Erro == "" {
		return
	}
	// Após erro, tenta de novo no máximo a cada hora.
	if e.Erro != "" {
		if info, err := os.Stat(m.arquivoEstadoBackup()); err == nil && time.Since(info.ModTime()) < time.Hour {
			return
		}
	}
	go func() {
		if _, err := m.FazerBackup(); err != nil {
			log.Printf("backup: %v", err)
		}
	}()
}

// FazerBackup copia banco e XMLs para a pasta configurada.
func (m *Monitor) FazerBackup() (EstadoBackup, error) {
	if !backupMu.TryLock() {
		return m.EstadoDoBackup(), errors.New("já existe um backup em andamento")
	}
	defer backupMu.Unlock()
	inicio := time.Now()
	e := EstadoBackup{}
	if b, err := os.ReadFile(m.arquivoEstadoBackup()); err == nil {
		json.Unmarshal(b, &e)
	}
	err := m.executarBackup(&e)
	e.DuracaoSeg = int(time.Since(inicio).Seconds())
	if err != nil {
		e.Erro = err.Error()
	} else {
		e.Erro, e.Ultimo = "", time.Now().Format(time.RFC3339)
	}
	os.MkdirAll(filepath.Dir(m.arquivoEstadoBackup()), 0o755)
	b, _ := json.MarshalIndent(e, "", "  ")
	os.WriteFile(m.arquivoEstadoBackup(), b, 0o600)
	m.Hub.Publicar("backup.concluido", e)
	if err == nil {
		m.Store.Auditar("sistema", "BACKUP", "backup", e.Arquivo, fmt.Sprintf("%d XML novos de %d", e.XMLCopiados, e.XMLTotal))
	}
	r := m.EstadoDoBackup()
	r.EmAndamento = false
	return r, err
}

func (m *Monitor) executarBackup(e *EstadoBackup) error {
	if strings.TrimSpace(m.Cfg.BackupPasta) == "" {
		return errors.New("escolha a pasta de destino do backup em Configurações › Backup")
	}
	if st, err := os.Stat(m.Cfg.BackupPasta); err != nil || !st.IsDir() {
		return fmt.Errorf("pasta de backup indisponível: %s (pendrive/rede desconectado?)", m.Cfg.BackupPasta)
	}
	raiz := filepath.Join(m.Cfg.BackupPasta, "VorticisMonitor-Backup")
	dados, _ := filepath.Abs(m.Cfg.DataDir)
	if r, _ := filepath.Abs(raiz); strings.HasPrefix(r+string(filepath.Separator), dados+string(filepath.Separator)) {
		return errors.New("a pasta de backup não pode ficar dentro da pasta de dados do monitor — escolha outro disco, pendrive, rede ou pasta da nuvem")
	}
	bancoDir := filepath.Join(raiz, "banco")
	if err := os.MkdirAll(bancoDir, 0o755); err != nil {
		return fmt.Errorf("sem permissão para gravar em %s: %w", raiz, err)
	}

	// 1) Banco: cópia consistente com VACUUM INTO (funciona com o programa em uso).
	tmp := filepath.Join(m.Cfg.DataDir, "backup", "vorticis-copia.db")
	os.MkdirAll(filepath.Dir(tmp), 0o755)
	os.Remove(tmp)
	if _, err := m.Store.DB.Exec(`VACUUM INTO ?`, tmp); err != nil {
		return fmt.Errorf("cópia do banco: %w", err)
	}
	defer os.Remove(tmp)
	nome := "vorticis-" + time.Now().Format("20060102-1504") + ".zip"
	destino := filepath.Join(bancoDir, nome)
	if err := zipar(destino+".parcial", map[string]string{
		"vorticis.db":  tmp,
		"config.json":  filepath.Join(m.Cfg.DataDir, "config.json"),
		"chave.mestra": filepath.Join(m.Cfg.DataDir, "chave.mestra"),
		"LEIA-ME.txt":  "",
	}); err != nil {
		os.Remove(destino + ".parcial")
		return err
	}
	if err := os.Rename(destino+".parcial", destino); err != nil {
		return err
	}
	if st, err := os.Stat(destino); err == nil {
		e.TamanhoBanco = st.Size()
	}
	e.Arquivo = destino

	// 2) Cópias antigas do banco além do limite.
	manter := m.Cfg.BackupManter
	if manter <= 0 {
		manter = 7
	}
	if l, err := filepath.Glob(filepath.Join(bancoDir, "vorticis-*.zip")); err == nil && len(l) > manter {
		sort.Strings(l)
		for _, f := range l[:len(l)-manter] {
			os.Remove(f)
		}
	}

	// 3) XMLs: espelho incremental.
	e.XMLCopiados, e.XMLTotal = 0, 0
	origem := filepath.Join(m.Cfg.DataDir, "xml")
	return filepath.WalkDir(origem, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(origem, p)
		alvo := filepath.Join(raiz, "xml", rel)
		e.XMLTotal++
		si, _ := d.Info()
		if di, err := os.Stat(alvo); err == nil && si != nil && di.Size() == si.Size() {
			return nil
		}
		os.MkdirAll(filepath.Dir(alvo), 0o755)
		if err := copiar(p, alvo); err != nil {
			return fmt.Errorf("copiando %s: %w", rel, err)
		}
		e.XMLCopiados++
		return nil
	})
}

const leiaMeBackup = `Backup do Vorticis Monitor

Para restaurar no MESMO computador:
 1. Feche o Vorticis Monitor.
 2. Copie vorticis.db, config.json e chave.mestra deste ZIP para a pasta de dados
    (%LOCALAPPDATA%\Vorticis\Monitor ou a pasta "dados" da versão portátil),
    substituindo os arquivos existentes (apague vorticis.db-wal e vorticis.db-shm se existirem).
 3. Copie a pasta "xml" do backup para a pasta de dados.

Em OUTRO computador, as notas, relatórios e configurações são restaurados do mesmo modo,
mas os certificados digitais precisam ser incluídos de novo (a chave do cofre é protegida
pelo Windows do computador de origem), exceto quando o backup vem da versão portátil.
`

func zipar(destino string, arquivos map[string]string) error {
	f, err := os.Create(destino)
	if err != nil {
		return fmt.Errorf("sem permissão para gravar o backup: %w", err)
	}
	z := zip.NewWriter(f)
	nomes := make([]string, 0, len(arquivos))
	for n := range arquivos {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)
	for _, n := range nomes {
		w, err := z.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			f.Close()
			return err
		}
		if arquivos[n] == "" {
			io.WriteString(w, strings.ReplaceAll(leiaMeBackup, "\n", "\r\n"))
			continue
		}
		src, err := os.Open(arquivos[n])
		if err != nil {
			if os.IsNotExist(err) && n != "vorticis.db" {
				continue
			}
			f.Close()
			return err
		}
		_, err = io.Copy(w, src)
		src.Close()
		if err != nil {
			f.Close()
			return err
		}
	}
	if err := z.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func copiar(de, para string) error {
	src, err := os.Open(de)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(para + ".parcial")
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(para + ".parcial")
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return os.Rename(para+".parcial", para)
}
