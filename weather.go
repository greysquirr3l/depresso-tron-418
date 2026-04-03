package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Holt, Michigan — the only weather that matters for digital milk stability.
const (
	holtLat = 42.6286
	holtLon = -84.5211
)

// WeatherData is a snapshot of conditions in Holt, MI.
type WeatherData struct {
	Temperature   float64 // °F
	WindSpeed     float64 // km/h
	Precipitation float64 // mm
}

type openMeteoResponse struct {
	Current struct {
		Temperature2M float64 `json:"temperature_2m"`
		WindSpeed10M  float64 `json:"wind_speed_10m"`
		Precipitation float64 `json:"precipitation"`
	} `json:"current"`
}

var (
	weatherMu        sync.Mutex
	cachedWeather    *WeatherData
	weatherCacheTime time.Time
)

// fetchHoltWeather retrieves current conditions from Open-Meteo (no API key required).
// Results are cached for 10 minutes to avoid becoming a denial-of-service tool
// against the only free weather API that will still talk to us.
func fetchHoltWeather(ctx context.Context) (*WeatherData, error) {
	weatherMu.Lock()
	defer weatherMu.Unlock()

	if cachedWeather != nil && time.Since(weatherCacheTime) < 10*time.Minute {
		return cachedWeather, nil
	}

	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
			"&current=temperature_2m,wind_speed_10m,precipitation&temperature_unit=fahrenheit",
		holtLat, holtLon,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch weather: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var data openMeteoResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("parse weather: %w", err)
	}

	w := &WeatherData{
		Temperature:   data.Current.Temperature2M,
		WindSpeed:     data.Current.WindSpeed10M,
		Precipitation: data.Current.Precipitation,
	}
	cachedWeather = w
	weatherCacheTime = time.Now()
	return w, nil
}

// isMilkSpoiled returns true when the digital milk thermodynamic threshold
// has been exceeded. Above 70°F, no responsible HTCPCP server will risk it.
func isMilkSpoiled(temp float64) bool { return temp > 70.0 }

// isTooHumidForExtraction returns true when precipitation would compromise
// the digital grounds (soggy extraction produces inferior virtual espresso).
func isTooHumidForExtraction(precip float64) bool { return precip > 0 }

// whenWindowSecond derives the WHEN challenge window from current wind speed.
// Per our proprietary RFC 9999 extension, the optimal milk-stop moment is
// wind_speed_km_h mod 60 seconds into any given minute.
func whenWindowSecond(windSpeed float64) int {
	return int(windSpeed) % 60
}
