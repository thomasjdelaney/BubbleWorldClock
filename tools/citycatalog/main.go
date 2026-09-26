package main

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type cityRecord struct {
	GeoNameID   int64  `json:"geoname_id"`
	Name        string `json:"name"`
	ASCIIName   string `json:"ascii_name"`
	CountryCode string `json:"country_code"`
	Country     string `json:"country"`
	Admin1Code  string `json:"admin1_code"`
	Region      string `json:"region"`
	Timezone    string `json:"timezone"`
	Population  int64  `json:"population"`
}

func main() {
	citiesPath := flag.String("cities", "cities15000.zip", "GeoNames cities15000.zip file")
	countriesPath := flag.String("countries", "countryInfo.txt", "GeoNames countryInfo.txt file")
	admin1Path := flag.String("admin1", "admin1CodesASCII.txt", "GeoNames admin1CodesASCII.txt file")
	outputPath := flag.String("output", "data/city_catalog.json", "output catalog JSON path")
	flag.Parse()

	if err := generate(*citiesPath, *countriesPath, *admin1Path, *outputPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(citiesPath, countriesPath, admin1Path, outputPath string) error {
	countries, err := loadNames(countriesPath, '#', 0, 4, "")
	if err != nil {
		return fmt.Errorf("load countries: %w", err)
	}
	admin1, err := loadNames(admin1Path, 0, 0, 2, ".")
	if err != nil {
		return fmt.Errorf("load admin regions: %w", err)
	}

	archive, err := zip.OpenReader(citiesPath)
	if err != nil {
		return fmt.Errorf("open cities archive: %w", err)
	}
	defer archive.Close()

	var source io.ReadCloser
	for _, file := range archive.File {
		if filepath.Base(file.Name) == "cities15000.txt" {
			source, err = file.Open()
			if err != nil {
				return fmt.Errorf("open city data: %w", err)
			}
			break
		}
	}
	if source == nil {
		return errors.New("archive does not contain cities15000.txt")
	}
	defer source.Close()

	reader := csv.NewReader(source)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true

	var records []cityRecord
	invalidZones := 0
	for {
		row, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read city data: %w", readErr)
		}
		if len(row) <= 17 {
			continue
		}

		id, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil || strings.TrimSpace(row[1]) == "" || strings.TrimSpace(row[2]) == "" {
			continue
		}
		countryCode := strings.TrimSpace(row[8])
		countryName := countries[countryCode]
		if countryName == "" {
			continue
		}
		timezone := strings.TrimSpace(row[17])
		if _, err := time.LoadLocation(timezone); err != nil {
			invalidZones++
			continue
		}

		admin1Code := strings.TrimSpace(row[10])
		regionName := admin1[countryCode+"."+admin1Code]
		population, _ := strconv.ParseInt(row[14], 10, 64)
		records = append(records, cityRecord{
			GeoNameID:   id,
			Name:        strings.TrimSpace(row[1]),
			ASCIIName:   strings.TrimSpace(row[2]),
			CountryCode: countryCode,
			Country:     countryName,
			Admin1Code:  admin1Code,
			Region:      regionName,
			Timezone:    timezone,
			Population:  population,
		})
	}
	if len(records) == 0 {
		return errors.New("no usable cities found in source archive")
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Country != records[j].Country {
			return records[i].Country < records[j].Country
		}
		if records[i].Name != records[j].Name {
			return records[i].Name < records[j].Name
		}
		return records[i].GeoNameID < records[j].GeoNameID
	})

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	output, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output catalog: %w", err)
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(records); err != nil {
		output.Close()
		return fmt.Errorf("encode catalog: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close output catalog: %w", err)
	}
	fmt.Printf("wrote %d cities; skipped %d with invalid time zones\n", len(records), invalidZones)
	return nil
}

func loadNames(filename string, comment rune, keyColumn, nameColumn int, separator string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.Comment = comment
	reader.FieldsPerRecord = -1
	result := make(map[string]string)
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(row) <= keyColumn || len(row) <= nameColumn {
			continue
		}
		key := strings.TrimSpace(row[keyColumn])
		if separator != "" {
			key = strings.Replace(key, separator, ".", 1)
		}
		name := strings.TrimSpace(row[nameColumn])
		if key != "" && name != "" {
			result[key] = name
		}
	}
	return result, nil
}
