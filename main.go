package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/litneet64/lab-2-squid-game/datanode"
	"github.com/litneet64/lab-2-squid-game/leader"
	"github.com/litneet64/lab-2-squid-game/namenode"
	"github.com/litneet64/lab-2-squid-game/player"
	"github.com/litneet64/lab-2-squid-game/pool"
)

const (
	playerNum = 16
)

func show_help() {
	log.Fatalf("[!] Usage: %s <role [leader/player/playerbot/namenode/datanode/pool]>", os.Args[0])
}

func main() {
	if len(os.Args[:]) < 2 {
		show_help()
	}

	parseId := flag.Int("player-id", 0, "Specify player's ID (for bot's internals)")
	flag.Parse()

	switch cmd := os.Args[1]; cmd {
	case "leader":
		leader.Leader_go()
	case "player":
		// Spawn other 15 players on their own processes
		for id := 1; id < playerNum; id++ {
			go exec.Command(os.Args[0], "playerbot", fmt.Sprintf("-player-id=%d", id))
		}

		player.Player_go("human", 0)
	case "playerbot":
		player.Player_go("bot", uint32(*parseId))
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
