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
	appVersion = "v0.4"
)

//go:embed assets/icon_ico.ico
var trayIcon []byte

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

func onReady() {
	slog.Info("onReady")

	systray.SetIcon(trayIcon) // use embedded icon bytes
	systray.SetTitle(appName)
	systray.SetTooltip(appName + " " + appVersion)

	cfg, err := loadConfig("config.ini")
	if err != nil {
		slog.Warn("using built-in defaults; failed to load config.ini", "err", err)
		cfg = AppConfig{
			Resolutions: []displayManager.Resolution{
				{Width: 2560, Height: 1600},
				{Width: 2560, Height: 1440},
			},
			RefreshRates: []displayManager.RefreshRate{
				240,
				60,
			},
		}
	}

	// build resolution menu
	resMenus = nil
	for _, r := range cfg.Resolutions {
		label := fmt.Sprintf("%dx%d", r.Width, r.Height)
		item := systray.AddMenuItem(label, label)
		resMenus = append(resMenus, resMenu{
			item: item,
			res:  r,
		})
	}

	systray.AddSeparator()

	// build refresh rate menu
	rateMenus = nil
	for _, hz := range cfg.RefreshRates {
		label := fmt.Sprintf("%dhz", hz)
		item := systray.AddMenuItem(label, label)
		rateMenus = append(rateMenus, rateMenu{
			item: item,
			rate: hz,
		})
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
