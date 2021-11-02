package main;

import (
    "log"
    "os"
    "github.com/litneet64/lab-2-squid-game/leader"
    "github.com/litneet64/lab-2-squid-game/player"
    "github.com/litneet64/lab-2-squid-game/datanode"
    "github.com/litneet64/lab-2-squid-game/namenode"
    "github.com/litneet64/lab-2-squid-game/pool"
)

func show_help() {
  log.Printf("[!] Usage: %s <role [leader/player_bot/player/namenode/datanode/pool]>", os.Args[0])
  os.Exit(127)
}


func main() {
    if len(os.Args[:]) < 2 {
      show_help()
    }

    switch cmd := os.Args[1]; cmd {
      case "leader":
        leader.Leader_go()
      case "player_bot":
        player.Player_bot_go()
      case "player":
        player.Player_human_go()
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
