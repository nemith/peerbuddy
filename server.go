package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var (
	httpAddr = flag.String("addr", ":8080", "HTTP address to listen on")
)

var (
	db        *sqlx.DB
	templates map[string]*template.Template

	nameCacheLock sync.RWMutex
	nameCache     map[string]string
)

func init() {
	var err error
	db, err = sqlx.Connect("sqlite3", "/tmp/pdb.db")
	if err != nil {
		log.Fatalf("Cannot connect to db: %v", err)
	}

	// Init nameCache
	lookupAddrs()

	if templates == nil {
		templates = make(map[string]*template.Template)
	}

	layouts, err := filepath.Glob(filepath.Join("./templates", "*.tmpl"))
	if err != nil {
		log.Fatal(err)
	}

	includes, err := filepath.Glob(filepath.Join("./templates", "*.tmplinc"))
	if err != nil {
		log.Fatal(err)
	}

	for _, layout := range layouts {
		files := append(includes, layout)
		tmpl, err := template.ParseFiles(files...)
		if err != nil {
			log.Fatal(fmt.Errorf("Couldn't parse templates '%s': %v", strings.Join(files, ","), err))
		}
		templates[filepath.Base(layout)] = tmpl
	}
}

func renderTemplate(w http.ResponseWriter, name string, v interface{}) {
	// Ensure the template exists in the map.
	tmpl, ok := templates[name]
	if !ok {
		panic(fmt.Sprintf("The template %s does not exist.", name))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", v); err != nil {
		panic(err)
	}

}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index.tmpl", nil)
}

type netSummary struct {
	Name     string
	IPv4Addr string
	IPv6Addr string
	Speed    int
}

func lookupAddrs() error {
	if nameCache == nil {
		nameCache = make(map[string]string)
	}

	rows, err := db.Queryx("SELECT net.ipaddr4 FROM network_ix_lans as net WHERE net.asn = 46489")
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for rows.Next() {
		var addr string
		err := rows.Scan(&addr)
		if err != nil {
			return err
		}
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			log.Printf("Looking up name for %s", addr)
			names, err := net.LookupAddr(addr)
			if err == nil && len(names) > 0 {
				nameCacheLock.Lock()
				nameCache[addr] = names[0]
				nameCacheLock.Unlock()
			}
		}(addr)
	}
	wg.Wait()
	return nil
}

func ixReportHandler(w http.ResponseWriter, r *http.Request) {

	rows, err := db.Queryx("SELECT ix.name, net.speed, net.ipaddr4, net.ipaddr6 FROM network_ix_lans as net JOIN ix WHERE net.ix_id = ix.id AND net.asn = 46489")
	if err != nil {
		panic(err)
	}

	ixMap := make(map[string][]netSummary)

	for rows.Next() {
		var ixName string
		netSum := netSummary{
			Name: "(unknown)",
		}
		if err := rows.Scan(&ixName, &netSum.Speed, &netSum.IPv4Addr, &netSum.IPv6Addr); err != nil {
			panic(err)
		}

		nameCacheLock.RLock()
		if name, ok := nameCache[netSum.IPv4Addr]; ok {
			netSum.Name = name
		}
		nameCacheLock.RUnlock()

		_, ok := ixMap[ixName]
		if !ok {
			ixMap[ixName] = append([]netSummary{}, netSum)
		} else {
			ixMap[ixName] = append(ixMap[ixName], netSum)
		}
	}
	if rows.Err() != nil {
		panic(err)
	}

	type Summary struct {
		Speed   int
		NumNets int
		NumOrgs int
	}
	summary := Summary{}
	err = db.Get(&summary, "SELECT SUM(net.speed) as speed, COUNT(net.id) as numnets, COUNT(DISTINCT ix_id) as numorgs from network_ix_lans as net WHERE net.asn = 46489")
	if err != nil {
		panic(err)
	}

	ctx := struct {
		IXMap   map[string][]netSummary
		Summary Summary
	}{
		IXMap:   ixMap,
		Summary: summary,
	}
	renderTemplate(w, "ixreport.tmpl", ctx)
}

func main() {
	flag.Parse()

	// Update cache ever 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			if err := lookupAddrs(); err != nil {
				log.Printf("Cannot update name cachce: %v", err)
			}
		}
	}()

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/report/ix", ixReportHandler)
	http.HandleFunc("/", indexHandler)
	log.Printf("HTTP server listening on %s", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}
