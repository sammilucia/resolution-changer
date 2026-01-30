package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/sammilucia/resolution-changer/displayManager"
)

type ResolutionConfig struct {
	Resolution displayManager.Resolution
	Hotkey     string
}

type RefreshRateConfig struct {
	Rate   displayManager.RefreshRate
	Hotkey string
}

type AppConfig struct {
	Resolutions  []ResolutionConfig
	RefreshRates []RefreshRateConfig
}

func loadConfig(path string) (AppConfig, error) {
	var cfg AppConfig

	f, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		// strip inline comments
		if idx := strings.Index(line, ";"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		switch currentSection {
		case "Resolutions":
			// format: 2560x1600 = Ctrl+F1
			value, hotkey := splitKeyValue(line)
			parts := strings.Split(value, "x")
			if len(parts) != 2 {
				slog.Warn("invalid resolution line", "line", line)
				continue
			}
			w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 != nil || err2 != nil {
				slog.Warn("invalid resolution values", "line", line)
				continue
			}
			cfg.Resolutions = append(cfg.Resolutions, ResolutionConfig{
				Resolution: displayManager.Resolution{
					Width:  uint32(w),
					Height: uint32(h),
				},
				Hotkey: hotkey,
			})

		case "RefreshRates":
			// format: 240 = Alt+F1
			value, hotkey := splitKeyValue(line)
			hz, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				slog.Warn("invalid refresh rate", "line", line)
				continue
			}
			cfg.RefreshRates = append(cfg.RefreshRates, RefreshRateConfig{
				Rate:   displayManager.RefreshRate(hz),
				Hotkey: hotkey,
			})
		default:
			// ignore other sections
		}
	}

	if err := scanner.Err(); err != nil {
		return cfg, fmt.Errorf("scan config: %w", err)
	}

	return cfg, nil
}

// splitKeyValue splits "2560x1600 = Ctrl+F1" into ("2560x1600", "Ctrl+F1")
// If no "=" is present, returns (line, "")
func splitKeyValue(line string) (value, hotkey string) {
	if idx := strings.Index(line, "="); idx != -1 {
		return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:])
	}
	return line, ""
}
