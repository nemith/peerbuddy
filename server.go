package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var (
	httpAddr = flag.String("addr", ":8080", "HTTP address to listen on")
)

var (
	db        *sqlx.DB
	templates map[string]*template.Template
)

func init() {
	var err error
	db, err = sqlx.Connect("sqlite3", "/tmp/pdb.db")
	if err != nil {
		log.Fatalf("Cannot connect to db: %v", err)
	}
}

func init() {
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
	Speed    int
	IPv4Addr string
	IPv6Addr string
}

func ixReportHandler(w http.ResponseWriter, r *http.Request) {

	rows, err := db.Queryx("SELECT ix.name, net.speed, net.ipaddr4, net.ipaddr6 FROM network_ix_lans as net JOIN ix WHERE net.ix_id = ix.id AND net.asn = 46489")
	if err != nil {
		panic(err)
	}

	ixMap := make(map[string][]netSummary)

	for rows.Next() {
		var ixName string
		net := netSummary{}
		if err := rows.Scan(&ixName, &net.Speed, &net.IPv4Addr, &net.IPv6Addr); err != nil {
			panic(err)
		}

		_, ok := ixMap[ixName]
		if !ok {
			ixMap[ixName] = append([]netSummary{}, net)
		} else {
			ixMap[ixName] = append(ixMap[ixName], net)
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

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/report/ix", ixReportHandler)
	http.HandleFunc("/", indexHandler)
	log.Printf("HTTP server listening on %s", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}
