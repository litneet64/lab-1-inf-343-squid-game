package namenode

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"google.golang.org/grpc"
)

type RoundInfo struct {
	playerId   uint32
	playerMove uint32
}

type Client struct {
	id     uint32
	addr   string
	client pb.DataRegistryServiceClient
}

var (
	datanodeAddr = [3]string{
		"localhost:50051",
		"localhost:50052",
		"localhost:50053",
	}
)

func checkIfErr(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

// Save the given round info into a txt file of the given datanode
func RegisterRoundMoves(client pb.DataRegistryServiceClient, stage uint32, round uint32, roundInfo []RoundInfo) {
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

// Recieves all the moves that a player has made.
func RetrievePlayerData(clients []Client, player uint32) {
	var requestQueue []*Client
	log.Printf("Called RetrievePlayerData")

	// Map each address to the corresponding client object
	addrToClient := make(map[string]*Client, 3)
	for i := 0; i < 3; i++ {
		addrToClient[clients[i].addr] = &clients[i]
	}

	// For each stage, get if there is an address associated to
	// moves of the player
	for i := 0; i < 3; i++ {
		addr, err := GetMoveLocations(player, uint32(i))
		log.Printf("Address found: \"%v\"", addr)

		if err == nil {
			requestQueue = append(requestQueue, addrToClient[addr])
			log.Printf("Found client")

		} else {
			break
		}
	}

	// Start timed context
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	for i := 0; i < len(requestQueue); i++ {
		stage := uint32(i)

		// Request to datanode and parse output
		data, _ := requestQueue[i].client.RequestPlayerData(ctx,
			&pb.DataRequestParams{
				PlayerId: &player,
				Stage:    &stage,
			})

		// 'data' should be sent to leader
		log.Printf("Retrieved data: %v", data.GetPlayerMoves())
	}
}

// Saves node locations of player moves for each stage
func SaveMoveLocations(player uint32, stage uint32, address string) {

	f, err := os.OpenFile("tablemap.txt", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	checkIfErr(err)
	defer f.Close()

	locationtemplate := "Jugador_%d Ronda_%d %v\n"
	f.WriteString(fmt.Sprintf(locationtemplate, player, stage, address))
	f.Sync()
}

// Returns datanode address where player moves for a stage are located,
// return empty string if not found
func GetMoveLocations(player uint32, stage uint32) (string, error) {
	// Checks if save file exists
	_, fErr := os.Stat("tablemap.txt")
	if fErr != nil {
		return "", fErr
	}
	// Open savefile
	f, err := os.Open("tablemap.txt")
	checkIfErr(err)
	defer f.Close()
	// reads each line and checks if it has requested player and stage
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		saveData := strings.Split(scanner.Text(), " ")

		samePlayer := saveData[0] == fmt.Sprintf("Jugador_%d", player)
		sameStage := saveData[1] == fmt.Sprintf("Ronda_%d", stage)

		if samePlayer && sameStage {
			return saveData[2], nil
		}
	}
	return "", os.ErrNotExist
}

func Namenode_go() {
	// Define arrays of both connections and errors for each of the
	// three datanodes that are connected to the namenode

	var conns [3]*grpc.ClientConn
	var errs [3]error
	var clients [3]Client

	// Dial each datanode
	for i := 0; i < 3; i++ {
		conns[i], errs[i] = grpc.Dial(datanodeAddr[i], grpc.WithInsecure())
		if errs[i] != nil {
			log.Fatalf("[Namenode] Error connecting to datanode #%d: \"%v\"", i, errs[i])
		}

		clients[i] = Client{
			uint32(i),
			datanodeAddr[i],
			pb.NewDataRegistryServiceClient(conns[i]),
		}
		defer conns[i].Close()
	}

	RetrievePlayerData(clients[:], 0)
}
