// Package config carrega a configuração do Vorticis Monitor (config.json no diretório de dados).
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Host  string `json:"host"` // 127.0.0.1 (somente local) | 0.0.0.0 (rede/servidor web)
	Porta int    `json:"porta"`
	// TLS opcional para o modo web (certificado do servidor, não o e-CNPJ).
	TLSCert string `json:"tls_cert,omitempty"`
	TLSKey  string `json:"tls_key,omitempty"`

	APIKey        string `json:"api_key"`        // integração ERP/escrituração (header X-API-Key)
	SegredoSessao string `json:"segredo_sessao"` // assinatura HMAC dos tokens de sessão

	VerificarTLSSefaz bool   `json:"verificar_tls_sefaz"`
	CadeiaICPBrasil   string `json:"cadeia_icp_brasil,omitempty"` // PEM adicional de ACs (ICP-Brasil)

	IntervaloDistDFeMin     int     `json:"intervalo_distdfe_min"`
	IntervaloVarreduraSeg   int     `json:"intervalo_varredura_seg"`
	PrazoCancelNFeHoras     int     `json:"prazo_cancelamento_nfe_horas"`
	PrazoCancelNFCeMin      int     `json:"prazo_cancelamento_nfce_min"`
	PrazoManifestacaoDias   int     `json:"prazo_manifestacao_dias"`
	PrazoCienciaDias        int     `json:"prazo_ciencia_dias"`
	AntecedenciaAlertaHoras int     `json:"antecedencia_alerta_horas"`
	DiasAlertaCertificado   int     `json:"dias_alerta_certificado"`
	ToleranciaPrecoPct      float64 `json:"tolerancia_preco_pct"`
	ToleranciaQtdPct        float64 `json:"tolerancia_qtd_pct"`
	PastaMonitorada         string  `json:"pasta_monitorada,omitempty"` // importação automática de XML (ERP grava aqui)
	ConsultarPendentesSefaz bool    `json:"consultar_pendentes_sefaz"`

	DataDir string `json:"-"`
}

func padrao() *Config {
	return &Config{
		Host: "127.0.0.1", Porta: 8765, VerificarTLSSefaz: true,
		IntervaloDistDFeMin: 60, IntervaloVarreduraSeg: 60,
		PrazoCancelNFeHoras: 24, PrazoCancelNFCeMin: 30,
		PrazoManifestacaoDias: 180, PrazoCienciaDias: 10, AntecedenciaAlertaHoras: 6,
		DiasAlertaCertificado: 30, ToleranciaPrecoPct: 1, ToleranciaQtdPct: 0,
		ConsultarPendentesSefaz: true,
	}
}

// DiretorioDados retorna %ProgramData%\Vorticis\Monitor no Windows ou ./dados.
func DiretorioDados() string {
	if d := os.Getenv("VORTICIS_DATA"); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, "Vorticis", "Monitor")
		}
	}
	return "dados"
}

func aleatorio(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Carregar lê ou cria a configuração.
func Carregar(dir string) (*Config, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	for _, sub := range []string{"xml", "logs", "backup", "importar"} {
		os.MkdirAll(filepath.Join(dir, sub), 0o700)
	}
	c := padrao()
	p := filepath.Join(dir, "config.json")
	if b, err := os.ReadFile(p); err == nil {
		if err := json.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}
	if c.APIKey == "" {
		c.APIKey = "vtx_" + aleatorio(20)
	}
	if c.SegredoSessao == "" {
		c.SegredoSessao = aleatorio(32)
	}
	if c.PastaMonitorada == "" {
		c.PastaMonitorada = filepath.Join(dir, "importar")
	}
	if v := os.Getenv("VORTICIS_HOST"); v != "" {
		c.Host = v
	}
	c.DataDir = dir
	return c, c.Salvar()
}

func (c *Config) Salvar() error {
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(filepath.Join(c.DataDir, "config.json"), b, 0o600)
}
