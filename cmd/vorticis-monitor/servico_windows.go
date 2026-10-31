//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func executandoComoServico() bool {
	ok, _ := svc.IsWindowsService()
	return ok
}

type handler struct{}

func (handler) Execute(_ []string, req <-chan svc.ChangeRequest, st chan<- svc.Status) (bool, uint32) {
	st <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fim := make(chan error, 1)
	go func() { fim <- executar(ctx, nil) }()
	st <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case err := <-fim:
			if err != nil {
				log.Printf("serviço encerrado com erro: %v", err)
				return true, 1
			}
			return false, 0
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				st <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				st <- svc.Status{State: svc.StopPending}
				cancel()
				select {
				case <-fim:
				case <-time.After(15 * time.Second):
				}
				return false, 0
			}
		}
	}
}

func executarServico() {
	// O serviço (opcional, instalado via linha de comando como administrador) roda
	// como SYSTEM; usa a pasta comum de dados da máquina.
	if os.Getenv("VORTICIS_DATA") == "" {
		os.Setenv("VORTICIS_DATA", filepath.Join(os.Getenv("ProgramData"), "Vorticis", "Monitor"))
	}
	if err := svc.Run(nomeServico, handler{}); err != nil {
		log.Fatal(err)
	}
}

func controlarServico(acao string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("execute como administrador: %w", err)
	}
	defer m.Disconnect()
	switch acao {
	case "install":
		exe, _ := os.Executable()
		if s, err := m.OpenService(nomeServico); err == nil {
			s.Close()
			return nil
		}
		s, err := m.CreateService(nomeServico, exe, mgr.Config{
			DisplayName: "Vorticis Monitor Fiscal", Description: "Monitor fiscal em tempo real: NF-e, NFC-e, CT-e, MDF-e, NFS-e e SPED.",
			StartType: mgr.StartAutomatic, DelayedAutoStart: true,
		})
		if err != nil {
			return err
		}
		defer s.Close()
		s.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 10 * time.Second}, {Type: mgr.ServiceRestart, Delay: 30 * time.Second}}, 86400)
		return nil
	case "uninstall":
		s, err := m.OpenService(nomeServico)
		if err != nil {
			return nil
		}
		defer s.Close()
		s.Control(svc.Stop)
		return s.Delete()
	case "start":
		s, err := m.OpenService(nomeServico)
		if err != nil {
			return err
		}
		defer s.Close()
		return s.Start()
	case "stop":
		s, err := m.OpenService(nomeServico)
		if err != nil {
			return err
		}
		defer s.Close()
		_, err = s.Control(svc.Stop)
		return err
	}
	return fmt.Errorf("ação desconhecida: %s", acao)
}
