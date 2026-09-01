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
	actions := &desktop.Actions{}
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
		BackgroundColour: application.NewRGB(13, 15, 18),
	})
	actions.ChooseInboxFunc = func() (string, error) {
		initial := shelf.Paths.Inbox
		if directories := shelf.Inboxes(); len(directories) > 0 {
			initial = directories[0]
		}
		return wailsApp.Dialog.OpenFile().
			CanChooseFiles(false).
			CanChooseDirectories(true).
			CanCreateDirectories(true).
			AttachToWindow(window).
			SetTitle("选择要自动扫描的目录").
			SetDirectory(initial).
			PromptForSingleSelection()
	}
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
	menu.Add("打开 Inbox").OnClick(func(*application.Context) {
		if directories := shelf.Inboxes(); len(directories) > 0 {
			_ = actions.OpenInbox(directories[0])
		}
	})
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
	dark := color.RGBA{R: 21, G: 24, B: 29, A: 255}
	border := color.RGBA{R: 54, G: 59, B: 69, A: 255}
	light := color.RGBA{R: 232, G: 235, B: 239, A: 255}
	muted := color.RGBA{R: 104, G: 112, B: 125, A: 255}
	for y := 4; y < 60; y++ {
		for x := 4; x < 60; x++ {
			dx := max(0, max(19-x, x-44))
			dy := max(0, max(19-y, y-44))
			if dx*dx+dy*dy <= 15*15 {
				canvas.SetRGBA(x, y, dark)
				if x < 6 || x >= 58 || y < 6 || y >= 58 {
					canvas.SetRGBA(x, y, border)
				}
			}
		}
	}
	fillRect(canvas, image.Rect(17, 18, 30, 47), light)
	fillRect(canvas, image.Rect(34, 18, 47, 47), light)
	fillRect(canvas, image.Rect(21, 23, 26, 42), muted)
	fillRect(canvas, image.Rect(38, 23, 43, 42), muted)
	fillRect(canvas, image.Rect(14, 49, 50, 51), muted)
	var output bytes.Buffer
	_ = png.Encode(&output, canvas)
	return output.Bytes()
}

func fillRect(canvas *image.RGBA, rectangle image.Rectangle, fill color.RGBA) {
	for y := rectangle.Min.Y; y < rectangle.Max.Y; y++ {
		for x := rectangle.Min.X; x < rectangle.Max.X; x++ {
			canvas.SetRGBA(x, y, fill)
		}
	}
}
