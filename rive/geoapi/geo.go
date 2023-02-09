package geoapi

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
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

	token string
}

func New(token string) *Client {
	db, err := sql.Open("sqlite", "geo.db")
	if err != nil {
		log.Fatal(err)
	}
	db.Exec(`
		PRAGMA journal_mode = 'WAL';
		PRAGMA auto_vacuum = '1';
		BEGIN;
			CREATE TABLE IF NOT EXISTS "locations" ("name" TEXT NOT NULL,"latitude" REAL,"longitude" REAL, PRIMARY KEY("name"));
		COMMIT;
	`)
	return &Client{
		token: token,
		db:    db,
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

func (c *Client) GetLocationCache(location string) (GPS, error) {
	var gps GPS
	row := c.db.QueryRow(`SELECT latitude, longitude FROM locations WHERE name = ? COLLATE NOCASE;`, location)
	switch err := row.Scan(&gps.Latitude, &gps.Longitude); err {
	case sql.ErrNoRows:
		geo, err := c.GetLocation(location)
		if err != nil {
			return gps, err
		}
		for _, v := range geo.Features {
			if strings.ToLower(v.Text) == location {
				if len(v.Center) == 2 {
					gps = GPS{
						Latitude:  v.Center[1],
						Longitude: v.Center[0],
					}

					err = c.WriteNewGPS(v.Text, gps)
					if err != nil {
						return gps, nil
					}

					return gps, nil
				}
			}
		}
		return gps, fmt.Errorf("%+v", gps)
	case nil:
		return gps, nil
	default:
		panic(err)
	}
}

func (c *Client) GetLocation(location string) (Geo, error) {
	client := &http.Client{}

	host, err := url.Parse(fmt.Sprintf("https://api.mapbox.com/geocoding/v5/mapbox.places/%s.json", html.EscapeString(location)))
	if err != nil {
		return Geo{}, err
	}
	values := host.Query()
	values.Set("access_token", c.token)
	values.Set("types", "region,place,postcode")
	values.Set("autocomplete", "false")
	host.RawQuery = values.Encode()

	req, err := http.NewRequest(http.MethodGet, host.String(), nil)
	if err != nil {
		return Geo{}, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	if err != nil {
		return Geo{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return Geo{}, err
	}

	var response Geo
	err = json.Unmarshal(body, &response)
	if err != nil {
		return Geo{}, err
	}

	return response, err
}
