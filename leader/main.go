package leader

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"github.com/streadway/amqp"
	"google.golang.org/grpc"
)

const (
	bindAddrEnv     = "LEADER_BIND_ADDR"
	playerAddrEnv   = "PLAYER_ADDR"
	poolAddrEnv     = "POOL_ADDR"
	namenodeAddrEnv = "NAMENODE_ADDR"
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
	cancel       *context.CancelFunc
}

type RabbitMqData struct {
	conn  *amqp.Connection
	queue *amqp.Queue
	ch    *amqp.Channel
}

type GameData struct {
	pid_list  [playerNum]uint32 // list of player's id
	pid_state [playerNum]uint32 // list with the most current player's state (dead, alive, not playing)
	pid_move  [playerNum]uint32 // list with the most current player's movement
	p_len     uint32            // num of players in the current game
	stage     uint32            // current stage
	round     uint32            // current round
	stage_len []uint32          // stage's duration (defined prior to game)
}

type Player struct {
	index int
	id    uint32
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
	}

	// global player data
	gamedata = GameData{stage: 0, round: 0, stage_len: []uint32{4, 1, 1}}

	rabbitMqData = RabbitMqData{}
)

// should work as struct
type PlayerState struct {
	Dead, Alive, NotPlaying uint32
}


// holds required data for a succesful grpc preamble dial
type GrpcData struct {
	ctx *context.Context,
	conn *grpc.ClientConn,
	client *DataRegistryServiceClient,
	cancel *grpc.DialOption
}


var (
	// PlayerState "enum"
	playerState = PlayerState{
		Dead: 0,
		Alive: 1,
		NotPlaying: 2
	}
)

// all player's data
type GameData struct {
	pid_list [playerNum]uint32 				// list of player's id
	pid_curr_state [playerNum]uint32 	// list of the most current player's state (dead, alive, not playing)
	p_len uint32											// num of players in the current game
	level uint32											// current level
}

// global player data
var gamedata = GameData{level: 1}


type server struct {
	pb.UnimplementedGameInteractionServer
}

// handle player's movement
func (s *server) PlayerAction(ctx context.Context, in *pb.PlayerMove) (*pb.PlayerState, error) {
	// player sent movement, store in namenode
	// TODO: HANDLE LEVEL SPECIFIC LOGIC (USE MUTEXES IF GAMEDATA IS TO BE MODIFIED)

	// If player died, then tell Pool to save that info
	//
	// Because RabbitMQ asks for byte arrays, the player ids (uint32)
	// should be converted

	// Transform the player id to int64 in an array of bytes
	body := fmt.Sprintf("%d %d", in.GetPlayerId(), in.GetStage())

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

	return &pb.PlayerState{}, nil
}

// handle player's join game request
func (s *server) PlayerJoin(ctx context.Context, in *pb.JoinGameRequest) (*pb.JoinGameReply, error) {
	var reply *pb.JoinGameReply

	if gamedata.p_len != playerNum || gamedata.stage != 1 {
		reply = &pb.JoinGameReply{Msg: pb.JoinGameReply_DENY_JOIN.Enum()}
	} else {
		// get player index in list, update state and increase index
		p_idx := gamedata.p_len
		gamedata.pid_list[p_idx] = in.GetPlayerId()
		gamedata.pid_state[p_idx] = playerState.Alive
		gamedata.p_len++
		reply = &pb.JoinGameReply{Msg: pb.JoinGameReply_ACCEPT_JOIN.Enum()}
	}

	return reply, nil
}

// handle player's command (e.g. read pool prize)
func (s *server) RequestCommand(ctx context.Context, in *pb.PlayerCommand) (*pb.CommandReply, error) {
	var reply *pb.CommandReply

	switch in.GetCommand() {
	case pb.PlayerCommand_POOL:

	}

	return reply, nil
}

// request current accumulated prize to pool node
func RequestPrize(ctx context.Context, client pb.PrizeClient, prize *uint32) uint32 {
	response, err := client.GetPrize(ctx,
		&pb.CurrentPoolRequest{Prize: prize},
	)
	FailOnError(err, "[Leader] Couldn't communicate with Pool")

	resp := response.GetCurrPrize()
	log.Printf("[Leader] Pool prize response: %v", resp)

	return resp
}

