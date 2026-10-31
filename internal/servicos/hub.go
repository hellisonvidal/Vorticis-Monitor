package servicos

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"vorticis.com.br/monitor/internal/store"
)

// Evento em tempo real (SSE para o painel e webhooks para ERP/escrituração).
type Evento struct {
	Tipo  string `json:"tipo"` // documento.novo | documento.status | captura.ciclo
	Dados any    `json:"dados"`
	Em    string `json:"em"`
}

type Hub struct {
	mu    sync.Mutex
	subs  map[chan Evento]struct{}
	store *store.Store
	http  *http.Client
}

func NovoHub(s *store.Store) *Hub {
	return &Hub{subs: map[chan Evento]struct{}{}, store: s, http: &http.Client{Timeout: 10 * time.Second}}
}

func (h *Hub) Assinar() chan Evento {
	c := make(chan Evento, 32)
	h.mu.Lock()
	h.subs[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *Hub) Cancelar(c chan Evento) {
	h.mu.Lock()
	delete(h.subs, c)
	h.mu.Unlock()
	close(c)
}

// Publicar envia o evento aos painéis conectados e aos webhooks inscritos.
func (h *Hub) Publicar(tipo string, dados any) {
	e := Evento{Tipo: tipo, Dados: dados, Em: time.Now().Format(time.RFC3339)}
	h.mu.Lock()
	for c := range h.subs {
		select {
		case c <- e:
		default: // cliente lento: descarta
		}
	}
	h.mu.Unlock()
	go h.webhooks(e)
}

func (h *Hub) webhooks(e Evento) {
	ws, err := h.store.ListarWebhooks()
	if err != nil {
		return
	}
	body, _ := json.Marshal(e)
	for _, w := range ws {
		if !strings.Contains(","+w.Eventos+",", ","+e.Tipo+",") && w.Eventos != "*" {
			continue
		}
		req, _ := http.NewRequest(http.MethodPost, w.URL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if w.Segredo != "" {
			m := hmac.New(sha256.New, []byte(w.Segredo))
			m.Write(body)
			req.Header.Set("X-Vorticis-Assinatura", "sha256="+hex.EncodeToString(m.Sum(nil)))
		}
		if resp, err := h.http.Do(req); err != nil {
			log.Printf("webhook %s: %v", w.URL, err)
		} else {
			resp.Body.Close()
		}
	}
}
