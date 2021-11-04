package leader

import (
	"context"
	"log"
	"net"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"google.golang.org/grpc"
)

const (
	port = ":50051"
)

type server struct {
	pb.UnimplementedGameInteractionServer
}

func (s *server) PlayerSend(ctx context.Context, in *pb.PlayerToLeaderRequest) (*pb.PlayerToLeaderReply, error) {
	log.Printf("Received: %v", in.GetMsg())
	return &pb.PlayerToLeaderReply{Msg: pb.PlayerToLeaderReply_ACCEPT_JOIN.Enum()}, nil
}

func Leader_go() {
	log.Println("[+] Hello from Leader")

	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("[Leader] Could not listen: %v", err)
	}

	leader_srv := grpc.NewServer()
	pb.RegisterGameInteractionServer(leader_srv, &server{})
	log.Printf("[Leader] Listening at %v", lis.Addr())

	if err := leader_srv.Serve(lis); err != nil {
		log.Fatalf("[Leader] Could not serve: %v", err)
	}
}
