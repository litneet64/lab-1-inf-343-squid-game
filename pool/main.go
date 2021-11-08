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
	rabbitMqAddrEnv = "RABBITMQ_ADDR"
	bindAddrEnv     = "POOL_BIND_ADDR"
)

var (
	bindAddr, rabbitMqAddr string
)

type server struct {
	pb.UnimplementedPrizeServer
}

// writes last player death and new pool prize
func RegisterPlayerDeath(player uint32, stage uint32) {
	DebugLogf("\t[Server:RegisterPlayerDeath] Running function: RegisterPlayerDeath(player: %d, stage: %d)", player, stage)

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
	DebugLogf("\t[RetrievePrice] Running function: RetrievePrice()")

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
	DebugLogf("\t[setupPoolServer] Running function: setupPoolServer()")

	lis, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterPrizeServer(s, &server{})
	DebugLogf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (s *server) GetPrize(ct context.Context, in *pb.CurrentPoolRequest) (*pb.CurrentPoolReply, error) {
	price := RetrievePrice()

	return &pb.CurrentPoolReply{CurrPrize: &price}, nil
}

func Pool_go() {
	InitLogger("pool.log")

	bindAddr = os.Getenv(bindAddrEnv)
	rabbitMqAddr = os.Getenv(rabbitMqAddrEnv)

	// grpc conection
	go setupPoolServer()
	// Dial Leader
	DebugLog("Dialing RabbitMq pool")
	conn, err := amqp.Dial(rabbitMqAddr)
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
			DebugLogf("Registering player %d, who died in stage %d", playerId, stage)
		}
	}()

	DebugLogf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}
