package player

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"google.golang.org/grpc"
)

const (
	leaderAddrEnv = "LEADER_ADDR"
	bindAddrEnv   = "PLAYER_BIND_ADDR"
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

type PlayerMove struct {
	optNumber  uint32
	optCommand pb.PlayerCommandCommandType
	isNumber   bool
}

var (
	bindAddr, leaderAddr string
	gamedata             = GameData{}
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

// Send the player's chosen move to Leader
func SendPlayerCommand(ctx context.Context, client pb.GameInteractionClient, command *pb.PlayerCommandCommandType) {
	response, err := client.RequestCommand(ctx,
		&pb.PlayerCommand{
			PlayerId: &gamedata.playerId,
			Command:  command,
		})
	if err != nil {
		log.Fatalf("[Player] Could not send message to server: %v", err)
	}

	switch command {
	case pb.PlayerCommand_POOL.Enum():
		log.Printf("La cantidad de dinero en el pozo es de: %v", response.GetReply())

	}
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

	var move PlayerMove
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
	if move.isNumber {
		roundResult, _ := SendPlayerMove(ctx, gamedata.client, &move.optNumber)
		gamedata.state = roundResult

		// If the player died, then kill the current process
		if roundResult == pb.PlayerState_DEAD {
			log.Fatalf("> Jugador \"%d\" ha muerto, terminando el proceso.", gamedata.playerId)
		}
	} else {
		SendPlayerCommand(ctx, gamedata.client, pb.PlayerCommand_POOL.Enum())

	}
}

// user movement function
func GetUserInput(stage uint32) (move PlayerMove, err error) {

	// If it's the second stage, then the range is limited to
	// a range of [1, 4]
	if stage == 1 {
		log.Printf("> Ingrese número del 1 al 4 (inclusive) ")

	} else {
		// Otherwise, the range is from [1, 10]
		log.Printf("> Ingrese número del 1 al 10 (inclusive) ")
	}
	log.Printf("o ingrese \"pozo\" para ver la cantidad de dinero actual: ")

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
		if userInput == "pozo" {
			return PlayerMove{optCommand: pb.PlayerCommand_POOL}, nil

		} else {
			log.Println("> No se pudo interpretar bien el input.")
			return GetUserInput(stage)
		}
	}

	return PlayerMove{optNumber: uint32(i_number), isNumber: true}, nil
}

// bot movement generator
func AutoMove(stage uint32) (move PlayerMove, err error) {
	var number uint32

	// If it's the second stage, then the range is limited to
	// a range of [1, 4]
	if stage == 1 {
		number = uint32(rand.Intn(4) + 1)

	} else {
		// Otherwise, the range is from [1, 10]
		number = uint32(rand.Intn(10) + 1)
	}

	return PlayerMove{optNumber: number, isNumber: true}, nil
}

func SetupPlayerServer(playerId uint32) {
	// Listening server

	lis, err := net.Listen("tcp", bindAddr) // cambiar address
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	player_srv := grpc.NewServer()
	pb.RegisterGameInteractionServer(player_srv, &server{})
	log.Printf("[player %d] Listening at %v", playerId, lis.Addr())

	if err := player_srv.Serve(lis); err != nil {
		log.Fatalf("[player %d] Could not bind to %v : %v", playerId, bindAddr, err)
	}
}

func Player_go(playerType string, playerId uint32) {
	gamedata.playerType = playerType

	leaderAddr = os.Getenv(leaderAddrEnv)
	tmpAddr := strings.Join([]string{os.Getenv(bindAddrEnv), "%02d"}, "")
	bindAddr = fmt.Sprintf(tmpAddr, playerId)

	log.Println("Started New Game")
	log.Println("Timeout for moves between every round is 10 [s]")

	conn, err := grpc.Dial(leaderAddr, grpc.WithInsecure())

	if err != nil {
		log.Fatalf("[Error] Couldn't connect to target: %v", err)
	}
	defer conn.Close()

	client := pb.NewGameInteractionClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	gamedata.client = client
	defer cancel()

	// request to join game
	gamedata.playerId = playerId
	_, err = client.PlayerJoin(ctx,
		&pb.JoinGameRequest{
			PlayerId: &playerId,
		})

	if err != nil {
		log.Fatalf("[Error] Couldn't connect to leader\n")
	}

	SetupPlayerServer(playerId)
}
