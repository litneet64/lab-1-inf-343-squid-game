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

// DEBUG TESTING --
type DebugLogger struct {
	fileName    string
	initialized bool
}

func InitLogger(fileName string) {
	dlogger.fileName = fileName
	dlogger.initialized = true
}

func DebugLog(msg ...string) {
	if !dlogger.initialized {
		log.Fatalf("[DebugLog] logger was not initialized")
	}

	f, err := os.OpenFile(dlogger.fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	FailOnError(err, fmt.Sprintf("[InitLogger] Could not open file \"%s\": %v", dlogger.fileName, err))
	defer f.Close()

	logger := log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	logger.Println(strings.Join(msg, " "))
}

func DebugLogf(msg string, a ...interface{}) {
	if !dlogger.initialized {
		log.Fatalf("[DebugLogf] logger was not initialized")
	}

	f, err := os.OpenFile(dlogger.fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	FailOnError(err, fmt.Sprintf("[InitLogger] Could not open file \"%s\": %v", dlogger.fileName, err))
	defer f.Close()

	logger := log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	logger.Println(fmt.Sprintf(msg, a...))
}

func FailOnError(err error, msg string) {
	if err != nil {
		DebugLogf("[Fatal] %s: %v", msg, err)
		log.Fatalf("[Fatal] %s: %v", msg, err)
	}
}

var dlogger DebugLogger

// DEBUG TESTING --

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
	DebugLogf("\t[SendPlayerMove] Running function: SendPlayerMove(ctx, client, move: %d)", *move)
	response, err := client.PlayerAction(ctx,
		&pb.PlayerMove{
			PlayerId: &gamedata.playerId,
			Move:     move,
		})
	FailOnError(err, fmt.Sprintf("[Player] Could not send message to server: %v", err))

	resp = (response.GetPlayerState())
	DebugLogf("\t[SendPlayerMove] Got response \"%s\"", resp.String())

	return
}

// Send the player's chosen move to Leader
func SendPlayerCommand(ctx context.Context, client pb.GameInteractionClient, command *pb.PlayerCommandCommandType) {
	DebugLogf("\t[SendPlayerCommand] Running function: SendPlayerCommand(ctx, client, command: %s)", command.String())
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
		log.Printf("> La cantidad de dinero en el pozo es de: %v", response.GetReply())

	}
}

// Recieve round start info
func (s *server) RoundStart(ctx context.Context, in *pb.RoundState) (ret *pb.PlayerAck, err error) {
	DebugLogf("\t[server:RoundStart] Running function: RoundStart(ctx, in: %s)", in.String())
	stage := in.GetStage()
	round := in.GetRound()

	if in.GetPlayerState() == pb.RoundState_DEAD {
		// If the player died, then kill the current process
		log.Fatalf("> Jugador \"%d\" ha muerto, terminando el proceso.", gamedata.playerId)
	}

	// Choose a number after responding with ACK (empty)
	defer ProcessPlayerMove(ctx, stage, round)

	return &pb.PlayerAck{}, nil

}

// Ask user for input or randomly choose a number (if Player is a bot), then send it to
// the Leader, and process the Leader's response with the Player state
func ProcessPlayerMove(ctx context.Context, stage uint32, round uint32) {
	DebugLogf("\t[ProcessPlayerMove] Running function: ProcessPlayerMove(ctx, stage: %d, round: %d)", stage, round)

	var move PlayerMove
	var err error

	switch gamedata.playerType {
	case "bot":
		DebugLog("\t[ProcessPlayerMove] Player is a bot, running AutoMove")
		move, err = AutoMove(stage)
		if err != nil {
			log.Fatalf("[Error] While making automove: %v", err)
		}
	case "human":
		DebugLog("\t[ProcessPlayerMove] Player is a human, getting input")
		// get user input and parse to int
		move, err = GetUserInput(stage)
		if err != nil {
			log.Fatalf("[Error] While reading user input: %v", err)
		}
		DebugLogf("\t[ProcessPlayerMove] Input was {number: %d, command: %s}", move.optNumber, move.optCommand.String())
	}

	// Send selected number to server (Leader)
	if move.isNumber {
		DebugLog("\t[ProcessPlayerMove] Sending move to Leader")
		roundResult, _ := SendPlayerMove(ctx, gamedata.client, &move.optNumber)
		gamedata.state = roundResult

		if roundResult == pb.PlayerState_DEAD {
			DebugLog("\t[ProcessPlayerMove] Leader chose player randomly and killed this process")
			// If the player died, then kill the current process
			log.Fatalf("> Jugador \"%d\" ha muerto, terminando el proceso.", gamedata.playerId)
		}
	} else {
		DebugLog("\t[ProcessPlayerMove] Requesting to read pool price to Leader")
		SendPlayerCommand(ctx, gamedata.client, pb.PlayerCommand_POOL.Enum())

	}
}

