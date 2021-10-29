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
    log.Println("[+] Hello from player!")
    conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("No funca :( : %v", err)
	} else {
		log.Println("[+] Success!")
	}
	defer conn.Close()
	client := pb.NewGameInteractionClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	response, err := client.PlayerSend(ctx, &pb.PlayerToLeaderRequest{Msg: pb.PlayerToLeaderRequest_JOIN_GAME.Enum()})
	if err != nil {
		log.Fatalf("Error de mensaje(?): %v", err)
	}
	log.Printf("Mensaje: %v", response.GetMsg())
}
