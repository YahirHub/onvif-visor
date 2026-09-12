package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"onvif-visor/internal/account"
	"onvif-visor/internal/media"
	"onvif-visor/internal/server"
	"onvif-visor/internal/store"
	"onvif-visor/web"
)

func main() {
	var (
		listenAddr = flag.String("listen", "127.0.0.1:8080", "dirección HTTP de escucha")
		dataDir    = flag.String("data-dir", "data", "directorio de configuración y credenciales")
		recordings = flag.String("recordings-dir", "recordings", "directorio de grabaciones")
		ffmpegPath = flag.String("ffmpeg", "ffmpeg", "ruta al ejecutable ffmpeg")
		uiUser     = flag.String("ui-user", os.Getenv("ONVIF_VISOR_USER"), "usuario bootstrap de acceso al visor")
		uiPassword = flag.String("ui-password", os.Getenv("ONVIF_VISOR_PASSWORD"), "contraseña bootstrap de acceso al visor")
	)
	flag.Parse()

	storePath, err := filepath.Abs(*dataDir)
	if err != nil {
		log.Fatalf("resolver data-dir: %v", err)
	}
	recordingPath, err := filepath.Abs(*recordings)
	if err != nil {
		log.Fatalf("resolver recordings-dir: %v", err)
	}

	cameraStore, err := store.Open(storePath)
	if err != nil {
		log.Fatalf("abrir almacenamiento: %v", err)
	}
	accountStore, err := account.Open(storePath)
	if err != nil {
		log.Fatalf("abrir cuentas: %v", err)
	}
	defer accountStore.Close()

	bootstrapUser := strings.TrimSpace(*uiUser)
	bootstrapPassword := *uiPassword
	if (bootstrapUser == "") != (bootstrapPassword == "") {
		log.Fatal("-ui-user y -ui-password deben configurarse juntos")
	}
	if bootstrapUser != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = accountStore.EnsureBootstrapUser(ctx, bootstrapUser, bootstrapPassword)
		cancel()
		if err != nil {
			log.Fatalf("preparar usuario de acceso: %v", err)
		}
	}
	authCtx, authCancel := context.WithTimeout(context.Background(), 5*time.Second)
	hasUsers, err := accountStore.HasUsers(authCtx)
	authCancel()
	if err != nil {
		log.Fatalf("comprobar cuentas: %v", err)
	}
	if !isLoopbackListen(*listenAddr) && !hasUsers {
		log.Fatal("al escuchar fuera de loopback debe existir al menos un usuario; define -ui-user y -ui-password en el primer arranque")
	}

	mediaManager, err := media.New(*ffmpegPath, recordingPath)
	if err != nil {
		log.Fatalf("inicializar medios: %v", err)
	}
	assets, err := web.FS()
	if err != nil {
		log.Fatalf("cargar interfaz: %v", err)
	}
	app, err := server.New(server.Config{
		Store:    cameraStore,
		Accounts: accountStore,
		Media:    mediaManager,
		Assets:   assets,
	})
	if err != nil {
		log.Fatalf("inicializar aplicación: %v", err)
	}
	defer app.Close()

	httpServer := &http.Server{
		Addr:              *listenAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	listener, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("escuchar en %s: %v", *listenAddr, err)
	}
	log.Printf("ONVIF Visor listo en http://%s", displayAddress(listener.Addr().String()))
	if !isLoopbackListen(*listenAddr) {
		log.Printf("modo LAN activo: acceso protegido con sesiones persistentes")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Serve(listener)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("apagando por señal %s", sig)
	case serveErr := <-errCh:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("servidor HTTP: %v", serveErr)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("apagado HTTP incompleto: %v", err)
	}
}

func isLoopbackListen(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func displayAddress(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "::" || host == "0.0.0.0" || host == "" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("%s:%s", host, port)
}