// call grpc preamble
func SetupDial(addr string, grpcdata *GrpcData, entity string) error {
	connection, err := grpc.Dial(addr, grpc.WithInsecure())
	grpcdata.conn = connection

	if err != nil {
		log.Fatalf("[Error] Couldn't connect to target: %v", err)
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
	lis, err := net.Listen("tcp", bindAddr)
	FailOnError(err, "[Leader] failed to listen on address")

	leader_srv := grpc.NewServer()
	pb.RegisterGameInteractionServer(leader_srv, &server{})
	log.Printf("[Leader] Listening at %v", lis.Addr())

	if err := leader_srv.Serve(lis); err != nil {
		log.Fatalf("[Leader] Could not bind to %v : %v", bindAddr, err)
	}
}

// request player history from namenode (CURRENTLY NOT USED ANYWHERE)
func RequestPlayerHistory(gamedata *GameData, grpcdata *GrpcData, playerId uint32) *[]uint32 {
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
	// Store each move in dynamic slice
	all_moves := make([]*pb.PlayersMoves_Move, playerNum)

	for i := 0; i < playerNum; i++ {
		all_moves[i] = &pb.PlayersMoves_Move{
			PlayerId:   &gamedata.pid_list[i],
			PlayerMove: &gamedata.pid_move[i],
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
	// -- Special setup to dial Pool with RabbitMQ
	conn, err := amqp.Dial(addrListMap["pool"])
	FailOnError(err, "[Leader] Failed to connect to Pool using RabbitMQ")

	ch, err := conn.Channel()
	FailOnError(err, "[Leader] Failed to open a channel with Pool")

	q, err := ch.QueueDeclare(
		"deadpool", // name
		false,      // durable
		false,      // delete when unused
		false,      // exclusive
		false,      // no-wait
		nil,        // arguments
	)
	FailOnError(err, "[Leader] Failed to declare queue \"money pool\"")

	// Save globally
	rabbitMqData.queue = &q
	rabbitMqData.ch = ch
	rabbitMqData.conn = conn
}

func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

// Check the list of players, and return only the ones alive
func GetLivingPlayers() (players []Player) {
	for i := 0; i < 16; i++ {
		if gamedata.pid_state[i] == playerState.Alive {
			players = append(players, Player{
				index: i,
				id:    gamedata.pid_list[i],
			})
		}
	}

	return
}

// main leader function
func Leader_go() {

	bindAddr = os.Getenv(bindAddrEnv)

	// -- Communication setup with Pool
	RabbitMqSetup()

	// -- Main gRPC setup

	// Map with struct that saves basic grpc dial data
	var grpcmap = map[string]GrpcData{
		"namenode": {},
		"pool":     {},
	}

	// Loop over all players to save grpc data
	for i := 0; i < 16; i++ {
		tmpAddr := strings.Join([]string{os.Getenv(playerAddrEnv), "%02d"}, "")
		addrListMap[fmt.Sprintf("player_%d", i)] = fmt.Sprintf(tmpAddr, i)
		grpcmap[fmt.Sprintf("player_%d", i)] = GrpcData{}
	}

	log.Println("[Leader] Starting Squid Game...")

	// Start listening for players on another goroutine
	go LeaderToPlayerServer()

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

	// Iterate over each stage
	for ; *stage < 3; (*stage)++ {
		log.Printf("[Level %v] Started...\n", stage)

		currPlayers := GetLivingPlayers()

		// For each round, tell all (alive) players that the round started
		for ; *round < gamedata.stage_len[*stage]; (*round)++ {
			for i := 0; i < len(currPlayers); i++ {
				playerKey := fmt.Sprintf("player_%d", currPlayers[i].index)

				(*grpcmap[playerKey].clientPlayer).RoundStart(*grpcmap[playerKey].ctx,
					&pb.RoundState{
						Stage: stage,
						Round: round,
					})
			}
		}
	}
}
