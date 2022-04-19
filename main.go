package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/vishvananda/netlink"
)

func main() {
	cf_token, ok := os.LookupEnv("CF_API_TOKEN")
	if !ok {
		log.Fatalf("CF_API_TOKEN environment variable must be set\n")
	}

	//set these from environment or cli args
	zoneName := "travishegner.com"
	recordName := "travishegner.com"

	defRoute := net.ParseIP("8.8.8.8")
	routes, err := netlink.RouteGet(defRoute)
	if err != nil {
		log.Fatalf("failed to get default route:\n\t%v\n", err)
	}

	if len(routes) < 1 {
		log.Fatal("no route found to 8.8.8.8")
	}

	localAddress := routes[0].Src.String()
	localLink := routes[0].LinkIndex
	syncInterval := 10 * time.Minute

	err = syncAddress(cf_token, zoneName, recordName, localAddress)
	if err != nil {
		log.Fatalf("failure during initial sync'ing of address:\n\t%v\n", err)
	}

	done := make(chan struct{}, 1)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	auc := make(chan netlink.AddrUpdate, 1)
	err = netlink.AddrSubscribe(auc, done)
	if err != nil {
		log.Fatalf("failure subscribing to address updates\n")
	}

	timec := time.NewTicker(syncInterval)

MainLoop:
	for {
		select {
		case s := <-sigc:
			fmt.Printf("captured %v signal, exiting\n", s)
			break MainLoop
		case au := <-auc:
			if au.LinkIndex == localLink {
				log.Printf("detected address change on default link\n")
				err = syncAddress(cf_token, zoneName, recordName, localAddress)
				if err != nil {
					log.Printf("failure during sync after detected address change:\n\t%v\n", err)
				}
			}
		case t := <-timec.C:
			log.Printf("ticker fired at %v, sync'ing address\n", t)
			err = syncAddress(cf_token, zoneName, recordName, localAddress)
			if err != nil {
				log.Printf("failure during sync on interval:\n\t%v\n", err)
			}
		}
	}

	close(done)
	fmt.Println("tetelestai")
}

func syncAddress(cf_token, zone, hostname, address string) error {
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

	if recs[0].Content != address {
		log.Printf("dns record points to %v, but our interface is %v. Updating dns record...", recs[0].Content, address)
		newRec := cloudflare.DNSRecord{Type: "A", Name: hostname, Content: address, TTL: 60}
		err = api.UpdateDNSRecord(ctx, zoneID, recs[0].ID, newRec)
		if err != nil {
			return fmt.Errorf("failed to update record %v to content %v:\n\t%w\n", hostname, address, err)
		}
		return nil
	}

	log.Printf("dns record matches local interface, no updates necessary")

	return nil
}