// user movement function
func GetUserInput(stage uint32) (move PlayerMove, err error) {
	DebugLogf("\t[GetUserInput] Running function: GetUserInput(stage: %d)", stage)

	// If it's the second stage, then the range is limited to
	// a range of [1, 4]
	if stage == 1 {
		log.Printf("> Ingrese número del 1 al 4 (inclusive) ")

	} else {
		// Otherwise, the range is from [1, 10]
		log.Printf("> Ingrese número del 1 al 10 (inclusive) ")
	}
	log.Printf("> o ingrese \"pozo\" para ver la cantidad de dinero actual: ")

	// Get user input
	reader := bufio.NewReader(os.Stdin)
	userInput, err := reader.ReadString('\n')
	FailOnError(err, "[Error] Reading input")
	parsedInput := strings.Trim(userInput, "\n")

	if parsedInput == "pozo" {
		DebugLog("\t[GetUserInput] User input was \"pozo\"")
		return PlayerMove{optCommand: pb.PlayerCommand_POOL, isNumber: false}, nil

	} else {
		// Convert string into an int
		i_number, err := strconv.Atoi(parsedInput)
		if err != nil {
			log.Println("> No se pudo interpretar bien el input.")
			return GetUserInput(stage)
		}

		DebugLogf("\t[GetUserInput] User input was \"%d\"", i_number)
		return PlayerMove{optNumber: uint32(i_number), isNumber: true}, nil
	}
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
	FailOnError(err, fmt.Sprintf("failed to listen: %v", err))

	player_srv := grpc.NewServer()
	pb.RegisterGameInteractionServer(player_srv, &server{})
	DebugLogf("[player %d] Listening at %v", playerId, lis.Addr())

	err = player_srv.Serve(lis)
	FailOnError(err, fmt.Sprintf("[player %d] Could not bind to %v : %v", playerId, bindAddr, err))
}

func Player_go(playerType string, playerId uint32) {
	// DEBUG LOGGER
	InitLogger(fmt.Sprintf("player_%d.log", playerId))

	gamedata.playerType = playerType

	leaderAddr = os.Getenv(leaderAddrEnv)
	tmpAddr := strings.Join([]string{os.Getenv(bindAddrEnv), "%02d"}, "")
	bindAddr = fmt.Sprintf(tmpAddr, playerId)

	DebugLogf("Setup addresses: leader=%s, bind=%s", leaderAddr, bindAddr)

	conn, err := grpc.Dial(leaderAddr, grpc.WithInsecure())
	FailOnError(err, fmt.Sprintf("[Error] Couldn't connect to target: %v", err))
	defer conn.Close()

	client := pb.NewGameInteractionClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	gamedata.client = client
	defer cancel()

	// Start listening on port
	go SetupPlayerServer(playerId)

	DebugLog("Setup player and server clients")

	// -- Request to join game

	// Start game
	if playerType == "human" {

		// Get user input
		reader := bufio.NewReader(os.Stdin)
		for {
			log.Printf("> Para unirte al juego, escribe \"ingresar\": ")
			userInput, err := reader.ReadString('\n')
			FailOnError(err, "[Error] While reading your input!")

			parsedInput := strings.Trim(userInput, "\n")

			DebugLogf("Read user input: \"%s\"", parsedInput)

			if parsedInput == "ingresar" {
				DebugLog("Sending 'PlayerJoin' request to Leader")
				gamedata.playerId = playerId
				_, err = client.PlayerJoin(ctx,
					&pb.JoinGameRequest{
						PlayerId: &playerId,
					})
				FailOnError(err, "[Error] Couldn't connect to leader")

				break
			} else {
				DebugLog("Comand not recognized")
				log.Printf("> No se pudo reconocer el comando, por favor inténtelo de nuevo.")
			}
		}
	} else {
		_, err = client.PlayerJoin(ctx,
			&pb.JoinGameRequest{
				PlayerId: &playerId,
			})
		FailOnError(err, "[Error] Couldn't connect to leader")
	}

	// waits forever
	forever_ch := make(chan bool)
	<-forever_ch
}
