package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/litneet64/lab-2-squid-game/datanode"
	"github.com/litneet64/lab-2-squid-game/leader"
	"github.com/litneet64/lab-2-squid-game/namenode"
	"github.com/litneet64/lab-2-squid-game/player"
	"github.com/litneet64/lab-2-squid-game/pool"
)

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
	playerNum = 16
)

func show_help() {
	log.Fatalf("[!] Usage: %s <role [leader/player/playerbot/namenode/datanode/pool]>", os.Args[0])
}

func main() {
	InitLogger("hub.log")

	if len(os.Args[:]) < 2 {
		show_help()
	}
	var playerId int

	flag.IntVar(&playerId, "playerid", 0, "Specify player's ID (bot internals)")
	flag.Parse()
	DebugLogf("Arguments received: %s \nFlags parsed: playerId: %d ", strings.Join(os.Args[1:], ", "), playerId)

	switch cmd := os.Args[1]; cmd {
	case "leader":
		leader.Leader_go()
	case "player":
		// Spawn other 15 players on their own processes
		for id := 1; id < playerNum; id++ {
			exec.Command("/bin/bash", "-c", fmt.Sprintf("%v playerbot -playerid %d &", os.Args[0], id)).Start()
		}

		player.Player_go("human", 0)
	case "playerbot":
		player.Player_go("bot", uint32(playerId))
	case "namenode":
		namenode.Namenode_go()
	case "datanode":
		datanode.Datanode_go()
	case "pool":
		pool.Pool_go()
	default:
		show_help()
	}

}
