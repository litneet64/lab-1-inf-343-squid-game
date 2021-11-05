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

const namenodeAddr string = "localhost:50051"

func checkIfErr(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

// Writes player move to a file
func writeToFile(player uint32, stage uint32, move uint32) {
	var f *os.File
	var err error

	filename := fmt.Sprintf("jugador_%d__etapa_%d.txt", player, stage)

	// Create if it doesn't exist, otherwise just open it
	if _, err := os.Stat(filename); err == nil {
		f, err = os.Create(filename)
	} else {
		f, err = os.Open(filename)
	}
	checkIfErr(err)
	defer f.Close()

	// Writes to file
	f.WriteString(fmt.Sprintf("%d\n", move))
	f.Sync()
}

// Parses registered player moves from it's file and returns them
func getPlayerStageRounds(player uint32, stage uint32) []uint32 {
	// List of all player moves for a given stage
	var moves []uint32

	// Read player file for the given stage
	f, fErr := os.Open(fmt.Sprintf("jugador_%d__etapa_%d.txt", player, stage))
	checkIfErr(fErr)

	// Start saving each move
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		move, err := strconv.Atoi(scanner.Text())
		checkIfErr(err)
		moves = append(moves, uint32(move))
	}
	f.Sync()

	return moves
}

// The server-side implementation of the rpc function that the namenode calls
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

func Datanode_go() {
	// Set the listening port for the server
	lis, err := net.Listen("tcp", namenodeAddr)
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
