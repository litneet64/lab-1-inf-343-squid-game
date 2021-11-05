package namenode

import (
	"context"
	"log"
	"time"
	"strings"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"google.golang.org/grpc"
)

type RoundInfo struct {
	playerId   uint32
	playerMove uint32
}

var (
	datanodeAddr = [3]string{
		"localhost:50051",
		"localhost:50052",
		"localhost:50053",
	}
)

// Save the given round info into a txt file of the given datanode
func Register(client pb.DataRegistryServiceClient, stage uint32, round uint32, roundInfo []RoundInfo) {
	// Start timed context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Store each move, using `roundInfo`
	all_moves := make([]*pb.PlayersMoves_Move, len(roundInfo))

	for i := 0; i < len(roundInfo); i++ {
		all_moves[i] = &pb.PlayersMoves_Move{
			PlayerId:   &roundInfo[i].playerId,
			PlayerMove: &roundInfo[i].playerMove,
		}
	}

	// Send message to datanode
	client.TransferPlayerMoves(ctx,
		&pb.PlayersMoves{
			Stage:        &stage,
			Round:        &round,
			PlayersMoves: all_moves,
		})
}

// Receive round info from datanode
func Retrieve() {

}

func Namenode_go() {
	// Define two arrays of both connections and errors for each of the
	// three datanodes that are connected to the namenode

	var conns [3]*grpc.ClientConn
	var errs [3]error
	var clients [3]pb.DataRegistryServiceClient

	// Dial each datanode
	for i := 0; i < 3; i++ {
		conns[i], errs[i] = grpc.Dial(datanodeAddr[i], grpc.WithInsecure())
		if errs[i] != nil {
			log.Fatalf("[Namenode] Error connecting to datanode #%d: \"%v\"", i, errs[i])
		}
		clients[i] = pb.NewDataRegistryServiceClient(conns[i])
		defer conns[i].Close()
	}

	rounds := []RoundInfo{
		{0, 1},
		{1, 2},
		{2, 3},
	}
	// Test client 0, stage 0, round 0, etc.
	Register(clients[0], 0, 0, rounds)
	Register(clients[0], 0, 1, rounds)

}
