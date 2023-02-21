package geoapi

import (
	"database/sql"
	"fmt"

	"log"
	"sync"

	_ "modernc.org/sqlite"
)

type Geo struct {
	Type     string   `json:"type"`
	Query    []string `json:"query"`
	Features []struct {
		ID         string   `json:"id"`
		Type       string   `json:"type"`
		PlaceType  []string `json:"place_type"`
		Relevance  float64  `json:"relevance"`
		Properties struct {
			Wikidata   string `json:"wikidata,omitempty"`
			Category   string `json:"category,omitempty"`
			Accuracy   string `json:"accuracy,omitempty"`
			Landmark   bool   `json:"landmark,omitempty"`
			Address    string `json:"address,omitempty"`
			Foursquare string `json:"foursquare,omitempty"`
		} `json:"properties,omitempty"`
		Text      string    `json:"text"`
		PlaceName string    `json:"place_name"`
		Bbox      []float64 `json:"bbox,omitempty"`
		Center    []float64 `json:"center"`
		Geometry  struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Context []struct {
			ID        string `json:"id"`
			ShortCode string `json:"short_code"`
			Wikidata  string `json:"wikidata"`
			Text      string `json:"text"`
		} `json:"context"`
		MatchingText      string `json:"matching_text,omitempty"`
		MatchingPlaceName string `json:"matching_place_name,omitempty"`
	} `json:"features"`
	Attribution string `json:"attribution"`
}

type Client struct {
	db   *sql.DB
	lock sync.Mutex
}

func New() *Client {
	db, err := sql.Open("sqlite", "geo.db")
	if err != nil {
		log.Fatal(err)
	}
	db.Exec(`
		PRAGMA journal_mode = 'WAL';
		PRAGMA auto_vacuum = '1';
		BEGIN;
			CREATE TABLE IF NOT EXISTS "locations" ("name" TEXT NOT NULL, "zip" TEXT NOT NULL ,"latitude" REAL,"longitude" REAL, PRIMARY KEY("name"));
		COMMIT;
	`)
	return &Client{
		db: db,
	}
}

func (c *Client) Close() {
	c.db.Close()
}

type GPS struct {
	Latitude  float64
	Longitude float64
}

func (c *Client) WriteNewGPS(name string, location GPS) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	stmt, err := c.db.Prepare(`INSERT OR IGNORE INTO locations (name, latitude, longitude)VALUES(?,?,?);`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(name, location.Latitude, location.Longitude)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetLocation(location string) (GPS, error) {
	var gps GPS
	row := c.db.QueryRow(`SELECT latitude, longitude FROM locations WHERE name = ? COLLATE NOCASE;`, location)
	switch err := row.Scan(&gps.Latitude, &gps.Longitude); err {
	case sql.ErrNoRows:
		return gps, fmt.Errorf("no location for %+v", gps)
	case nil:
		return gps, nil
	default:
		panic(err)
	}
}
