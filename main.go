package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudflare/cloudflare-go"
	"github.com/vishvananda/netlink"
)

func main() {
	cf_token, ok := os.LookupEnv("CF_API_TOKEN")
	if !ok {
		log.Fatalf("CF_API_TOKEN environment variable must be set")
	}

	err := syncAddress(cf_token, "travishegner.com", "travishegner.com")
	if err != nil {
		log.Fatalf("failure while sync'ing address:\n\t%v\n", err)
	}

	fmt.Println("done")
	os.Exit(0)

	sig := make(chan os.Signal, 1)

	//handle signals
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

MainLoop:
	for {
		select {
		case s := <-sig:
			fmt.Printf("captured %v signal, exiting\n", s)
			break MainLoop
		}
	}

	fmt.Println("tetelestai")
}

func syncAddress(cf_token, zone, hostname string) error {
	api, err := cloudflare.NewWithAPIToken(cf_token)
	if err != nil {
		return fmt.Errorf("failed to get cloudflare API:\n\t%w\n", err)
	}
	ctx := context.Background()

	zoneID, err := api.ZoneIDByName(zone)
	if err != nil {
		return fmt.Errorf("failed to get zone %v: %w", zone, err)
	}

	searchRec := cloudflare.DNSRecord{Type: "A", Name: hostname}

	recs, err := api.DNSRecords(ctx, zoneID, searchRec)
	if err != nil {
		return fmt.Errorf("failed to search for dns record %v:\n\t%w\n", hostname, err)
	}

	if len(recs) < 1 {
		return fmt.Errorf("no records found for %v in zone %v", hostname, zone)
	}

	defRoute := net.ParseIP("8.8.8.8")
	routes, err := netlink.RouteGet(defRoute)
	if err != nil {
		return fmt.Errorf("failed to get default route:\n\t%w\n", err)
	}

	if len(routes) < 1 {
		return fmt.Errorf("no route found to 8.8.8.8")
	}

	fmt.Println(recs[0].Content, routes[0].Src.String())
	return nil
}
