package datanode

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"google.golang.org/grpc"
)

type RoundInfo struct {
	playerId   uint32
	playerMove uint32
}

type server struct {
	pb.UnimplementedDataRegistryServiceServer
}

const (
	bindAddrEnv = "DATANODE_BIND_ADDR"
)

var (
	bindAddr string
)

func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("[Error]: (%v) %s", err, msg)
	}
}

// Writes player move to a file
func writeToFile(player uint32, stage uint32, move uint32) {
	var f *os.File
	var err error

	filename := fmt.Sprintf("jugador_%d__etapa_%d.txt", player, stage)

	// Create if it doesn't exist, otherwise just open it
	f, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	FailOnError(err, fmt.Sprintf("can't open file \"%s\"", filename))
	defer f.Close()

	// Writes to file
	f.WriteString(fmt.Sprintf("%d\n", move))
	f.Sync()
}

// Parses registered player moves from it's file and returns them
func GetPlayerStageRounds(player uint32, stage uint32) []uint32 {
	// List of all player moves for a given stage
	var moves []uint32

	// Read player file for the given stage
	filename := fmt.Sprintf("jugador_%d__etapa_%d.txt", player, stage)
	f, err := os.Open(filename)
	FailOnError(err, fmt.Sprintf("can't open file \"%s\"", filename))

	// Start saving each move
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		move, err := strconv.Atoi(scanner.Text())
		FailOnError(err, fmt.Sprintf("can't convert string \"%s\" to int", scanner.Text()))
		moves = append(moves, uint32(move))
	}

	return moves
}

// The server-side implementation of the rpc function that the namenode
// calls. Transfers all moves that the group of players do in a specific
// round and stage.
func (s *server) TransferPlayerMoves(ctx context.Context, in *pb.PlayersMoves) (*pb.Empty, error) {
	moves := in.GetPlayersMoves()
	stage := in.GetStage()

	// For each move, append it to the corresponding file
	for i := 0; i < len(moves); i++ {
		writeToFile(moves[i].GetPlayerId(), stage, moves[i].GetPlayerMove())
	}

	// No reply is expected, so return empty message
	return &pb.Empty{}, nil
}

// The server-side implementation of the rpc function that the namenode
// calls. Given a player id and stage, send all moves
func (s *server) RequestPlayerData(ctx context.Context, in *pb.DataRequestParams) (*pb.StageData, error) {
	player := in.GetPlayerId()
	stage := in.GetStage()

	// Get moves by reading the player's files
	moves := GetPlayerStageRounds(player, stage)

	// Send moves to the namenode
	return &pb.StageData{PlayerMoves: moves}, nil
}

func Datanode_go() {
	bindAddr = os.Getenv(bindAddrEnv)

	// Set the listening port for the server
	lis, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatalf("[Datanode] Could not listen: %v", err)
	}

	// Define and register new server for this datanode
	datanodeServer := grpc.NewServer()
	pb.RegisterDataRegistryServiceServer(datanodeServer, &server{})

	// Start listening
	if err := datanodeServer.Serve(lis); err != nil {
		log.Fatalf("[Datanode] Could not serve: %v", err)
	}

}
