package servicos

import (
	"testing"
	"time"

	"vorticis.com.br/monitor/internal/store"
)

func TestAntecipar(t *testing.T) {
	ha := func(d time.Duration) string { return time.Now().Add(-d).Format(time.RFC3339) }
	if !antecipar(store.NSU{UltimaConsulta: ha(70 * time.Minute), UltimoCStat: 137}, time.Hour, 656) {
		t.Fatal("intervalo de 60 min vencido deve permitir a consulta")
	}
	if antecipar(store.NSU{UltimaConsulta: ha(30 * time.Minute), UltimoCStat: 137}, time.Hour, 656) {
		t.Fatal("antes do intervalo não pode consultar")
	}
	if antecipar(store.NSU{UltimaConsulta: ha(5 * time.Hour), UltimoCStat: 656}, time.Hour, 656) {
		t.Fatal("bloqueio 656 exige aguardar a próxima consulta definida")
	}
	if antecipar(store.NSU{}, time.Hour) {
		t.Fatal("sem consulta anterior quem decide é proxima_consulta")
	}
}
