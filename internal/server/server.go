package server

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"ICPCRemoteControl/internal/service"
)

//go:embed web/*
var webFS embed.FS

// Server is the main HTTP server.
type Server struct {
	httpServer *http.Server
	avahiCmd   *exec.Cmd
	bindIP     string
}

// Config holds server configuration.
type Config struct {
	Port      string
	BindIP    string
	DBPath    string
	Avahi     bool
	DeviceH    *service.DeviceHandler
	CommandH   *service.CommandHandler
	StatsH     *service.StatsHandler
	AdminWSH   *service.AdminWSHandler
	TerminalWSH *service.TerminalWSHandler
}

// New creates a new Server.
func New(cfg Config) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/stats", cfg.StatsH.GetStats)
	mux.HandleFunc("GET /api/devices", cfg.DeviceH.List)
	mux.HandleFunc("GET /api/devices/{id}", cfg.DeviceH.Get)
	mux.HandleFunc("DELETE /api/devices/{id}", cfg.DeviceH.Delete)
	mux.HandleFunc("POST /api/devices/reset", cfg.DeviceH.Reset)
	mux.HandleFunc("POST /api/commands", cfg.CommandH.Execute)
	mux.HandleFunc("GET /api/commands", cfg.CommandH.List)
	mux.HandleFunc("GET /api/commands/{id}", cfg.CommandH.Get)
	mux.HandleFunc("POST /api/commands/{id}/cancel", cfg.CommandH.Cancel)
	mux.HandleFunc("POST /api/commands/clear", cfg.CommandH.Clear)
	mux.HandleFunc("GET /api/presets", cfg.CommandH.Presets)

	mux.HandleFunc("GET /ws/admin", cfg.AdminWSH.Serve)
	mux.HandleFunc("GET /ws/terminal/{id}", cfg.TerminalWSH.Serve)

	webSubFS, err := fs.Sub(webFS, "web")
	if err != nil {
		panic("failed to get web sub filesystem: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(webSubFS))
	mux.Handle("GET /", fileServer)

	var handler http.Handler = mux
	handler = Recovery(Logger(handler))

	return &Server{
		httpServer: &http.Server{
			Addr:         ":" + cfg.Port,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		bindIP: cfg.BindIP,
	}
}

// Start begins listening and handles graceful shutdown.
func (s *Server) Start() error {
	go s.startAvahi()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("[server] shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("[server] shutdown error: %v", err)
		}
		s.stopAvahi()
	}()

	log.Printf("[server] listening on :%s", s.httpServer.Addr[1:])
	if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	log.Println("[server] stopped")
	return nil
}

func (s *Server) startAvahi() {
	path, err := exec.LookPath("avahi-publish")
	if err != nil {
		log.Println("[avahi] avahi-publish not found, mDNS publishing disabled")
		return
	}

	ip := s.bindIP
	if ip == "" {
		ip = "0.0.0.0"
	}
	log.Printf("[avahi] publishing icpc-server.local on %s via mDNS", ip)
	ip = "192.168.1.10" // 暂时写死
	s.avahiCmd = exec.Command(path, "-a", "-R", "icpc-server.local", ip)
	s.avahiCmd.Stdout = log.Writer()
	s.avahiCmd.Stderr = log.Writer()

	if err := s.avahiCmd.Start(); err != nil {
		log.Printf("[avahi] failed to start avahi-publish: %v", err)
		s.avahiCmd = nil
	}
}

func (s *Server) stopAvahi() {
	if s.avahiCmd != nil && s.avahiCmd.Process != nil {
		log.Println("[avahi] stopping avahi-publish")
		s.avahiCmd.Process.Signal(syscall.SIGTERM)
	}
}

// ListInterfaces prints available network interfaces and their IPs for user selection.
func ListInterfaces() {
	interfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	fmt.Println("Available network interfaces:")
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				fmt.Printf("  %s: %s\n", iface.Name, ipnet.IP.String())
			}
		}
	}
}

// AutoDetectIP returns a single IP if exactly one non-loopback interface is available.
func AutoDetectIP() string {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return "0.0.0.0"
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	if len(ips) == 1 {
		return ips[0]
	}
	return "0.0.0.0"
}
