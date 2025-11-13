package main

import (
  "crypto/sha256"
  "encoding/hex"
  "encoding/json"
  "log"
  "net"
  "net/http"
  "time"
  "github.com/grandcat/zeroconf"
)

func fingerprint(cert []byte) string {
  h := sha256.Sum256(cert)
  return hex.EncodeToString(h[:])
}

func startMdns(name string, port int, fp string) (*zeroconf.Server, error) {
  meta := []string{"device=" + name, "port=8444", "fp=" + fp}
  return zeroconf.Register("AirShare", "_airshare._tcp", "local.", port, meta, nil)
}

func startUdp(port int) (*net.UDPConn, error) {
  addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:53317")
  if err != nil { return nil, err }
  conn, err := net.ListenUDP("udp4", nil)
  if err != nil { return nil, err }
  go func() {
    for {
      msg := []byte("HELLO " + time.Now().Format(time.RFC3339))
      conn.WriteToUDP(msg, addr)
      time.Sleep(3 * time.Second)
    }
  }()
  return conn, nil
}

func main() {
  fp := ""
  srv, err := startMdns("Agent", 8444, fp)
  if err != nil { log.Fatal(err) }
  defer srv.Shutdown()
  _, err = startUdp(53317)
  if err != nil { log.Fatal(err) }
  devices := make([]map[string]any, 0)
  res, _ := zeroconf.NewResolver(nil)
  entries := make(chan *zeroconf.ServiceEntry)
  go func(results <-chan *zeroconf.ServiceEntry) {
    for e := range results {
      addr := ""
      if len(e.AddrIPv4) > 0 { addr = e.AddrIPv4[0].String() }
      device := map[string]any{"name": e.Instance, "addr": addr, "port": e.Port, "meta": e.Text}
      devices = append(devices, device)
    }
  }(entries)
  res.Browse("_airshare._tcp", "local.", entries)
  http.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
  http.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) { b, _ := json.Marshal(devices); w.Header().Set("Content-Type", "application/json"); w.Write(b) })
  log.Println("Agent running on :8444")
  http.ListenAndServe(":8444", nil)
}
