package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/fatih/color"
)

// manda resposta pro canal do jogador no redis
func (s *Server) sendToClient(canal string, tipo string, payload interface{}) {
	msg := map[string]interface{}{
		"tipo":    tipo,
		"payload": payload,
	}
	data, _ := json.Marshal(msg)

	err := s.redisClient.Publish(context.Background(), canal, data).Err()
	if err != nil {
		color.Red("Erro ao publicar no Redis (%s): %v", canal, err)
	}
}

// faz um post simples pra outro server
func (s *Server) sendToHost(host string, endpoint string, payload interface{}) error {
	url := fmt.Sprintf("http://%s%s", host, endpoint)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}
	return nil
}

// broadcast de mensangens
func (s *Server) broadcastToServers(endpoint string, payload interface{}) {
	s.muLiveServers.RLock()
	targets := make([]string, 0)
	for id, alive := range s.liveServers {
		if alive && id != s.ID {
			if host, ok := s.serverList[id]; ok {
				targets = append(targets, host)
			}
		}
	}
	s.muLiveServers.RUnlock()

	for _, host := range targets {
		go func(h string) {
			s.sendToHost(h, endpoint, payload)
		}(host)
	}
}

// função auxilicar para o Ping
func (s *Server) lidarPing(conn *net.UDPConn) {
	buffer := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}
		if string(buffer[:n]) == "ping" {
			conn.WriteToUDP([]byte("pong"), addr)
		}
	}
}
