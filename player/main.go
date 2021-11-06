package player

import (
	"bufio"
	"context"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"google.golang.org/grpc"
)

const (
	address     = "localhost:50051" // Cambiar despues
	defaultName = "leader"          // También
)

type server struct {
	pb.UnimplementedGameInteractionServer
}

type GameData struct {
	playerType string
	playerId   uint32
	client     pb.GameInteractionClient
	state      pb.PlayerStateState
}

var (
	gamedata = GameData{}
)

// Send the player's chosen move to Leader
func SendPlayerMove(ctx context.Context, client pb.GameInteractionClient, move *uint32) (resp pb.PlayerStateState, err error) {
	response, err := client.PlayerAction(ctx,
		&pb.PlayerMove{
			PlayerId: &gamedata.playerId,
			Move:     move,
		})

	if err != nil {
		log.Fatalf("[Player] Could not send message to server: %v", err)
	}

	resp = (response.GetPlayerState())

	log.Printf("[Player] Message response: %v", resp)
	return
}

// Recieve round start info
func (s *server) RoundStart(ctx context.Context, in *pb.RoundState) (ret *pb.PlayerAck, err error) {
	stage := in.GetStage()
	round := in.GetRound()

	// Choose a number after responding with ACK (empty)
	defer ProcessPlayerMove(ctx, stage, round)

	return &pb.PlayerAck{}, nil
}

// Ask user for input or randomly choose a number (if Player is a bot), then send it to
// the Leader, and process the Leader's response with the Player state
func ProcessPlayerMove(ctx context.Context, stage uint32, round uint32) {

	var move uint32
	var err error

	switch gamedata.playerType {
	case "bot":
		move, err = AutoMove(stage)
		if err != nil {
			log.Fatalf("[Error] While making automove: %v", err)
		}
	case "human":
		// get user input and parse to int
		move, err = GetUserInput(stage)
		if err != nil {
			log.Fatalf("[Error] While reading user input: %v", err)
		}
	}

	// Send selected number to server (Leader)
	roundResult, err := SendPlayerMove(ctx, gamedata.client, &move)
	gamedata.state = roundResult

	// If the player died, then kill the current process
	if roundResult == pb.PlayerState_DEAD {
		log.Fatalf("Jugador \"%d\" ha muerto, terminando el proceso.", gamedata.playerId)
	}
}

// user movement function
func GetUserInput(stage uint32) (number uint32, err error) {

	// If it's the second stage, then the range is limited to
	// a range of [1, 4]
	if stage == 1 {
		log.Printf("> Ingrese número del 1 al 4 (inclusive): ")

	} else {
		// Otherwise, the range is from [1, 10]
		log.Printf("> Ingrese número del 1 al 10 (inclusive): ")
	}

	// Get user input
	reader := bufio.NewReader(os.Stdin)
	userInput, err := reader.ReadString('\n')
	if err != nil {
		log.Println("[Error] While reading your input!")
		return
	}

	// Convert string into an int
	i_number, err := strconv.Atoi(userInput)
	if err != nil {
		log.Println("[Error] Can only parse integers!")
		return
	}

	number = uint32(i_number)

	return
}

// bot movement generator
func AutoMove(stage uint32) (number uint32, err error) {

	// If it's the second stage, then the range is limited to
	// a range of [1, 4]
	if stage == 1 {
		number = uint32(rand.Intn(4) + 1)

	} else {
		// Otherwise, the range is from [1, 10]
		number = uint32(rand.Intn(10) + 1)
	}

	return
}

func SetupPlayerServer(playerId uint32) {
	// Listening server

	lis, err := net.Listen("tcp", address) // cambiar address
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	player_srv := grpc.NewServer()
	pb.RegisterGameInteractionServer(player_srv, &server{})
	log.Printf("[player %d] Listening at %v", playerId, lis.Addr())

	if err := player_srv.Serve(lis); err != nil {
		log.Fatalf("[player %d] Could not bind to %v : %v", playerId, address, err)
	}
}

func Player_go(playerType string) {
	gamedata.playerType = playerType

	log.Println("Started New Game")
	log.Println("Timeout for moves between every round is 10 [s]")

	conn, err := grpc.Dial(address, grpc.WithInsecure())

	if err != nil {
		log.Fatalf("[Error] Couldn't connect to target: %v", err)
	} else {
		log.Println("Connection to leader was successful")
	}
	defer conn.Close()

	client := pb.NewGameInteractionClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	gamedata.client = client
	defer cancel()

	// request to join game
	p_id := uint32(rand.Intn(2<<32 - 1))
	gamedata.playerId = p_id
	_, err = client.PlayerJoin(ctx,
		&pb.JoinGameRequest{
			PlayerId: &p_id,
		})

	if err != nil {
		log.Fatalf("[Error] Couldn't connect to leader\n")
	}

	go SetupPlayerServer(p_id)
}
