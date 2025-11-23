package iataplaces

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Airport represents one row from ourairports.com/airports.csv.
type Airport struct {
	ID             int64
	Ident          string
	Type           string
	Name           string
	LatitudeDeg    float64
	LongitudeDeg   float64
	ElevationFt    *int64
	Continent      string
	CountryName    string
	IsoCountry     string
	RegionName     string
	IsoRegion      string
	LocalRegion    string
	Municipality   string
	Scheduled      bool
	GPSCode        string
	ICAOCode       string
	IATACode       string
	LocalCode      string
	HomeLink       string
	WikipediaLink  string
	Keywords       string
	Score          *int64
	LastUpdateTime *time.Time
}

// Store holds airports indexed for fast lookup.
type Store struct {
	byIATA map[string]*Airport
}

// LookupIATA on a Store (used by the default global store).
func (s *Store) LookupIATA(code string) (*Airport, bool) {
	if s == nil {
		return nil, false
	}
	if code == "" {
		return nil, false
	}
	upper := toUpperASCII(code)
	a, ok := s.byIATA[upper]
	return a, ok
}

// -------- Global default store & public API --------

var (
	defaultStore *Store
	loadOnce     sync.Once
	loadErr      error
)

// defaultCSVPath returns where we load from by default.
//
//  1. If AIRPORTS_CSV_PATH is set, use that.
//  2. Else, use "data/airports-latest.csv".
func defaultCSVPath() string {
	if p := os.Getenv("AIRPORTS_CSV_PATH"); p != "" {
		return p
	}
	return "data/airports-latest.csv"
}

// ensureDefaultStore lazily loads the CSV into memory once.
func ensureDefaultStore() (*Store, error) {
	loadOnce.Do(func() {
		path := defaultCSVPath()
		var err error
		defaultStore, err = LoadFromFile(path)
		if err != nil {
			loadErr = fmt.Errorf("iataplaces: failed to load CSV from %s: %w", path, err)
		}
	})
	return defaultStore, loadErr
}

// LookupIATA is the simple API you want.
// It lazily loads the airports CSV (once) and does an O(1) lookup.
func LookupIATA(code string) (*Airport, bool) {
	store, err := ensureDefaultStore()
	if err != nil {
		// Optionally log here if you like.
		return nil, false
	}
	return store.LookupIATA(code)
}

// -------- Loader helpers (used internally, but also handy for tests/tools) --------

// LoadFromFile loads airports from a CSV file on disk into memory.
func LoadFromFile(path string) (*Store, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open airports csv: %w", err)
	}
	defer f.Close()

	return LoadFromReader(f)
}

// LoadFromReader loads airports from any io.Reader.
func LoadFromReader(r io.Reader) (*Store, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // allow variable length lines

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	colIndex := make(map[string]int, len(header))
	for i, col := range header {
		colIndex[strings.TrimSpace(col)] = i
	}

	get := func(rec []string, col string) string {
		idx, ok := colIndex[col]
		if !ok || idx >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[idx])
	}

	// Preallocate with a sensible size. OurAirports has ~70k airports,
	// but only a subset has IATA codes.
	byIATA := make(map[string]*Airport, 80000)

	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read record: %w", err)
		}

		idStr := get(rec, "id")
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			// Skip bad rows rather than failing the whole load.
			continue
		}

		lat, _ := strconv.ParseFloat(get(rec, "latitude_deg"), 64)
		lon, _ := strconv.ParseFloat(get(rec, "longitude_deg"), 64)

		var elev *int64
		if ev := get(rec, "elevation_ft"); ev != "" {
			if v, err := strconv.ParseInt(ev, 10, 64); err == nil {
				elev = &v
			}
		}

		var score *int64
		if sc := get(rec, "score"); sc != "" {
			if v, err := strconv.ParseInt(sc, 10, 64); err == nil {
				score = &v
			}
		}

		var lastUpdated *time.Time
		if lu := get(rec, "last_updated"); lu != "" {
			if t, err := time.Parse(time.RFC3339, lu); err == nil {
				lastUpdated = &t
			}
		}

		sched := false
		if ss := strings.ToLower(get(rec, "scheduled_service")); ss == "1" || ss == "yes" || ss == "true" {
			sched = true
		}

		iata := strings.ToUpper(strings.TrimSpace(get(rec, "iata_code")))
		if iata == "" {
			// Many airports have no IATA; skip them for an IATA-focused index.
			continue
		}

		airport := &Airport{
			ID:             id,
			Ident:          get(rec, "ident"),
			Type:           get(rec, "type"),
			Name:           get(rec, "name"), // csv.Reader already unquotes
			LatitudeDeg:    lat,
			LongitudeDeg:   lon,
			ElevationFt:    elev,
			Continent:      get(rec, "continent"),
			CountryName:    get(rec, "country_name"),
			IsoCountry:     get(rec, "iso_country"),
			RegionName:     get(rec, "region_name"),
			IsoRegion:      get(rec, "iso_region"),
			LocalRegion:    get(rec, "local_region"),
			Municipality:   get(rec, "municipality"),
			Scheduled:      sched,
			GPSCode:        get(rec, "gps_code"),
			ICAOCode:       get(rec, "icao_code"),
			IATACode:       iata,
			LocalCode:      get(rec, "local_code"),
			HomeLink:       get(rec, "home_link"),
			WikipediaLink:  get(rec, "wikipedia_link"),
			Keywords:       get(rec, "keywords"),
			Score:          score,
			LastUpdateTime: lastUpdated,
		}

		// Only one entry per IATA – if duplicates exist, keep the first one.
		if _, exists := byIATA[iata]; !exists {
			byIATA[iata] = airport
		}
	}

	return &Store{
		byIATA: byIATA,
	}, nil
}

// toUpperASCII turns a short ASCII string into upper-case efficiently.
func toUpperASCII(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		b[i] = c
	}
	return string(b)
}
