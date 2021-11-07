package leader

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"github.com/streadway/amqp"
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
	bindAddrEnv     = "LEADER_BIND_ADDR"
	playerAddrEnv   = "PLAYER_ADDR"
	poolAddrEnv     = "POOL_ADDR"
	namenodeAddrEnv = "NAMENODE_ADDR"
	rabbitMqAddrEnv = "RABBITMQ_ADDR"
	playerNum       = 16
)

// should work as struct
type PlayerState struct {
	Dead, Alive, NotPlaying uint32
}

// holds required data for a succesful grpc preamble dial
type GrpcData struct {
	ctx          *context.Context
	conn         *grpc.ClientConn
	clientData   *pb.DataRegistryServiceClient
	clientPlayer *pb.GameInteractionClient
	clientPrize  *pb.PrizeClient
	cancel       *context.CancelFunc
}

type RabbitMqData struct {
	conn  *amqp.Connection
	queue *amqp.Queue
	ch    *amqp.Channel
}

type GameData struct {
	playerIdList      [playerNum]uint32 // list of player's id
	playerIdStates    [playerNum]uint32 // list with the most current player's state (dead, alive, not playing)
	playerIdMoves     [playerNum]uint32 // list with the most current player's movement
	currPlayers       uint32            // num of players in the current game
	stage             uint32            // current stage
	round             uint32            // current round
	numRoundsPerStage []uint32          // stage's duration (defined prior to game)
	leaderNumber      uint32            // number selected by leader for current round
	groupsProcessed   bool              // groups have been processed by a thread already after a stage (used for thread sync)
}

type Player struct {
	index int    // player's index in gamedata's list
	id    uint32 // player id
}

type UserInput struct {
	optPlayerId uint32 // optional player id if leader inputs is a number
	optCommand  string // optional command (e.g. "comenzar")
	isPlayerId  bool   // bool to check if the input was a player id or not
}

type PlayerGroup struct {
	playerIds        []uint32
	maxGroupSize     int
	isComplete       bool       // If the group is full already
	optMoveSum       uint32     // Used in stage 2 to store the sum of all moves
	optLeaderNum     uint32     // Used in stage 3 to store the leader's number
	optPlayerNumRest [2]uint32  //               - to store <leader's num - player num>
	mu               sync.Mutex // struct's mutex for safe thread interaction
	optWon           bool
}

var (
	// PlayerState "enum"
	playerState = PlayerState{
		Dead:       0,
		Alive:      1,
		NotPlaying: 2,
	}

	bindAddr string

	// Addresses list map
	addrListMap = map[string]string{
		"pool":     os.Getenv(poolAddrEnv),
		"namenode": os.Getenv(namenodeAddrEnv),
		"rabbitmq": os.Getenv(rabbitMqAddrEnv),
	}

	// global player data
	gamedata = GameData{
		stage:             0,
		round:             0,
		numRoundsPerStage: []uint32{4, 1, 1},
		leaderNumber:      0,
	}

	rabbitMqData = RabbitMqData{}

	// Map with struct that saves basic grpc dial data
	grpcmap = map[string]GrpcData{
		"namenode": {},
		"pool":     {},
	}

	stage2PlayerGroups [2]PlayerGroup
	stage3PlayerGroups []PlayerGroup
)

type server struct {
	pb.UnimplementedGameInteractionServer
}

