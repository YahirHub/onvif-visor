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
	"onvif-visor/internal/analytics"
	"onvif-visor/internal/media"
	"onvif-visor/internal/server"
	"onvif-visor/internal/service"
	"onvif-visor/internal/store"
	"onvif-visor/web"
)

type serverOptions struct {
	Listen            string
	DataDir           string
	RecordingsDir     string
	FFmpeg            string
	AnalyticsModel    string
	ONNXRuntime       string
	BootstrapUser     string
	BootstrapPassword string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "service-run" {
		if len(args) != 1 {
			return errors.New("service-run no acepta argumentos adicionales")
		}
		runtimeCfg, err := service.Runtime()
		if err != nil {
			return fmt.Errorf("cargar instalación del servicio: %w", err)
		}
		return runServer(serverOptions{
			Listen: runtimeCfg.Listen, DataDir: runtimeCfg.DataDir,
			RecordingsDir: runtimeCfg.RecordingsDir, FFmpeg: runtimeCfg.FFmpeg,
			AnalyticsModel: filepath.Join(runtimeCfg.ModelsDir, analytics.ModelFileName), ONNXRuntime: runtimeCfg.ONNXRuntime,
		})
	}

	flags := flag.NewFlagSet("onvif-visor", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	listenAddr := flags.String("listen", "127.0.0.1:8080", "dirección HTTP de escucha")
	dataDir := flags.String("data-dir", "data", "directorio de configuración y credenciales")
	recordings := flags.String("recordings-dir", "recordings", "directorio de grabaciones")
	ffmpegPath := flags.String("ffmpeg", "ffmpeg", "ruta al ejecutable ffmpeg")
	analyticsModel := flags.String("analytics-model", "", "ruta opcional al modelo NanoDet ONNX")
	onnxRuntime := flags.String("onnxruntime", os.Getenv("ONVIF_ONNXRUNTIME"), "ruta opcional a libonnxruntime.so")
	uiUser := flags.String("ui-user", os.Getenv("ONVIF_VISOR_USER"), "usuario bootstrap de acceso al visor")
	uiPassword := flags.String("ui-password", os.Getenv("ONVIF_VISOR_PASSWORD"), "contraseña bootstrap de acceso al visor")
	install := flags.Bool("install", false, "instalar o actualizar ONVIF Visor como servicio del sistema")
	remove := flags.Bool("remove", false, "desinstalar el servicio y binario, conservando datos y grabaciones")
	serviceAction := flags.String("service", "", "administrar servicio instalado: status|start|stop|restart")
	sourceDir := flags.String("source-dir", "", "origen de data/ y recordings/ para la primera instalación")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) > 0 {
		return fmt.Errorf("argumentos no reconocidos: %s", strings.Join(flags.Args(), " "))
	}
	if (*install && *remove) || ((*install || *remove) && strings.TrimSpace(*serviceAction) != "") {
		return errors.New("--install, --remove y --service son operaciones excluyentes")
	}
	bootstrapUser := strings.TrimSpace(*uiUser)
	bootstrapPassword := *uiPassword
	if (bootstrapUser == "") != (bootstrapPassword == "") {
		return errors.New("-ui-user y -ui-password deben configurarse juntos")
	}

	if *install {
		elevated, err := service.EnsureElevated(args)
		if err != nil {
			return err
		}
		if elevated {
			return nil
		}
		installListen := *listenAddr
		listenExplicit := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "listen" {
				listenExplicit = true
			}
		})
		if !listenExplicit {
			installListen = "0.0.0.0:8081"
		}
		origin := strings.TrimSpace(*sourceDir)
		if origin == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolver source-dir: %w", err)
			}
			origin = cwd
		}
		result, err := service.Install(service.InstallOptions{
			SourceDir:   origin,
			Listen:      installListen,
			FFmpeg:      *ffmpegPath,
			ONNXRuntime: strings.TrimSpace(*onnxRuntime),
			Prepare: func(runtimeCfg service.RuntimeConfig) error {
				if err := prepareInstalledAccount(runtimeCfg, bootstrapUser, bootstrapPassword); err != nil {
					return err
				}
				_, err := analytics.EnsureModel(runtimeCfg.ModelsDir)
				return err
			},
		})
		if err != nil {
			return err
		}
		fmt.Printf("ONVIF Visor instalado con %s\n", result.Manager)
		fmt.Printf("Binario: %s\nDatos: %s\nGrabaciones: %s\nCLI: %s\n", result.Binary, result.DataDir, result.RecordingsDir, result.CLI)
		return nil
	}

	if *remove {
		elevated, err := service.EnsureElevated(args)
		if err != nil {
			return err
		}
		if elevated {
			return nil
		}
		result, err := service.Remove()
		if err != nil {
			return err
		}
		fmt.Printf("Servicio ONVIF Visor eliminado (%s).\n", result.Manager)
		fmt.Printf("Datos conservados en %s y grabaciones en %s.\n", result.DataDir, result.RecordingsDir)
		return nil
	}

	if actionText := strings.ToLower(strings.TrimSpace(*serviceAction)); actionText != "" {
		action := service.Action(actionText)
		if action != service.ActionStatus {
			elevated, err := service.EnsureElevated(args)
			if err != nil {
				return err
			}
			if elevated {
				return nil
			}
		}
		output, err := service.Control(action)
		if output != "" {
			fmt.Println(output)
		}
		return err
	}

	return runServer(serverOptions{
		Listen: *listenAddr, DataDir: *dataDir, RecordingsDir: *recordings, FFmpeg: *ffmpegPath,
		AnalyticsModel: strings.TrimSpace(*analyticsModel), ONNXRuntime: strings.TrimSpace(*onnxRuntime),
		BootstrapUser: bootstrapUser, BootstrapPassword: bootstrapPassword,
	})
}

