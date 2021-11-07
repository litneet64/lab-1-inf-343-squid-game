package main

import (
	"log"
	"os"

	"github.com/litneet64/lab-2-squid-game/datanode"
	"github.com/litneet64/lab-2-squid-game/leader"
	"github.com/litneet64/lab-2-squid-game/namenode"
	"github.com/litneet64/lab-2-squid-game/player"
	"github.com/litneet64/lab-2-squid-game/pool"
)

const (
	playerId  = 0
	playerNum = 16
)

func show_help() {
	log.Fatalf("[!] Usage: %s <role [leader/player/namenode/datanode/pool]>", os.Args[0])
}

func main() {
	if len(os.Args[:]) < 2 {
		show_help()
	}

	switch cmd := os.Args[1]; cmd {
	case "leader":
		leader.Leader_go()
	case "player":
		// Instantiate other 15 players
		for id := 1; id < playerNum; id++ {
			go player.Player_go("bot", uint32(id))
		}

		player.Player_go("human", playerId)
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