// handle player's movement
func (s *server) PlayerAction(ctx context.Context, in *pb.PlayerMove) (*pb.PlayerState, error) {
	DebugLogf("\t[server:PlayerAction] Running function: PlayerAction(ctx, in: %s)", in.String())

	playerMove := in.GetMove()
	playerId := in.GetPlayerId()
	stage := in.GetStage()

	switch gamedata.stage {
	case 0:
		if playerMove <= gamedata.leaderNumber {
			return &pb.PlayerState{PlayerState: pb.PlayerState_ALIVE.Enum()}, nil
		}
	case 1:
		var playerGroupIdx int
		var reply pb.PlayerStateState
		gamedata.groupsProcessed = false

		for i := 0; i < 2; i++ {
			stage2PlayerGroups[i].mu.Lock()
			if stage2PlayerGroups[i].isComplete {
				continue
			}

			stage2PlayerGroups[i].playerIds = append(stage2PlayerGroups[i].playerIds, in.GetPlayerId())
			stage2PlayerGroups[i].optMoveSum += playerMove
			playerGroupIdx = i

			if len(stage2PlayerGroups[i].playerIds) == stage2PlayerGroups[i].maxGroupSize {
				stage2PlayerGroups[i].isComplete = true
			}
			stage2PlayerGroups[i].mu.Unlock()

		}

		// wait until every player has joined a group
		for !stage2PlayerGroups[0].isComplete && !stage2PlayerGroups[1].isComplete {
			time.Sleep(100 * time.Millisecond)
		}

		// lock thread access to process winner group(s)
		stage2PlayerGroups[playerGroupIdx].mu.Lock()
		if !gamedata.groupsProcessed {
			for i := 0; i < 2; i++ {
				if stage2PlayerGroups[0].optMoveSum%2 == gamedata.leaderNumber%2 {
					stage2PlayerGroups[0].optWon = true
				} else {
					stage2PlayerGroups[0].optWon = false
				}
			}

			if stage2PlayerGroups[0].optWon == false && stage2PlayerGroups[1].optWon == false {
				winnerGroup := rand.Intn(2)
				stage2PlayerGroups[winnerGroup].optWon = true
			}
			gamedata.groupsProcessed = true
		}
		stage2PlayerGroups[playerGroupIdx].mu.Unlock()

		// all threads return the corresponding "alive or dead" reply according to game process
		if stage2PlayerGroups[playerGroupIdx].optWon {
			reply = *pb.PlayerState_ALIVE.Enum()
		} else {
			reply = *pb.PlayerState_DEAD.Enum()
		}

		return &pb.PlayerState{PlayerState: &reply}, nil

	case 2:
		for i := 0; i < len(stage3PlayerGroups); i++ {
			// if complete skip

			stage3PlayerGroups[i].mu.Lock()
			if stage3PlayerGroups[i].isComplete {
				continue
			}
			// add player to its own group
			stage3PlayerGroups[i].playerIds = append(stage3PlayerGroups[i].playerIds, in.GetPlayerId())
			playerList := stage3PlayerGroups[i].playerIds

			// if group now is complete, set complete flag to true
			if len(playerList) == stage3PlayerGroups[i].maxGroupSize {
				stage3PlayerGroups[i].isComplete = true
			}
			stage3PlayerGroups[i].mu.Unlock()

			// wait until current group is full
			for !stage3PlayerGroups[i].isComplete {
				time.Sleep(100 * time.Millisecond)
			}

			// do comparation
			leaderMove := gamedata.leaderNumber
			var player1res uint32 = uint32(math.Abs(float64(leaderMove - playerList[0])))
			var player2res uint32 = uint32(math.Abs(float64(leaderMove - playerList[1])))

			if player1res < player2res {
				// player 1 wins
			} else if player1res > player2res {
				// player 2 wins
				// send roundstart player 1 dies

				return &pb.PlayerState{PlayerState: pb.PlayerState_ALIVE.Enum()}, nil
			} else if playerList[0] == playerList[1] {
				// both wins
				return &pb.PlayerState{PlayerState: pb.PlayerState_ALIVE.Enum()}, nil
			}

		}
		return &pb.PlayerState{PlayerState: pb.PlayerState_WAITING.Enum()}, nil
	default:
		log.Fatalf("Unreachable stage reached: %d", gamedata.stage)
	}

	PublishDeadPlayer(&playerId, &stage)
	return &pb.PlayerState{PlayerState: pb.PlayerState_DEAD.Enum()}, nil

}

