package main

import (
	"errors"
	"log"
	"net"
	"context"
	"time"
	"flag"
	"io/ioutil"
	"strings"
	hetzner_dns "github.com/panta/go-hetzner-dns"
)

type Rule struct {
	CurrentIp net.IP
	IfaceName string
	DnsRecord string
}

type DnsClient struct {
	client hetzner_dns.Client
	zoneId string
}

func main() {
	var iface string
    var record string
	var secretPath string
	var timeBetweanChecks int
	var timeBetweanUpdates int
 
    flag.StringVar(&iface, "i", "", "iface")
    flag.StringVar(&record, "r", "", "record")
	flag.StringVar(&secretPath, "s", "/etc/node-ddns-controller/key", "secret path")
	flag.IntVar(&timeBetweanChecks, "c",  30, "timeBetweanChecks")
    flag.IntVar(&timeBetweanUpdates, "u",  60*60, "timeBetweanUpdates")

 
    flag.Parse() 

	if (iface == "" || record == "") {
		log.Fatal("-i and -r must be set")
	}

	log.Printf("loading secret from %s", secretPath)
	secret, err := ioutil.ReadFile(secretPath)
    if err != nil {
        log.Fatal(err)
    }

	rule := Rule{nil, iface, record}

	client, err := NewDnsClient("d4rk.io", strings.TrimSuffix(string(secret), "\n"))
	if err != nil {
		log.Fatal(err)
	}

	tickerCheckAndUpdate := time.NewTicker(time.Duration(timeBetweanChecks) * time.Second)
	tickerUpdate := time.NewTicker(time.Duration(timeBetweanUpdates) * time.Second)

	if rule.updateLocalIp() {
		client.updateRecord(rule.CurrentIp.String(), rule.DnsRecord)
	}
	log.Println("waiting ...")

	for {
		select {
			case <- tickerCheckAndUpdate.C:
				if rule.updateLocalIp() {
					client.updateRecord(rule.CurrentIp.String(), rule.DnsRecord)
				}
				log.Println("waiting ...")
			case <- tickerUpdate.C:
				if rule.CurrentIp != nil {
					client.updateRecord(rule.CurrentIp.String(), rule.DnsRecord)
				}
		}
	}
}

func NewDnsClient(name string, apiKey string) (*DnsClient, error) {
	client := hetzner_dns.Client{ApiKey: apiKey}

	zonesResponse, err := client.GetZones(context.Background(), "", "", 1, 100)
	if err != nil {
		return nil, err
	}

	for _, zone := range zonesResponse.Zones {
			if zone.Name == name {
				log.Printf("found zoneid: %s for %s", zone.ID, zone.Name)

				return &DnsClient{client, zone.ID}, nil
			}
	}

	return nil, errors.New("Zone not found")
}

func (dc *DnsClient) updateRecord(value string, name string) error {
	log.Printf("Update dns record, zoneid: %s, name: %s, value %s", dc.zoneId, value, name)

	recordResponse, err := dc.client.CreateOrUpdateRecord(context.Background(), hetzner_dns.RecordRequest{
		ZoneID: dc.zoneId,
		Type:   "AAAA",
		Name:   name,
		Value:  value,
		TTL: 60,
	})

	if err != nil {
		return err
	}

	log.Println("Updated dns")
	log.Printf("Created: %v\n", recordResponse.Record)

	return nil
}

func (rule *Rule) updateLocalIp() bool {
	log.Println("Check if ip has changed")

	iface, err := getInterfaceByName(rule.IfaceName)
	if err != nil {
		log.Fatalln(err)
	}

	ip, err := getGlobalPublicIPv6(iface)
	if err != nil {
		log.Fatalln(err)
	}

	if ip.Equal(rule.CurrentIp) {
		return false
	}

	rule.CurrentIp = ip
	return true
}

func getInterfaceByName(name string) (*net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		if iface.Name == name {
			if iface.Flags&net.FlagUp == 0 {
				return nil, errors.New("interface is down")
			}

			return &iface, nil
		}
	}

	return nil, errors.New("Interface not found")
}

func getGlobalPublicIPv6(iface *net.Interface) (net.IP, error) {
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				log.Printf("Unsupported addrs type")
				continue
		}

		var isGlobal = ip.IsGlobalUnicast()
		var isPublic = !ip.IsPrivate()
		var isIPv6 = nil == ip.To4()

		log.Printf("%s %t %t %t", ip, isGlobal, isPublic, isIPv6)

		if isGlobal && isPublic && isIPv6 {
			log.Printf("Stoppin search found %s", ip)

			return ip, nil
		}
	}

	return nil, errors.New("No Global Public ipv6 not found")
}