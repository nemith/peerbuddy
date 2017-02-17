package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const defaultBaseURL = "https://peeringdb.org/api"

var httpClient *http.Client
var baseURL *url.URL

var apiObjs = []Object{
	//"fac",
	&IX{},
	//	"ixfac",
	//	"ixlan",
	//	"ixpfx",
	&Network{},
	//	"netfac",
	&NetworkIXLan{},
	//	"org",
	//	"poc",
}

func init() {
	tr := http.DefaultTransport.(*http.Transport)
	// HACK: Real cert checking would be nice
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	httpClient = &http.Client{Transport: tr}
	var err error
	baseURL, err = url.Parse(defaultBaseURL)
	if err != nil {
		panic(err)
	}
}

type UnixTime struct {
	time.Time
}

func (t *UnixTime) UnmarshalJSON(data []byte) error {
	parts := bytes.Split(data, []byte{'.'})
	if len(parts) != 2 {
		return fmt.Errorf("Malformed time entry: %s", data)
	}

	sec, err := strconv.ParseInt(string(parts[0]), 10, 64)
	if err != nil {
		return err
	}

	nsec, err := strconv.ParseInt(string(parts[1]), 10, 64)
	if err != nil {
		return err
	}

	t.Time = time.Unix(sec, nsec)
	return nil
}

type Response struct {
	Meta struct {
		Error     string   `json:"error"`
		Generated UnixTime `json:"generated"`
	} `json:"meta"`
	Data json.RawMessage `json:"data"`
}

type Object interface {
	GetID() int
	Deleted() bool
	SQLName() string
	PDBName() string
	UpdateDB(tx *sqlx.Tx, data []byte) error
}

type ObjBase struct {
	ID      int       `json:"id" db:"id"`
	Created time.Time `json:"created" db:"created"`
	Updated time.Time `json:"updated" db:"updated"`
	Status  string    `json:"status"`
}

func (o *ObjBase) GetID() int {
	return o.ID
}

func (o *ObjBase) Deleted() bool {
	return o.Status == "deleted"
}

type IX struct {
	ObjBase
	OrgID    int    `json:"org_id" db:"org_id"`
	Name     string `json:"name" db:"name"`
	NameLong string `json:"name_long" db:"name_long"`
	City     string `json:"city" db:"city"`
	Country  string `json:"country" db:"country"`
	// "region_continent": "choice",
	// "media": "choice",
	// "notes": "",
	// "proto_unicast": false,
	// "proto_multicast": false,
	// "proto_ipv6": false,
	// "website": "",
	// "url_stats": "",
	// "tech_email": "",
	// "tech_phone": "",
	// "policy_email": "",
	// "policy_phone": "",
}

func (o *IX) PDBName() string {
	return "ix"
}

func (o *IX) SQLName() string {
	return "ix"
}

func (o *IX) UpdateDB(tx *sqlx.Tx, data []byte) error {
	objs := []IX{}
	if err := json.Unmarshal(data, &objs); err != nil {
		return err
	}

	for _, o := range objs {
		if err := updateDB(tx, &o); err != nil {
			return err
		}
	}
	return nil
}

type IXLan struct {
	ObjBase
	IXID int `json:"IXID",db:"IXID"`
	//    "name": "",
	//    "descr": "",
	//    "mtu": 0,
	//    "dot1q_support": false,
	//    "rs_asn": 0,
	//    "arp_sponge": "",
}

type Network struct {
	ObjBase
	OrgID   int    `json:"org_id" db:"org_id"`
	ASN     int    `json:"asn" db:"asn"`
	Name    string `json:"name" db:"name"`
	AKA     string `json:"aka" db:"aka"`
	Website string `json:"website" db:"website"`
	//  "looking_glass": "",
	//  "route_server": "",
	//  "irr_as_set": "",
	//  "info_type": "choice",
	//  "info_prefixes4": 0,
	//  "info_prefixes6": 0,
	//  "info_traffic": "choice",
	//  "info_ratio": "choice",
	//  "info_scope": "choice",
	//  "info_unicast": false,
	//  "info_multicast": false,
	//  "info_ipv6": false,
	//  "notes": "",
	//  "policy_url": "",
	//  "policy_general": "choice",
	//  "policy_locations": "choice",
	//  "policy_ratio": false,
	//  "policy_contracts": "choice",
	//}
}

func (o *Network) PDBName() string {
	return "net"
}

func (o *Network) SQLName() string {
	return "networks"
}

func (o *Network) UpdateDB(tx *sqlx.Tx, data []byte) error {
	objs := []Network{}
	if err := json.Unmarshal(data, &objs); err != nil {
		return err
	}

	for _, o := range objs {
		if err := updateDB(tx, &o); err != nil {
			return err
		}
	}
	return nil
}

