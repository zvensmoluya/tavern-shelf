//go:build windows

package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/zvensmoluya/tavern-shelf/internal/app"
	"github.com/zvensmoluya/tavern-shelf/internal/brand"
	"github.com/zvensmoluya/tavern-shelf/internal/desktop"
	"github.com/zvensmoluya/tavern-shelf/internal/httpapi"
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
	connectorListen := flag.String("connector-listen", "127.0.0.1:8787", "local connector listen address")
	connectorOrigin := flag.String("connector-origin", "", "allow one HTTPS SillyTavern page origin")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	shelf, err := app.Open(app.Options{DataDir: *dataDir, Logger: logger})
	if err != nil {
		panic(err)
	}
	autoStartArguments := []string{"--background"}
	if *dataDir != "" {
		autoStartArguments = append(autoStartArguments, "--data-dir", *dataDir)
	}
	if *connectorListen != "127.0.0.1:8787" {
		autoStartArguments = append(autoStartArguments, "--connector-listen", *connectorListen)
	}
	if *connectorOrigin != "" {
		autoStartArguments = append(autoStartArguments, "--connector-origin", *connectorOrigin)
	}
	actions := &desktop.Actions{AutoStartArguments: autoStartArguments}
	handler, err := httpapi.Handler(shelf, actions)
	if err != nil {
		_ = shelf.Close()
		panic(err)
	}
	connectorServer := &http.Server{Handler: httpapi.ConnectorHandler(shelf, *connectorOrigin), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second}
	connectorListener, listenErr := net.Listen("tcp", *connectorListen)
	if listenErr != nil {
		shelf.Connector.SetListener(*connectorListen, listenErr)
		logger.Warn("Tavern Shelf connector is unavailable", "address", *connectorListen, "error", listenErr)
	} else {
		shelf.Connector.SetListener(connectorListener.Addr().String(), nil)
		go func() {
			if err := connectorServer.Serve(connectorListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				shelf.Connector.SetListener(*connectorListen, err)
				logger.Warn("Tavern Shelf connector stopped", "error", err)
			}
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			cancel()
			_ = connectorServer.Close()
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
		Name: "Tavern Shelf", Description: "Your private character card library · " + version, Icon: icon,
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
			SetTitle("选择资源目录").
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
	icon, err := brand.IconPNG(64)
	if err != nil {
		panic(err)
	}
	return icon
}
