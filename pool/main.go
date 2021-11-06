package pool

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	pb "github.com/litneet64/lab-2-squid-game/protogrpc"
	"github.com/streadway/amqp"
	"google.golang.org/grpc"
)

const (
	leaderRabbitEnv = "LEADER_RABBIT_ADDR"
	bindAddrEnv     = "POOL_BIND_ADDR"
)

var (
	bindAddr, leaderRabbitAddr string
)

type server struct {
	pb.UnimplementedPrizeServer
}

func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("[Error]: (%v) %s", err, msg)
	}
}

// writes last player death and new pool prize
func RegisterPlayerDeath(player uint32, stage uint32) {
	var f *os.File
	var err error
	var money uint32

	_, fErr := os.Stat("pool.txt")
	if fErr != nil {
		f, err = os.Create("pool.txt")
		money = 0
	}

	f, err = os.Open("pool.txt")

	FailOnError(err, "can't open file \"pool.txt\"")
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		moneyint, mErr := strconv.Atoi(strings.Split(scanner.Text(), " ")[2])
		FailOnError(mErr, "error")
		money = uint32(moneyint)
	}

	_, wErr := f.WriteString(fmt.Sprintf("Jugador_%d Ronda_%d %d", player, stage, money+100))
	FailOnError(wErr, "error")

	f.Sync()
}

// return pool prize
func RetrievePrice() (prize uint32) {
	prize = 0

	_, fErr := os.Stat("pool.txt")
	if fErr != nil {
		return 0
	}
	f, err := os.Open("pool.txt")

	FailOnError(err, "can't open file \"pool.txt\"")
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		moneyint, mErr := strconv.Atoi(strings.Split(scanner.Text(), " ")[2])
		FailOnError(mErr, "error")
		prize = uint32(moneyint)
	}

	return
}

func setupPoolServer() {
	lis, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterPrizeServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (s *server) GetPrize(ct context.Context, in *pb.CurrentPoolRequest) (*pb.CurrentPoolReply, error) {
	price := RetrievePrice()

	return &pb.CurrentPoolReply{CurrPrize: &price}, nil
}

func Pool_go() {

	bindAddr = os.Getenv(bindAddrEnv)
	leaderRabbitAddr = os.Getenv(leaderRabbitEnv)

	// grpc conection
	go setupPoolServer()
	// Dial Leader
	conn, err := amqp.Dial(leaderRabbitAddr)
	FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	FailOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"money pool", // name
		false,        // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	FailOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	FailOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			text := strings.Split(fmt.Sprint(d.Body), " ")
			playerId, _ := strconv.Atoi(text[0])
			stage, _ := strconv.Atoi(text[1])
			RegisterPlayerDeath(uint32(playerId), uint32(stage))
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
	log.Println("[+] Success!")
}