// NetworkIXLan represent a Netwok's public peering exchange points
type NetworkIXLan struct {
	ObjBase
	NetID    int    `json:"net_id" db:"net_id"`
	IXID     int    `json:"ix_id" db:"ix_id"`
	IXLanID  int    `json:"ixlan_id" db:"ixlan_id"`
	Notes    string `json:"notes" db:"notes"`
	Speed    int    `json:"speed" db:"speed"`
	ASN      int    `json:"asn" db:"asn"`
	IPv4Addr string `json:"ipaddr4" db:"ipaddr4"`
	IPv6Addr string `json:"ipaddr6" db:"ipaddr6"`
	IsRSPeer bool   `json:"is_rs_peer" db:"is_rs_peer"`
}

func (o *NetworkIXLan) PDBName() string {
	return "netixlan"
}

func (o *NetworkIXLan) SQLName() string {
	return "network_ix_lans"
}

func (o *NetworkIXLan) UpdateDB(tx *sqlx.Tx, data []byte) error {
	objs := []NetworkIXLan{}
	if err := json.Unmarshal(data, &objs); err != nil {
		return err
	}

	for _, o := range objs {
		if err := updateDB(tx, &o); err != nil {
			return err
		}
	}
	return nil
}

func updateDB(tx *sqlx.Tx, obj Object) error {
	tableName := obj.SQLName()

	if obj.Deleted() {
		_, err := tx.Exec(fmt.Sprintf("DELETE FROM `%s` WHERE `id` = ?", tableName), obj.GetID())
		if err != nil {
			return err
		}
		return nil
	}

	stmtString := insertStmt(tableName, obj)
	stmt, err := tx.PrepareNamed(stmtString)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(obj)
	return err
}

// This is pretty naive and probably can break in spectacular ways, but I wanted
// to play a bit with reflections since they are voodoo magic.
func insertStmt(tableName string, v interface{}) string {
	val := reflect.ValueOf(v).Elem()

	cols := make([]string, 0, val.NumField())
	values := make([]string, 0, val.NumField())

	for i := 0; i < val.NumField(); i++ {
		typeField := val.Type().Field(i)
		tag := typeField.Tag.Get("db")

		if tag != "" {
			cols = append(cols, tag)
			values = append(values, fmt.Sprintf(":%s", tag))
		}
	}
	return fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
		tableName,
		strings.Join(cols, ", "),
		strings.Join(values, ", "))
}

func fetchObjs(obj string, since time.Time) (*Response, error) {
	u := *baseURL
	u.Path = path.Join(u.Path, obj)

	if !since.IsZero() {
		q := url.Values{}
		q.Add("since", strconv.FormatInt(since.Unix(), 10))
		u.RawQuery = q.Encode()
	}

	log.Printf("Fetching %s", u.String())
	r, err := httpClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("Failed request for '%v' got code '%d'", u, r.StatusCode)
	}

	d := json.NewDecoder(r.Body)
	resp := &Response{}
	err = d.Decode(resp)
	return resp, err
}

type IDSet map[int]struct{}

func main() {
	// Fetch last update form the database so that we only receive changes
	lastFetch := time.Time{}

	db, err := sqlx.Connect("sqlite3", "/tmp/pdb.db")
	if err != nil {
		log.Fatalf("Cannot connect to db: %v", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		panic(err)
	}
	defer tx.Commit()

	for _, obj := range apiObjs {
		resp, err := fetchObjs(obj.PDBName(), lastFetch)
		if err != nil {
			log.Fatalf("Couldn't fetch objects for api object '%s': %v", obj.PDBName(), err)
		}

		log.Printf("Updating database table '%s'", obj.SQLName())
		err = obj.UpdateDB(tx, resp.Data)
		if err != nil {
			panic(err)
		}
	}

	//		objs := []PDBObj{}
	//		if err := json.Unmarshal(resp.Data, &objs); err != nil {
	//			panic(err)
	//		}
	//
	//		deletedIDs := make(IDSet)
	//		for _, o := range objs {
	//			if o.Status == "deleted" {
	//				deletedIDs[o.ID] = struct{}{}
	//			}
	//		}
	//
	//		log.Printf("Changes for object '%s' between '%s' and '%s' Added: %d, Removed: %d",
	//			objName, lastFetch.Format(time.RFC822), time.Now().Format(time.RFC822),
	//			len(objs)-len(deletedIDs), len(deletedIDs))
	//
	//		switch objName {
	//		case "netixlan":
	//			netIXLans := []NetworkIXLan{}
	//			if err := json.Unmarshal(resp.Data, &netIXLans); err != nil {
	//				log.Printf("Couldn't parse netixlan json data: %v", err)
	//			}
	//
	//		default:
	//			log.Printf("Unknown peeringdb api object '%s'", objName)
	//		}
	//	}
}