func prepareInstalledAccount(runtimeCfg service.RuntimeConfig, username, password string) error {
	accounts, err := account.Open(runtimeCfg.DataDir)
	if err != nil {
		return fmt.Errorf("abrir cuentas instaladas: %w", err)
	}
	defer accounts.Close()
	if username != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = accounts.EnsureBootstrapUser(ctx, username, password)
		cancel()
		if err != nil {
			return fmt.Errorf("preparar usuario instalado: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	hasUsers, err := accounts.HasUsers(ctx)
	cancel()
	if err != nil {
		return fmt.Errorf("comprobar usuarios instalados: %w", err)
	}
	if !isLoopbackListen(runtimeCfg.Listen) && !hasUsers {
		return errors.New("la instalación LAN necesita una cuenta: ejecuta --install con -ui-user y -ui-password o migra un data/accounts.db existente")
	}
	return nil
}

func runServer(opts serverOptions) error {
	storePath, err := filepath.Abs(opts.DataDir)
	if err != nil {
		return fmt.Errorf("resolver data-dir: %w", err)
	}
	recordingPath, err := filepath.Abs(opts.RecordingsDir)
	if err != nil {
		return fmt.Errorf("resolver recordings-dir: %w", err)
	}

	cameraStore, err := store.Open(storePath)
	if err != nil {
		return fmt.Errorf("abrir almacenamiento: %w", err)
	}
	accountStore, err := account.Open(storePath)
	if err != nil {
		return fmt.Errorf("abrir cuentas: %w", err)
	}
	defer accountStore.Close()

	if opts.BootstrapUser != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err = accountStore.EnsureBootstrapUser(ctx, opts.BootstrapUser, opts.BootstrapPassword)
		cancel()
		if err != nil {
			return fmt.Errorf("preparar usuario de acceso: %w", err)
		}
	}
	authCtx, authCancel := context.WithTimeout(context.Background(), 5*time.Second)
	hasUsers, err := accountStore.HasUsers(authCtx)
	authCancel()
	if err != nil {
		return fmt.Errorf("comprobar cuentas: %w", err)
	}
	if !isLoopbackListen(opts.Listen) && !hasUsers {
		return errors.New("al escuchar fuera de loopback debe existir al menos un usuario; define -ui-user y -ui-password en el primer arranque")
	}

	mediaManager, err := media.New(opts.FFmpeg, recordingPath)
	if err != nil {
		return fmt.Errorf("inicializar medios: %w", err)
	}

	modelPath := strings.TrimSpace(opts.AnalyticsModel)
	if modelPath == "" {
		modelPath, err = analytics.EnsureModel(filepath.Join(storePath, "models"))
		if err != nil {
			log.Printf("analytics=disabled reason=%v", err)
		}
	}
	var analyticsManager *analytics.Manager
	if modelPath != "" {
		analyticsManager, err = analytics.New(analytics.Config{
			DataDir: storePath, FFmpeg: opts.FFmpeg, ModelPath: modelPath,
			RuntimePath: opts.ONNXRuntime, Media: mediaManager,
		})
		if err != nil {
			log.Printf("analytics=disabled reason=%v", err)
			analyticsManager = nil
		} else {
			status := analyticsManager.Status()
			if status.Ready {
				log.Printf("analytics=ready model=%s sample_fps=%d", status.Model, status.SampleFPS)
			} else {
				log.Printf("analytics=disabled model=%s reason=%s", status.Model, status.LastError)
			}
		}
	}

	assets, err := web.FS()
	if err != nil {
		return fmt.Errorf("cargar interfaz: %w", err)
	}
	app, err := server.New(server.Config{Store: cameraStore, Accounts: accountStore, Media: mediaManager, Analytics: analyticsManager, Assets: assets, DataDir: opts.DataDir})
	if err != nil {
		if analyticsManager != nil {
			_ = analyticsManager.Close()
		}
		return fmt.Errorf("inicializar aplicación: %w", err)
	}
	defer app.Close()

	httpServer := &http.Server{
		Addr: opts.Listen, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 20 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20,
	}
	listener, err := net.Listen("tcp", opts.Listen)
	if err != nil {
		return fmt.Errorf("escuchar en %s: %w", opts.Listen, err)
	}
	log.Printf("ONVIF Visor listo en http://%s", displayAddress(listener.Addr().String()))
	if !isLoopbackListen(opts.Listen) {
		log.Printf("modo LAN activo: acceso protegido con sesiones persistentes")
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.Serve(listener) }()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	select {
	case sig := <-sigCh:
		log.Printf("apagando por señal %s", sig)
	case serveErr := <-errCh:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("servidor HTTP: %w", serveErr)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("apagado HTTP incompleto: %w", err)
	}
	return nil
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
