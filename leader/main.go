package leader

import (
	"context"
	"log"
	"net"
	"time"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"google.golang.org/grpc"
)

const (
	bindAddr = "0.0.0.0:50051"
	poolAddr = "dist183:50051"
	namenodeAddr = "dist182:50051"
	playerNum = 16
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


// handle player's request
func (s *server) PlayerSend(ctx context.Context, in *pb.PlayerToLeaderRequest) (*pb.PlayerToLeaderReply, error) {
	var reply *pb.PlayerToLeaderReply

	switch in.GetMsg() {
		case 0: // player sent movement, store in namenode
			// TODO: HANDLE LEVEL SPECIFIC LOGIC (USE MUTEXES IF GAMEDATA IS TO BE MODIFIED)
			// TODO: COMMUNICATE RESULTS TO POOL
		case 1: // player sent join_game request
			if gamedata.p_len != playerNum || gamedata.level != 1 {
				reply = &pb.PlayerToLeaderReply{Msg: pb.PlayerToLeaderReply_DENY_JOIN.Enum()}

			} else {
				// get player index in list, update state and increase index
				p_idx := gamedata.p_len
				gamedata.pid_list[p_idx] = in.GetPlayerId()
				gamedata.pid_state[p_idx] = playerstate.Alive
				gamedata.p_len++
				reply = &pb.PlayerToLeaderReply{Msg: pb.PlayerToLeaderReply_ACCEPT_JOIN.Enum()}
			}
		default:
			log.Fatalf("[Leader] Recieved wrong type of msg (%v) from player: %v",
					in.GetMsg(),
					in.GetPlayerId(),
				)
	}

	log.Printf("Received: %v", in.GetMsg())

	return reply, nil
}


// request current accumulated prize to pool node
func RequestPrize(ctx context.Context, client pb.PrizeClient, prize *uint32) uint32 {
	response, err := client.GetPrize(ctz,
		&pb.CurrentPoolRequest{prize: prize},
	})

	if err != nil {
		log.Fatalf("[Leader] Couldn't communicate with Pool: %v", err)
	}

	resp := response.GetCurrPrize()
	log.Printf("[Leader] Pool prize response: %v", resp)

	return resp
}

// call grpc preamble
func SetupDial(addr string, grpcdata *GrpcData) (error) {
	connection, err := grpc.Dial(addr, grpc.WithInsecure())
	grpcdata.conn = connection

	if err != nil {
		log.Fatalf("[Error] Couldn't connect to target: %v", err)
	} else {
		log.Println("Connection was successful")
	}

	grpcdata.client = pb.NewDataRegistryServiceClient(grpcdata.conn)
	grpcdata.ctx, grpcdata.cancel = context.WithTimeout(context.Background(), time.Second*10)

	return err
}


// main server for player functionality
func LeaderToPlayerServer() {
	lis, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatalf("[Leader] Could not listen: %v", err)
	}

	leader_srv := grpc.NewServer()
	pb.RegisterGameInteractionServer(leader_srv, &server{})
	log.Printf("[Leader] Listening at %v", lis.Addr())

	if err := leader_srv.Serve(lis); err != nil {
		log.Fatalf("[Leader] Could not bind to %v : %v", bindAddr, err)
	}
}


// request player history from namenode
func RequestPlayerHistory() {

}



// main leader function
func Leader_go() {
	// map with struct that saves basic grpc dial data
	var grpcmap = map[string]GrpcData {
		"namenode": GrpcData{},
		"pool": GrpcData{},
	}

	log.Println("[Leader] Starting Squid Game...")

	// start listening for players on another goroutine
	go LeaderToPlayerServer()

	// setup clients and other grpc data for different kind of nodes
	for k, _ := range grpcmap {
		if err := SetupDial(namenodeAddr, &grpcmap[k]); err != nil {
			log.Fatalf("[Leader] Error while setting up gRPC preamble: %v", err)
		}

		defer grpcmap[k].conn.Close()
		defer grpcmap[k].cancel()
	}

	// main game loop
	for ; gamedata.level < 3; gamedata.level++ {
		log.Printf("[Level %v] Started...\n", gamedata.level)

		// tell players

		// communicate with pool when if players are dead already
	}
}
