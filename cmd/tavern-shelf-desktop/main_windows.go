//go:build windows

package main

import (
	"bytes"
	"context"
	"flag"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/openai/tavern-shelf/internal/app"
	"github.com/openai/tavern-shelf/internal/desktop"
	"github.com/openai/tavern-shelf/internal/httpapi"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

var instanceKey = [32]byte{
	0x54, 0x61, 0x76, 0x65, 0x72, 0x6e, 0x53, 0x68,
	0x65, 0x6c, 0x66, 0x2d, 0x57, 0x69, 0x6e, 0x64,
	0x6f, 0x77, 0x73, 0x2d, 0x56, 0x30, 0x2d, 0x4c,
	0x6f, 0x63, 0x61, 0x6c, 0x2d, 0x30, 0x31, 0x21,
}

func main() {
	dataDir := flag.String("data-dir", "", "managed data directory")
	background := flag.Bool("background", false, "start hidden in the system tray")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	shelf, err := app.Open(app.Options{DataDir: *dataDir, Logger: logger})
	if err != nil {
		panic(err)
	}
	actions := desktop.Actions{Inbox: shelf.Paths.Inbox}
	handler, err := httpapi.Handler(shelf, actions)
	if err != nil {
		_ = shelf.Close()
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			cancel()
			_ = shelf.Close()
		})
	}
	go func() {
		if err := shelf.Scanner.Run(ctx); err != nil {
			logger.Error("scanner stopped", "error", err)
		}
	}()

	icon := makeIcon()
	var window *application.WebviewWindow
	wailsApp := application.New(application.Options{
		Name: "Tavern Shelf", Description: "Your private character card library", Icon: icon,
		Assets:  application.AssetOptions{Handler: handler, DisableLogging: true},
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true, WebviewUserDataPath: filepath.Join(shelf.Paths.AppData, "webview")},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.tavernplayer.tavern-shelf", EncryptionKey: instanceKey,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if window != nil {
					showWindow(window)
				}
			},
		},
		OnShutdown: shutdown,
	})
	window = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: "library", Title: "Tavern Shelf", Width: 1180, Height: 760,
		MinWidth: 760, MinHeight: 560, URL: "/", Hidden: *background,
		BackgroundColour: application.NewRGB(23, 19, 15),
	})
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		window.Hide()
		event.Cancel()
	})

	tray := wailsApp.SystemTray.New()
	tray.SetIcon(icon)
	tray.SetTooltip("Tavern Shelf · 正在监视 Inbox")
	tray.OnClick(func() { showWindow(window) })
	menu := wailsApp.NewMenu()
	menu.Add("打开 Tavern Shelf").OnClick(func(*application.Context) { showWindow(window) })
	menu.Add("打开 Inbox").OnClick(func(*application.Context) { _ = actions.OpenInbox() })
	menu.AddSeparator()
	autoStart, _ := actions.AutoStartEnabled()
	menu.AddCheckbox("登录后静默启动", autoStart).OnClick(func(ctx *application.Context) {
		_ = actions.SetAutoStart(ctx.ClickedMenuItem().Checked())
	})
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) { wailsApp.Quit() })
	tray.SetMenu(menu)

	if err := wailsApp.Run(); err != nil {
		shutdown()
		panic(err)
	}
}

func showWindow(window *application.WebviewWindow) {
	window.Show()
	window.Restore()
	window.Focus()
}

func makeIcon() []byte {
	canvas := image.NewRGBA(image.Rect(0, 0, 64, 64))
	gold := color.RGBA{R: 215, G: 168, B: 92, A: 255}
	dark := color.RGBA{R: 23, G: 19, B: 15, A: 255}
	for y := 4; y < 60; y++ {
		for x := 4; x < 60; x++ {
			dx, dy := x-32, y-32
			if dx*dx+dy*dy <= 28*28 {
				canvas.SetRGBA(x, y, dark)
			}
		}
	}
	for x := 14; x < 50; x++ {
		canvas.SetRGBA(x, 48, gold)
		canvas.SetRGBA(x, 49, gold)
	}
	books := []image.Rectangle{image.Rect(17, 19, 24, 47), image.Rect(27, 14, 35, 47), image.Rect(38, 22, 46, 47)}
	for _, book := range books {
		for y := book.Min.Y; y < book.Max.Y; y++ {
			for x := book.Min.X; x < book.Max.X; x++ {
				if x == book.Min.X || x == book.Max.X-1 || y == book.Min.Y || y == book.Max.Y-1 {
					canvas.SetRGBA(x, y, gold)
				}
			}
		}
	}
	var output bytes.Buffer
	_ = png.Encode(&output, canvas)
	return output.Bytes()
}
