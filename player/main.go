package player

import (
	"context"
	"log"
	"time"
	"bufio"
	"os"
	"fmt"
	"strconv"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"google.golang.org/grpc"
)

const (
	address     = "localhost:50051" // Cambiar despues
	defaultName = "leader"          // También
)


func sendMove(ctx context.Context, client *pb.GameInteractionClient, moves uint32) (resp string, err error) {
	response, err := client.PlayerSend(ctx, &pb.PlayerToLeaderRequest{Msg: pb.PlayerToLeaderRequest_MOVE.Enum(), Moves: moves})
	if err != nil {
		log.Fatalf("[Player] Could not send message to server: %v", err)
	}

	resp := response.GetMsg()

	log.Printf("[Player] Message response: %v", resp)
	return
}


// user movement function
func getInput() (n uint32, err error) {
	var inp string
	err = nil

	fmt.Print("> Insert move: ")
	reader := bufio.NewReader(os.Stdin)

	if inp, err := reader.ReadString("\n"); err != nil {
		log.Println("[Error] While reading your input!")
		return
	}

	if n, err = strconv.Atoi(inp); err {
		log.Println("[Error] Can only parse integers!")
		return
	}

	return
}


// bot movement generator
func autoMove() (n uint32, err error) {
	// CHECK: that this is the correct range
	n = rand.Intn(10)

	return
}


func Player_go(player_type string) {
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancel()

	// request to join game
	p_id := rand.Intn(2 << 32 - 1)
	res, err := client.PlayerSend(ctx, &pb.PlayerToLeaderRequest{Msg: pb.PlayerToLeaderRequest_JOIN_GAME.Enum(), PlayerId: p_id})

	if err {
		log.Fatalf("[Error] Couldn't connect to leader\n")
	}

	if state := res.GetState(); state != 1 {
		log.Fatalf("[Error] Leader rejected game joining request\n")
	}

	// main game loop
	for i := 1;; i++ {
		fmt.Printf("[Round %v]\n", i)

		switch player_type {
			case "bot":
				mov := autoMove()
			case "human":
				// get user input and parse to int
				if mov, err := getInput(); err {
					log.Println("[Error] While reading user input: %v", err)
					mov = autoMove()
				}
			default:
				log.Fatalf("[Error] Wrong usage of Player_go function!\n")
		}

		// send player move and recieve player status
		if state_res, err := sendMove(ctx, &client, mov); err {
			log.Println("[Error] While sending moves: %v", err)
		}

		// do something with state
	}
}
