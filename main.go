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

	playerBotFlag := flag.NewFlagSet("playerbot", flag.ExitOnError)
	playerId := playerBotFlag.Int("playerid", 1, "Specify player's ID (bot internals)")

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
		playerBotFlag.Parse(os.Args[2:])
		player.Player_go("bot", uint32(*playerId))
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
