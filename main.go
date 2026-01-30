// this code was written on Christmas day because I have no life 😀

package main

import (
	_ "embed"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sammilucia/resolution-changer/displayManager"
	"github.com/getlantern/systray"
)

const (
	appName    = "Resolution Changer"
	appVersion = "v0.5"
)

//go:embed assets/icon_ico.ico
var trayIcon []byte

// hotkey ID ranges
const (
	hotkeyResBase  = 1000 // resolution hotkeys: 1000, 1001, ...
	hotkeyRateBase = 2000 // refresh rate hotkeys: 2000, 2001, ...
)

type resMenu struct {
	item *systray.MenuItem
	res  displayManager.Resolution
}

type rateMenu struct {
	item *systray.MenuItem
	rate displayManager.RefreshRate
}

var (
	resMenus  []resMenu
	rateMenus []rateMenu

	currentRes  displayManager.Resolution
	currentRate displayManager.RefreshRate

	stateMu sync.Mutex
)

func applyDisplayInfo(di displayManager.DisplayInfo) {
	stateMu.Lock()
	defer stateMu.Unlock()

	for _, rm := range resMenus {
		if rm.res.Width == di.Resolution.Width && rm.res.Height == di.Resolution.Height {
			rm.item.Check()
		} else {
			rm.item.Uncheck()
		}
	}

	for _, hm := range rateMenus {
		if hm.rate == di.Refresh {
			hm.item.Check()
		} else {
			hm.item.Uncheck()
		}
	}

	currentRes = di.Resolution
	currentRate = di.Refresh
}

// gcd calculates the greatest common divisor using Euclidean algorithm
func gcd(a, b uint32) uint32 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// aspectRatio returns the aspect ratio as a string like "16:10"
func aspectRatio(w, h uint32) string {
	d := gcd(w, h)
	return fmt.Sprintf("%d:%d", w/d, h/d)
}

func onReady() {
	slog.Info("onReady")

	systray.SetIcon(trayIcon) // use embedded icon bytes
	systray.SetTitle(appName)
	systray.SetTooltip(appName + " " + appVersion)

	cfg, err := loadConfig("config.ini")
	if err != nil {
		slog.Warn("using built-in defaults; failed to load config.ini", "err", err)
		cfg = AppConfig{
			Resolutions: []ResolutionConfig{
				{Resolution: displayManager.Resolution{Width: 2560, Height: 1600}},
				{Resolution: displayManager.Resolution{Width: 2560, Height: 1440}},
			},
			RefreshRates: []RefreshRateConfig{
				{Rate: 240},
				{Rate: 60},
			},
		}
	}

	// build resolution menu and register hotkeys
	resMenus = nil
	for i, r := range cfg.Resolutions {
		label := fmt.Sprintf("%dx%d", r.Resolution.Width, r.Resolution.Height)
		if r.Hotkey != "" {
			label += fmt.Sprintf("   (%s)", r.Hotkey)
		}
		item := systray.AddMenuItem(label, label)
		resMenus = append(resMenus, resMenu{
			item: item,
			res:  r.Resolution,
		})

		// register hotkey if configured
		if r.Hotkey != "" {
			mods, key, err := ParseHotkey(r.Hotkey)
			if err != nil {
				slog.Warn("invalid hotkey", "resolution", label, "hotkey", r.Hotkey, "err", err)
			} else if key != 0 {
				if err := RegisterHotkey(hotkeyResBase+i, mods, key); err != nil {
					slog.Warn("failed to register hotkey", "resolution", label, "hotkey", r.Hotkey, "err", err)
				} else {
					slog.Info("registered hotkey", "resolution", label, "hotkey", r.Hotkey)
				}
			}
		}
	}

	systray.AddSeparator()

	// build refresh rate menu and register hotkeys
	rateMenus = nil
	for i, hz := range cfg.RefreshRates {
		label := fmt.Sprintf("%dhz", hz.Rate)
		if hz.Hotkey != "" {
			label += fmt.Sprintf("   (%s)", hz.Hotkey)
		}
		item := systray.AddMenuItem(label, label)
		rateMenus = append(rateMenus, rateMenu{
			item: item,
			rate: hz.Rate,
		})

		// register hotkey if configured
		if hz.Hotkey != "" {
			mods, key, err := ParseHotkey(hz.Hotkey)
			if err != nil {
				slog.Warn("invalid hotkey", "rate", label, "hotkey", hz.Hotkey, "err", err)
			} else if key != 0 {
				if err := RegisterHotkey(hotkeyRateBase+i, mods, key); err != nil {
					slog.Warn("failed to register hotkey", "rate", label, "hotkey", hz.Hotkey, "err", err)
				} else {
					slog.Info("registered hotkey", "rate", label, "hotkey", hz.Hotkey)
				}
			}
		}
	}

	systray.AddSeparator()
	quit := systray.AddMenuItem("Exit", "exit")

	// initial state: mark current res / Hz
	if di, err := displayManager.GetCurrentDisplay(); err != nil {
		slog.Error("GetCurrentDisplay failed", "err", err)
	} else {
		applyDisplayInfo(di)
	}

	// resolution handlers
	for _, rm := range resMenus {
		rm := rm
		go func() {
			for range rm.item.ClickedCh {
				if err := displayManager.ChangeResolution(rm.res); err != nil {
					slog.Error("failed to change resolution", "err", err)
					continue
				}
				if di, err := displayManager.GetCurrentDisplay(); err == nil {
					applyDisplayInfo(di)
				}
			}
		}()
	}

	// refresh rate handlers
	for _, hm := range rateMenus {
		hm := hm
		go func() {
			for range hm.item.ClickedCh {
				if err := displayManager.ChangeRefreshRate(hm.rate); err != nil {
					slog.Error("failed to change refresh rate", "err", err)
					continue
				}
				if di, err := displayManager.GetCurrentDisplay(); err == nil {
					applyDisplayInfo(di)
				}
			}
		}()
	}

	// quit handler
	go func() {
		for range quit.ClickedCh {
			systray.Quit()
		}
	}()

	// hotkey listener
	go HotkeyListener(func(id int) {
		if id >= hotkeyResBase && id < hotkeyRateBase {
			idx := id - hotkeyResBase
			if idx < len(resMenus) {
				if err := displayManager.ChangeResolution(resMenus[idx].res); err != nil {
					slog.Error("hotkey: failed to change resolution", "err", err)
				} else if di, err := displayManager.GetCurrentDisplay(); err == nil {
					applyDisplayInfo(di)
				}
			}
		} else if id >= hotkeyRateBase {
			idx := id - hotkeyRateBase
			if idx < len(rateMenus) {
				if err := displayManager.ChangeRefreshRate(rateMenus[idx].rate); err != nil {
					slog.Error("hotkey: failed to change refresh rate", "err", err)
				} else if di, err := displayManager.GetCurrentDisplay(); err == nil {
					applyDisplayInfo(di)
				}
			}
		}
	})

	// "listener": poll for external changes every few seconds
	go func() {
		for {
			time.Sleep(2 * time.Second)

			di, err := displayManager.GetCurrentDisplay()
			if err != nil {
				continue
			}

			stateMu.Lock()
			same := di.Resolution.Width == currentRes.Width &&
				di.Resolution.Height == currentRes.Height &&
				di.Refresh == currentRate
			stateMu.Unlock()

			if !same {
				applyDisplayInfo(di)
			}
		}
	}()
}

func onExit() {
	slog.Info("onExit")
}

func main() {
	systray.Run(onReady, onExit)
}
