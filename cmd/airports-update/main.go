package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const defaultAirportsURL = "https://ourairports.com/airports.csv"

func main() {
	outDir := flag.String("out", "data", "output directory for airports CSV files")
	url := flag.String("url", defaultAirportsURL, "OurAirports CSV URL")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("failed to create output dir %s: %v", *outDir, err)
	}

	ts := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("airports-%s.csv", ts)
	fullPath := filepath.Join(*outDir, filename)
	latestPath := filepath.Join(*outDir, "airports-latest.csv")

	log.Printf("Downloading airports data from %s", *url)

	resp, err := http.Get(*url)
	if err != nil {
		log.Fatalf("failed to download airports CSV: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("unexpected status code %d from %s", resp.StatusCode, *url)
	}

	tempPath := fullPath + ".tmp"
	outFile, err := os.Create(tempPath)
	if err != nil {
		log.Fatalf("failed to create temp file %s: %v", tempPath, err)
	}

	n, err := io.Copy(outFile, resp.Body)
	closeErr := outFile.Close()
	if err != nil {
		log.Fatalf("failed to write CSV to %s: %v", tempPath, err)
	}
	if closeErr != nil {
		log.Fatalf("failed to close temp file %s: %v", tempPath, closeErr)
	}

	if err := os.Rename(tempPath, fullPath); err != nil {
		log.Fatalf("failed to move temp file to final path: %v", err)
	}

	log.Printf("Saved airports CSV to %s (%d bytes)", fullPath, n)

	// Also keep a stable "airports-latest.csv" for your scripts.
	if err := copyFile(fullPath, latestPath); err != nil {
		log.Fatalf("failed to update %s: %v", latestPath, err)
	}
	log.Printf("Updated %s", latestPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("close dst: %w", err)
	}

	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}
