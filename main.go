package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/MildC/dht-crawler/dht"
	"github.com/MildC/dht-crawler/torrent"
)

func main() {
	go func() {
		http.ListenAndServe(":6060", nil)
	}()

	logger := NewConsoleLogger()

	w := dht.NewWire(65536, 1024, 256)
	go func() {
		for resp := range w.Response() {
			metadata, err := dht.Decode(resp.MetadataInfo)
			if err != nil {
				continue
			}
			info := metadata.(map[string]interface{})

			if _, ok := info["name"]; !ok {
				continue
			}

			bt := torrent.BitTorrent{
				InfoHash: hex.EncodeToString(resp.InfoHash),
				Name:     info["name"].(string),
			}

			if v, ok := info["files"]; ok {
				files := v.([]interface{})
				bt.Files = make([]torrent.File, len(files))

				for i, item := range files {
					f := item.(map[string]interface{})
					bt.Files[i] = torrent.File{
						Path:   f["path"].([]interface{}),
						Length: f["length"].(int),
					}
				}
			} else if _, ok := info["length"]; ok {
				bt.Length = info["length"].(int)
			}

			data, err := json.Marshal(bt)
			if err == nil {
				fmt.Printf("%s\n\n", data)
			}
		}
	}()
	go w.Run()

	config := dht.NewCrawlConfig()
	config.OnAnnouncePeer = func(infoHash, ip string, port int) {
		w.Request([]byte(infoHash), ip, port)
	}
	d := dht.New(logger, config)

	d.Run()
}
