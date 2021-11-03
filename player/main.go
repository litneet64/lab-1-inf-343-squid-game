package player

import (
	"context"
	"log"
	"time"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"

	"google.golang.org/grpc"
)

const (
	address     = "localhost:50051" // Cambiar despues
	defaultName = "leader"          // También
)

func Player_bot_go() {
	log.Println("[+] Hello from bot player!")

}

func Player_human_go() {
	log.Println("[+] Hello from Player")

	conn, err := grpc.Dial(address, grpc.WithInsecure())

	if err != nil {
		log.Fatalf("[Player] Could not connect to target: %v", err)
	} else {
		log.Println("[Player] Connection successful")
	}
	defer conn.Close()

	client := pb.NewGameInteractionClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	response, err := client.PlayerSend(ctx, &pb.PlayerToLeaderRequest{Msg: pb.PlayerToLeaderRequest_JOIN_GAME.Enum()})
	if err != nil {
		log.Fatalf("[Player] Could not send message to server: %v", err)
	}

	log.Printf("[Player] Message response: %v", response.GetMsg())
}
