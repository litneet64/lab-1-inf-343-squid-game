package datanode

import (
	"fmt"
	"log"
	"net"
	"os"
	"bufio"

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

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func createFile(player uint32, game uint32, moves []RoundInfo) {
	f, err := os.Create(fmt.Sprintf("jugador_%d__etapa_%d.txt", player, game))
	check(err)
	defer f.Close()
	for i := 0; i < len(moves); i++ {
		if moves[i].playerId == player {
			f.WriteString(fmt.Sprintf("%d\n", moves[i].playerMove))
		}
	}
	f.Sync()
}
func appendRound(player uint32, game uint32, moves []RoundInfo) {
	f, err := os.Open(fmt.Sprintf("jugador_%d__etapa_%d.txt", player, game))
	check(err)
	defer f.Close()
	for i := 0; i < len(moves); i++ {
		if moves[i].playerId == player {
			f.WriteString(fmt.Sprintf("%d\n", moves[i].playerMove))
		}
	}
	f.Sync()
}

func getGame(player uint32, game uint32) []RoundInfo {
    var game []RoundInfo
    f, err := os.Open(fmt.Sprintf("jugador_%d__etapa_%d.txt", player, game))
    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        game.append(RoundInfo{playerId: player, playerMove: scanner.Int})
    }

    if err := scanner.Err(); err != nil {
        log.Fatal(err)
    }
}

    return game

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
