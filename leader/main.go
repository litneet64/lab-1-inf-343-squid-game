package leader

import (
	"context"
	"log"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
)

const (
	port = ":50051"
)

type PlayerToLeaderRequestMsgType int32

type server struct {
	pb.UnimplementedGameInteractionServer
}

func (s *server) PlayerSend(ctx context.Context, in *pb.PlayerToLeaderRequest) (*pb.PlayerToLeaderReply, error) {
	log.Printf("Received: %v", in.GetMsg())
	return &pb.PlayerToLeaderReply{Msg: pb.PlayerToLeaderReply_ACCEPT_JOIN.Enum()}, nil
}

func Leader_go() {

	log.Println("[+] Success!")
}
