package main

import (
	"fmt"
	"log/slog"
	"strings"
	"syscall"
	"unsafe"
)

const (
	MOD_ALT     = 0x0001
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004

	WM_HOTKEY = 0x0312
)

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	procGetMessageW       = user32.NewProc("GetMessageW")
)

var keyMap = map[string]uint32{
	"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73,
	"F5": 0x74, "F6": 0x75, "F7": 0x76, "F8": 0x77,
	"F9": 0x78, "F10": 0x79, "F11": 0x7A, "F12": 0x7B,
	"A": 0x41, "B": 0x42, "C": 0x43, "D": 0x44, "E": 0x45,
	"F": 0x46, "G": 0x47, "H": 0x48, "I": 0x49, "J": 0x4A,
	"K": 0x4B, "L": 0x4C, "M": 0x4D, "N": 0x4E, "O": 0x4F,
	"P": 0x50, "Q": 0x51, "R": 0x52, "S": 0x53, "T": 0x54,
	"U": 0x55, "V": 0x56, "W": 0x57, "X": 0x58, "Y": 0x59, "Z": 0x5A,
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
}

type Hotkey struct {
	ID        int
	Modifiers uint32
	KeyCode   uint32
}

type MSG struct {
	HWND   uintptr
	Msg    uint32
	WParam uintptr
	LParam uintptr
	Time   uint32
	Pt     struct{ X, Y int32 }
}

// ParseHotkey parses a string like "Ctrl+Shift+F1" into modifiers and key code
func ParseHotkey(s string) (modifiers uint32, keyCode uint32, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, nil // no hotkey configured
	}

	parts := strings.Split(s, "+")
	for i, part := range parts {
		part = strings.TrimSpace(strings.ToUpper(part))
		parts[i] = part
	}

	for _, part := range parts {
		switch part {
		case "CTRL", "CONTROL":
			modifiers |= MOD_CONTROL
		case "ALT":
			modifiers |= MOD_ALT
		case "SHIFT":
			modifiers |= MOD_SHIFT
		default:
			// must be the key
			if code, ok := keyMap[part]; ok {
				keyCode = code
			} else {
				return 0, 0, fmt.Errorf("unknown key: %s", part)
			}
		}
	}

	if keyCode == 0 {
		return 0, 0, fmt.Errorf("no key specified in hotkey: %s", s)
	}

	return modifiers, keyCode, nil
}

// RegisterHotkey registers a global hotkey with Windows
func RegisterHotkey(id int, modifiers, keyCode uint32) error {
	ret, _, err := procRegisterHotKey.Call(0, uintptr(id), uintptr(modifiers), uintptr(keyCode))
	if ret == 0 {
		return fmt.Errorf("RegisterHotKey failed: %v", err)
	}
	return nil
}

// UnregisterHotkey unregisters a global hotkey
func UnregisterHotkey(id int) {
	procUnregisterHotKey.Call(0, uintptr(id))
}

// HotkeyListener listens for hotkey events and calls the callback with the hotkey ID
func HotkeyListener(callback func(id int)) {
	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || int32(ret) == -1 {
			break
		}
		if msg.Msg == WM_HOTKEY {
			id := int(msg.WParam)
			slog.Info("hotkey pressed", "id", id)
			callback(id)
		}
	}
}