// Send player death to Pool via RabbitMQ
func PublishDeadPlayer(playerId *uint32, stage *uint32) {
	DebugLogf("\t[PublishDeadPlayer] Running function: PublishDeadPlayer(playerId: %d, stage: %d)", *playerId, *stage)
	// If player died, then tell Pool to save that info
	//
	// Because RabbitMQ asks for byte arrays, the player ids (uint32)
	// should be converted

	// Transform the player id to int64 in an array of bytes
	body := fmt.Sprintf("%d %d", playerId, stage)

	err := rabbitMqData.ch.Publish(
		"",                      // exchange
		rabbitMqData.queue.Name, // routing key
		false,                   // mandatory
		false,                   // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	FailOnError(err, "Failed to publish a message")
}

// handle player's join game request
func (s *server) PlayerJoin(ctx context.Context, in *pb.JoinGameRequest) (*pb.JoinGameReply, error) {
	DebugLogf("\t[server:PlayerJoin] Running function: PlayerJoin(ctx, in: %s)", in.String())
	var reply *pb.JoinGameReply

	if gamedata.currPlayers != playerNum || gamedata.stage != 1 {
		reply = &pb.JoinGameReply{Msg: pb.JoinGameReply_DENY_JOIN.Enum()}
	} else {
		// get player index in list, update state and increase index
		p_idx := gamedata.currPlayers
		gamedata.playerIdList[p_idx] = in.GetPlayerId()
		gamedata.playerIdStates[p_idx] = playerState.Alive
		gamedata.currPlayers++
		reply = &pb.JoinGameReply{Msg: pb.JoinGameReply_ACCEPT_JOIN.Enum()}
	}

	return reply, nil
}

// handle player's command (e.g. read pool prize)
func (s *server) RequestCommand(ctx context.Context, in *pb.PlayerCommand) (*pb.CommandReply, error) {
	DebugLogf("\t[server:RequestCommand] Running function: RequestCommand(ctx, in: %s)", in.String())
	var reply *pb.CommandReply

	playerId := in.GetPlayerId()

	switch in.GetCommand() {
	case pb.PlayerCommand_POOL:
		// Re send round start to player
		playerClient := (*grpcmap[fmt.Sprintf("player_%d", playerId)].clientPlayer)
		defer playerClient.RoundStart(ctx, in.GetRoundState())

		prizeClient := (*grpcmap[fmt.Sprintf("player_%d", playerId)].clientPrize)
		prize := RequestPrize(ctx, prizeClient)
		return &pb.CommandReply{Reply: &prize}, nil
	}

	return reply, nil
}

// request current accumulated prize to pool node
func RequestPrize(ctx context.Context, client pb.PrizeClient) uint32 {
	DebugLog("\t[RequestPrize] Running function: RequestPrize(ctx, client: pb.PrizeClient)")
	response, err := client.GetPrize(ctx,
		&pb.CurrentPoolRequest{},
	)
	FailOnError(err, "[Leader] Couldn't communicate with Pool")

	resp := response.GetCurrPrize()
	log.Printf("[Leader] Pool prize response: %v", resp)

	return resp
}

// call grpc preamble
func SetupDial(addr string, grpcdata *GrpcData, entity string) error {
	DebugLogf("\t[SetupDial] Running function: SetupDial(addr: %s, grpcdata, entity: %s)", addr, entity)
	connection, err := grpc.Dial(addr, grpc.WithInsecure())
	grpcdata.conn = connection

	if err != nil {
		log.Fatalf("[Leader] Couldn't connect to target: %v", err)
	} else {
		log.Println("Connection was successful")
	}

	if entity != "namenode" && entity != "pool" {
		client := pb.NewGameInteractionClient(grpcdata.conn)
		grpcdata.clientPlayer = &client
	} else {
		client := pb.NewDataRegistryServiceClient(grpcdata.conn)
		grpcdata.clientData = &client
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	grpcdata.ctx = &ctx
	grpcdata.cancel = &cancel

	return err
}

// main server for player functionality
func LeaderToPlayerServer() {
	DebugLog("\t[LeaderToPlayerServer] Running function: LeaderToPlayerServer()")
	lis, err := net.Listen("tcp", bindAddr)
	FailOnError(err, "[Leader] failed to listen on address")

	leader_srv := grpc.NewServer()
	pb.RegisterGameInteractionServer(leader_srv, &server{})
	log.Printf("[Leader] Listening at %v", lis.Addr())

	if err := leader_srv.Serve(lis); err != nil {
		log.Fatalf("[Leader] Could not bind to %v : %v", bindAddr, err)
	}
}

// request player history from namenode
func RequestPlayerHistory(gamedata *GameData, grpcdata *GrpcData, playerId uint32) *[]uint32 {
	DebugLogf("\t[Server:RequestPlayerHistory] Running function: RequestPlayerHistory(gamedata, grpcdata, playerId: %d)", playerId)

	player_moves := make([]uint32, 0)

	// Request stage player's history to namenode
	stageData, err := (*grpcdata.clientData).RequestPlayerData(*grpcdata.ctx,
		&pb.DataRequestParams{
			PlayerId: &playerId,
		})

	FailOnError(err,
		fmt.Sprintf("[Leader] Error found while trying to retrieve player (%v) history", playerId),
	)

	data := stageData.GetPlayerMoves()
	player_moves = append(player_moves, data...)

	return &player_moves
}

// send player moves to namenode
func SendPlayerMoves(grpcdata *GrpcData, gamedata *GameData) {
	DebugLog("\t[SendPlayerMoves] Running function: SendPlayerMoves(grpcdat, gamedata)")
	// Store each move in dynamic slice
	all_moves := make([]*pb.PlayersMoves_Move, playerNum)

	for i := 0; i < playerNum; i++ {
		all_moves[i] = &pb.PlayersMoves_Move{
			PlayerId:   &gamedata.playerIdList[i],
			PlayerMove: &gamedata.playerIdMoves[i],
		}
	}

	// Send message to datanode
	_, err := (*grpcdata.clientData).TransferPlayerMoves(*grpcdata.ctx,
		&pb.PlayersMoves{
			Stage:        &gamedata.stage,
			Round:        &gamedata.round,
			PlayersMoves: all_moves,
		})

	FailOnError(err, "[Leader] Error while sending player's moves to namenode\n")
	return
}

// setup dial to mqrabbit server (with global struct var)
func RabbitMqSetup() {
	DebugLog("\t[RabbitMqSettup] Running function: RabbitMqSettup()")
	// -- Special setup to dial Pool with RabbitMQ
	conn, err := amqp.Dial(addrListMap["rabbitmq"])
	FailOnError(err, "Failed to connect to Pool using RabbitMQ")

	ch, err := conn.Channel()
	FailOnError(err, "Failed to open a channel with Pool")

	q, err := ch.QueueDeclare(
		"deadpool", // name
		false,      // durable
		false,      // delete when unused
		false,      // exclusive
		false,      // no-wait
		nil,        // arguments
	)
	FailOnError(err, "Failed to declare queue \"dead pool\"")

	// Save globally
	rabbitMqData.queue = &q
	rabbitMqData.ch = ch
	rabbitMqData.conn = conn
}

// Check the list of players, and return only the ones alive
func GetLivingPlayers() (players []Player) {
	DebugLog("\t[GetLivingPlayers] Running function: GetLivingPlayers()")
	for i := 0; i < 16; i++ {
		if gamedata.playerIdStates[i] == playerState.Alive {
			players = append(players, Player{
				index: i,
				id:    gamedata.playerIdList[i],
			})
		}
	}

	return
}

func ProcessUserInput(round *uint32) bool {
	DebugLogf("\t[ProcessUserInput] Running function: ProcessUserInput(round: %d)", *round)

	userInput, err := GetUserInput(round)
	FailOnError(err, "")

	if userInput.isPlayerId {
		id := userInput.optPlayerId
		history := RequestPlayerHistory(&gamedata, &GrpcData{}, id)

		ShowPlayerHistory(id, history)

	} else {
		switch userInput.optCommand {
		case "comenzar":
			return true
		}
	}

	return false
}

func GetCurrentPlayers() uint32 {
	return gamedata.currPlayers
}

func GetUserInput(round *uint32) (UserInput, error) {
	DebugLogf("\t[GetUserInput] Running function: GetUserInput(round: %d)", *round)

	log.Printf("> Para comezar la ronda %d, ingrese \"comenzar\"", round)
	log.Printf("> Si desea consultar el historial de jugadas de un jugador, ingrese el id del jugador")

	// Get user input
	reader := bufio.NewReader(os.Stdin)
	userInput, err := reader.ReadString('\n')
	FailOnError(err, "[Error] While reading your input!")

	if userInput == "comenzar\n" {
		return UserInput{optCommand: "comenzar", isPlayerId: false}, nil

	} else {
		// Convert string into an int
		i_number, err := strconv.Atoi(userInput)
		if err != nil {
			log.Println("> No se pudo interpretar bien el input.")
			return GetUserInput(round)
		}

		return UserInput{optPlayerId: uint32(i_number), isPlayerId: true}, nil
	}
}

// Prints player history to console
func ShowPlayerHistory(playerId uint32, history *[]uint32) {
	DebugLogf("\t[ShowPlayerHistory] Running function: ShowPlayer(playerId: %d, history: %v)", playerId, history)
	log.Printf("> Jugadas del jugador %d:", playerId)
	for i := 0; i < len(*history); i++ {
		log.Printf(">    %d", (*history)[i])
	}
	log.Printf("")
}

// main leader function
func Leader_go() {
	InitLogger("leader.log")

	bindAddr = os.Getenv(bindAddrEnv)

	// -- Communication setup with Pool
	RabbitMqSetup()

	// -- Main gRPC setup

	// Loop over all players to save grpc data
	for i := 0; i < 16; i++ {
		tmpAddr := strings.Join([]string{os.Getenv(playerAddrEnv), "%02d"}, "")
		addrListMap[fmt.Sprintf("player_%d", i)] = fmt.Sprintf(tmpAddr, i)
		grpcmap[fmt.Sprintf("player_%d", i)] = GrpcData{}
	}

	DebugLog("Starting game")

	// Start listening for players on another goroutine
	go LeaderToPlayerServer()

	// wait until all players have connected
	for GetCurrentPlayers() < 16 {
		DebugLogf("Waiting for players (%d/16)...", GetCurrentPlayers())
		time.Sleep(50 * time.Millisecond)
	}

	// Setup clients and other grpc data for different kind of nodes
	for entity, data := range grpcmap {
		if err := SetupDial(addrListMap[entity], &data, entity); err != nil {
			log.Fatalf("[Leader] Error while setting up gRPC preamble: %v", err)
		}

		defer grpcmap[entity].conn.Close()
		defer (*grpcmap[entity].cancel)()
	}

	defer rabbitMqData.conn.Close()
	defer rabbitMqData.ch.Close()

	// Main game loop
	stage := &gamedata.stage
	round := &gamedata.round

	DebugLog("Starting main loop")

	// Iterate over each stage
	for ; *stage < 3; (*stage)++ {

		// For each round, tell all (alive) players that the round started
		for ; *round < gamedata.numRoundsPerStage[*stage]; (*round)++ {
			DebugLogf("Starting stage:%d, round:%d", stage, round)

			// Start the next round as long as the user input specifies that.
			// Otherwise, just repeat the process
			startRound := ProcessUserInput(stage)
			if !startRound {
				(*round)--
				continue
			}

			currPlayers := GetLivingPlayers()

			// Set leader number
			switch *stage {
			case 0:
				gamedata.leaderNumber = uint32(rand.Int31n(4) + 6)
			case 1:
				gamedata.leaderNumber = uint32(rand.Int31n(4) + 1)

			case 2:
				gamedata.leaderNumber = uint32(rand.Int31n(10) + 1)

			default:
				log.Fatalf("[Error] Unreachable stage: %d", *stage)
			}

			// Kill random odd player
			if (*stage) != 0 && gamedata.currPlayers%2 != 0 {
				// Get random player
				index := rand.Int31n(int32(gamedata.currPlayers))
				playerId := currPlayers[index].id
				playerIndex := currPlayers[index].index

				DebugLogf("Killing random player id:%d", playerId)

				// Inform Pool about the dead player
				PublishDeadPlayer(&playerId, stage)
				playerKey := fmt.Sprintf("player_%d", playerIndex)
				(*grpcmap[playerKey].clientPlayer).RoundStart(*grpcmap[playerKey].ctx, &pb.RoundState{
					Stage:       stage,
					Round:       round,
					PlayerState: pb.RoundState_DEAD.Enum(),
				})

				// Update player lists
				gamedata.currPlayers--
				gamedata.playerIdStates[playerId] = playerState.Dead
				currPlayers = GetLivingPlayers()

			}

			log.Printf("> Lista de jugadores vivos en etapa %d y ronda %d:", stage, round)
			for i := 0; i < int(gamedata.currPlayers); i++ {
				playerKey := fmt.Sprintf("player_%d", currPlayers[i].index)

				(*grpcmap[playerKey].clientPlayer).RoundStart(*grpcmap[playerKey].ctx,
					&pb.RoundState{
						Stage:       stage,
						Round:       round,
						PlayerState: pb.RoundState_ALIVE.Enum(),
					})

				log.Printf(">    - Jugador %d", currPlayers[i].id)
			}
			log.Printf("")
		}
	}
	DebugLog("Ending game")
	// End of game
	finalPlayers := GetLivingPlayers()
	if len(finalPlayers) > 0 {
		log.Printf("> Los ganadores del juego del calamar son 🦑: %v ", finalPlayers)
	} else {
		log.Printf("> Ningún jugador ganó 🥺")
	}
	DebugLog("Game ended")

}
