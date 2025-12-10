package main

import (
	"encoding/json"
	"time"

	"PlanoZ/internal/models"
)

func (s *Server) listenRedisGlobal(topico string) {
	// color.Cyan("Ouvindo tópico Redis: %s", topico)
	pubsub := s.redisClient.Subscribe(s.ctx, topico)
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(s.ctx)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// legado pra compatibilidade, se precisar processar msg velha
		if topico == TopicoConectar {
			var req models.ReqConectar
			json.Unmarshal([]byte(msg.Payload), &req)
			// logica extra viria aqui
		}
	}
}
