// Command apogee-probe discovers skies on the local network, joins one and
// prints the states it receives. It is the network spike's test client.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"apogee/internal/discovery"
	"apogee/internal/netclient"
)

func main() {
	addr := flag.String("addr", "", "server address; empty to discover one with mDNS")
	name := flag.String("name", "probe", "player name")
	n := flag.Int("n", 3, "states to print before quitting")
	wait := flag.Duration("wait", 3*time.Second, "mDNS discovery duration")
	flag.Parse()

	ctx := context.Background()
	if *addr == "" {
		servers, err := discovery.Browse(ctx, *wait)
		if err != nil {
			log.Fatal(err)
		}
		if len(servers) == 0 {
			log.Fatal("no sky found: multicast may be blocked, use -addr host:port")
		}
		for _, s := range servers {
			fmt.Printf("found %q at %s\n", s.Name, s.Addr)
		}
		*addr = servers[0].Addr
	}
	key, err := netclient.PlayerKey()
	if err != nil {
		log.Fatal(err)
	}
	c, err := netclient.Dial(ctx, *addr, *name, key)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	fmt.Printf("joined %q, island %d\n", c.Welcome.Sky, c.Welcome.Island)
	for range *n {
		st, err := c.Next()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("tick %d, %s (%ds left), %d island(s)\n", st.Tick, st.Phase, st.PhaseLeftMs/1000, len(st.Islands))
	}
}
