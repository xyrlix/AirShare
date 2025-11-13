package main

import (
  "crypto/rand"
  "crypto/rsa"
  "crypto/tls"
  "crypto/x509"
  "crypto/x509/pkix"
  "crypto/sha256"
  "encoding/hex"
  "encoding/json"
  "encoding/pem"
  "fmt"
  "io"
  "log"
  "math/big"
  "net"
  "net/http"
  "os"
  "path/filepath"
  "time"
  "github.com/grandcat/zeroconf"
  "strconv"
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

func createSelfSigned() (tls.Certificate, []byte) {
  priv, _ := rsa.GenerateKey(rand.Reader, 2048)
  tmpl := x509.Certificate{
    SerialNumber: bigInt(),
    Subject: pkix.Name{CommonName: "AirShare"},
    NotBefore: time.Now().Add(-time.Hour),
    NotAfter: time.Now().Add(365 * 24 * time.Hour),
    KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
    ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
    BasicConstraintsValid: true,
    DNSNames: []string{"localhost"},
  }
  der, _ := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
  certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
  keyPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
  crt, _ := tls.X509KeyPair(certPem, keyPem)
  return crt, der
}

func bigInt() *big.Int { return new(big.Int).SetInt64(time.Now().UnixNano()) }

func main() {
  tlsCert, der := createSelfSigned()
  fp := fingerprint(der)
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
  dataDir := filepath.Join(".", "data")
  os.MkdirAll(dataDir, 0755)
  chunks := make(map[string]map[int][]byte)
  mux := http.NewServeMux()
  mux.HandleFunc("/api/cert", func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", "application/json"); w.Write([]byte("{\"fp\":\"" + fp + "\"}")) })
  mux.HandleFunc("/api/files/list", func(w http.ResponseWriter, r *http.Request) {
    type item struct{ Name string `json:"name"`; Size int64 `json:"size"` }
    list := []item{}
    entries, _ := os.ReadDir(dataDir)
    for _, e := range entries {
      p := filepath.Join(dataDir, e.Name())
      st, _ := os.Stat(p)
      list = append(list, item{ Name: e.Name(), Size: st.Size() })
    }
    b, _ := json.Marshal(map[string]any{"files": list})
    w.Header().Set("Content-Type", "application/json")
    w.Write(b)
  })
  mux.HandleFunc("/api/files/upload/init", func(w http.ResponseWriter, r *http.Request) {
    var obj map[string]any
    json.NewDecoder(r.Body).Decode(&obj)
    id, _ := obj["id"].(string)
    if id == "" { w.WriteHeader(400); w.Write([]byte("{\"error\":\"param\"}")); return }
    chunks[id] = make(map[int][]byte)
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte("{\"ok\":true}"))
  })
  mux.HandleFunc("/api/files/upload/chunk", func(w http.ResponseWriter, r *http.Request) {
    id := r.URL.Query().Get("id")
    idxStr := r.URL.Query().Get("index")
    if id == "" { w.WriteHeader(400); w.Write([]byte("{\"error\":\"param\"}")); return }
    b, _ := io.ReadAll(r.Body)
    idx, _ := strconv.Atoi(idxStr)
    m, ok := chunks[id]
    if !ok { m = make(map[int][]byte) }
    m[idx] = b
    chunks[id] = m
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte("{\"ok\":true}"))
  })
  mux.HandleFunc("/api/files/upload/finish", func(w http.ResponseWriter, r *http.Request) {
    var obj map[string]any
    json.NewDecoder(r.Body).Decode(&obj)
    id, _ := obj["id"].(string)
    name, _ := obj["name"].(string)
    total, _ := obj["total"].(float64)
    m, ok := chunks[id]
    if !ok || name == "" { w.WriteHeader(400); w.Write([]byte("{\"error\":\"param\"}")); return }
    parts := make([][]byte, 0)
    for i := 0; i < int(total); i++ { if b, ok := m[i]; ok { parts = append(parts, b) } }
    buf := make([]byte, 0)
    for _, p := range parts { buf = append(buf, p...) }
    os.WriteFile(filepath.Join(dataDir, name), buf, 0644)
    delete(chunks, id)
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte("{\"ok\":true}"))
  })
  mux.HandleFunc("/api/files/download", func(w http.ResponseWriter, r *http.Request) {
    name := r.URL.Query().Get("name")
    p := filepath.Join(dataDir, name)
    st, err := os.Stat(p)
    if err != nil { w.WriteHeader(404); return }
    rng := r.Header.Get("Range")
    if rng != "" {
      var start, end int64
      fmt.Sscanf(rng, "bytes=%d-%d", &start, &end)
      if end == 0 || end >= st.Size() { end = st.Size() - 1 }
      size := end - start + 1
      w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, st.Size()))
      w.Header().Set("Accept-Ranges", "bytes")
      w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
      w.WriteHeader(206)
      f, _ := os.Open(p)
      defer f.Close()
      f.Seek(start, 0)
      io.CopyN(w, f, size)
    } else {
      w.Header().Set("Content-Length", fmt.Sprintf("%d", st.Size()))
      http.ServeFile(w, r, p)
    }
  })
  mux.HandleFunc("/api/files/delete", func(w http.ResponseWriter, r *http.Request) {
    name := r.URL.Query().Get("name")
    p := filepath.Join(dataDir, name)
    if _, err := os.Stat(p); err != nil { w.WriteHeader(404); return }
    os.Remove(p)
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte("{\"ok\":true}"))
  })
  mux.HandleFunc("/api/files/rename", func(w http.ResponseWriter, r *http.Request) {
    var obj map[string]any
    json.NewDecoder(r.Body).Decode(&obj)
    from, _ := obj["from"].(string)
    to, _ := obj["to"].(string)
    if from == "" || to == "" { w.WriteHeader(400); w.Write([]byte("{\"error\":\"param\"}")); return }
    os.Rename(filepath.Join(dataDir, from), filepath.Join(dataDir, to))
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte("{\"ok\":true}"))
  })
  tlsSrv := &http.Server{ Addr: ":8443", Handler: mux, TLSConfig: &tls.Config{ Certificates: []tls.Certificate{ tlsCert } } }
  log.Println("Agent running on :8444")
  go func() { log.Println("Agent HTTPS on :8443") ; _ = tlsSrv.ListenAndServeTLS("", "") }()
  http.ListenAndServe(":8444", nil)
}
