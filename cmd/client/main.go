package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"ICPCRemoteControl/internal/model"

	"github.com/creack/pty"
)

const (
	idFilePath       = "/var/lib/icpc-client/id"
	serverFile       = "server"
	serverDefaultURL = "icpc-server.local"
	serverPort       = "8081"
	cmdTimeout       = 60 * time.Second
	retryInterval    = 5 * time.Second
	pingInterval     = 15 * time.Second
	writeBufSize     = 128
)

func main() {
	if os.Geteuid() != 0 {
		log.Fatal("client must run as root")
	}
	serverFlag := flag.String("server", "", "override server address (ip:port)")
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[client] starting")

	serverAddr := resolveServer(*serverFlag)
	log.Printf("[client] server: %s", serverAddr)
	storedID := readStoredID()

	for {
		if err := connectAndServe(serverAddr, storedID); err != nil {
			log.Printf("[client] error: %v", err)
		}
		storedID = readStoredID()
		log.Printf("[client] reconnecting in %v", retryInterval)
		time.Sleep(retryInterval)
	}
}

func resolveServer(flagAddr string) string {
	if flagAddr != "" {
		return ensurePort(flagAddr)
	}
	addrs, _ := net.LookupHost(serverDefaultURL)
	if len(addrs) > 0 {
		return net.JoinHostPort(addrs[0], serverPort)
	}
	home, _ := os.UserHomeDir()
	data, _ := os.ReadFile(filepath.Join(home, serverFile))
	if len(data) > 0 {
		return ensurePort(strings.TrimSpace(string(data)))
	}
	log.Fatal("[client] cannot resolve server")
	return ""
}

func ensurePort(addr string) string {
	if !strings.Contains(addr, ":") {
		return addr + ":" + serverPort
	}
	return addr
}

func readStoredID() *int {
	data, _ := os.ReadFile(idFilePath)
	idStr := strings.TrimSpace(string(data))
	if idStr == "" { return nil }
	id, err := strconv.Atoi(idStr)
	if err != nil { return nil }
	return &id
}

func writeStoredID(id int) {
	os.MkdirAll(filepath.Dir(idFilePath), 0755)
	os.WriteFile(idFilePath, []byte(strconv.Itoa(id)), 0644)
}

func getMacAddress() string {
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 { continue }
		if len(i.HardwareAddr) > 0 { return i.HardwareAddr.String() }
	}
	return ""
}

func connectAndServe(serverAddr string, storedID *int) error {
	conn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil { return fmt.Errorf("dial: %w", err) }
	defer conn.Close()

	// Single write channel serializes all writes to conn.
	send := make(chan []byte, writeBufSize)
	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		for msg := range send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := conn.Write(msg); err != nil { return }
		}
	}()

	reader := bufio.NewReader(conn)
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// --- Registration ---
	hostname, _ := os.Hostname()
	sendJSON(send, model.RegisterRequest{
		Type: "register_request", AssignedID: storedID,
		MacAddress: getMacAddress(), Hostname: hostname,
	})

	line, err := reader.ReadString('\n')
	if err != nil { return fmt.Errorf("register_response: %w", err) }
	var regResp model.RegisterResponse
	json.Unmarshal([]byte(line), &regResp)
	log.Printf("[client] assigned ID: %d", regResp.AssignedID)

	writeStoredID(regResp.AssignedID)
	renameHostname(regResp.AssignedID)

	sysInfo, _ := collectSystemInfo(regResp.AssignedID)
	sendJSON(send, sysInfo)

	log.Println("[client] ready")
	conn.SetDeadline(time.Time{})

	// --- Main loop ---
	done := make(chan struct{})
	defer close(done)

	runningCmds := make(map[int64]*exec.Cmd)
	var cmdMu sync.Mutex
	termSessions := make(map[string]*os.File)
	var termMu sync.Mutex

	// Heartbeat.
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done: return
			case <-ticker.C:
				sendJSON(send, model.PingMessage{Type: "ping"})
			}
		}
	}()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			<-writeDone
			if err == io.EOF { return fmt.Errorf("server closed connection") }
			return fmt.Errorf("read: %w", err)
		}

		var base struct{ Type string }
		json.Unmarshal([]byte(line), &base)

		switch base.Type {
		case "execute":
			var msg model.ExecuteMessage
			json.Unmarshal([]byte(line), &msg)
			go runCommandStreaming(send, &msg, &cmdMu, runningCmds)

		case "cancel":
			var msg model.CancelMessage
			json.Unmarshal([]byte(line), &msg)
			cmdMu.Lock()
			if cmd, ok := runningCmds[msg.CommandID]; ok && cmd.Process != nil {
				log.Printf("[client] canceling cmd %d (pid=%d)", msg.CommandID, cmd.Process.Pid)
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				cmd.Process.Wait()
				delete(runningCmds, msg.CommandID)
			}
			cmdMu.Unlock()

		case "terminal_open":
			var msg model.TerminalOpenMessage
			json.Unmarshal([]byte(line), &msg)
			go startTerminal(send, &msg, &termMu, termSessions)

		case "terminal_input":
			var msg model.TerminalInputMessage
			json.Unmarshal([]byte(line), &msg)
			termMu.Lock()
			if f, ok := termSessions[msg.SessionID]; ok { f.Write([]byte(msg.Data)) }
			termMu.Unlock()

		case "terminal_resize":
			var msg model.TerminalResizeMessage
			json.Unmarshal([]byte(line), &msg)
			termMu.Lock()
			if f, ok := termSessions[msg.SessionID]; ok {
				pty.Setsize(f, &pty.Winsize{Rows: uint16(msg.Rows), Cols: uint16(msg.Cols)})
			}
			termMu.Unlock()

		case "terminal_close":
			var msg model.TerminalCloseMessage
			json.Unmarshal([]byte(line), &msg)
			termMu.Lock()
			if f, ok := termSessions[msg.SessionID]; ok { f.Close(); delete(termSessions, msg.SessionID) }
			termMu.Unlock()

		case "pong":
		}
	}
}

func sendJSON(ch chan<- []byte, v interface{}) {
	data, _ := json.Marshal(v)
	data = append(data, '\n')
	select {
	case ch <- data:
	default:
	}
}

func renameHostname(id int) {
	name := strconv.Itoa(id)
	exec.Command("hostnamectl", "set-hostname", name).Run()
	exec.Command("hostname", name).Run()
}

func collectSystemInfo(id int) (*model.SystemInfoMessage, error) {
	out, err := exec.Command("fastfetch", "--format", "json").Output()
	if err != nil { return nil, err }
	var entries []json.RawMessage
	json.Unmarshal(out, &entries)
	return &model.SystemInfoMessage{Type: "system_info", AssignedID: id, Info: entries}, nil
}

func runCommandStreaming(send chan<- []byte, msg *model.ExecuteMessage, mu *sync.Mutex, running map[int64]*exec.Cmd) {
	log.Printf("[client] cmd %d: %s", msg.CommandID, msg.Command)
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", msg.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	mu.Lock()
	running[msg.CommandID] = cmd
	mu.Unlock()

	cmd.Start()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			sendJSON(send, model.CommandOutputMessage{Type: "command_output", CommandID: msg.CommandID, Stream: "stdout", Line: scanner.Text()})
		}
	}()
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			sendJSON(send, model.CommandOutputMessage{Type: "command_output", CommandID: msg.CommandID, Stream: "stderr", Line: scanner.Text()})
		}
	}()

	wg.Wait()
	cmd.Wait()
	duration := time.Since(start)

	mu.Lock()
	delete(running, msg.CommandID)
	mu.Unlock()

	status := model.CommandStatusCompleted
	errMsg := ""
	if ctx.Err() != nil {
		status = model.CommandStatusTimeout
		errMsg = fmt.Sprintf("timeout after %v", cmdTimeout)
	} else if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		status = model.CommandStatusFailed
	}

	sendJSON(send, model.CommandResultMessage{
		Type: "command_result", CommandID: msg.CommandID,
		Status: status, ErrorOutput: errMsg, DurationMS: duration.Milliseconds(),
	})
	log.Printf("[client] cmd %d: %s (%v)", msg.CommandID, status, duration)
}

func startTerminal(send chan<- []byte, msg *model.TerminalOpenMessage, mu *sync.Mutex, sessions map[string]*os.File) {
	log.Printf("[client] terminal %s: cols=%d rows=%d", msg.SessionID, msg.Cols, msg.Rows)
	cmd := exec.Command("bash")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(msg.Rows), Cols: uint16(msg.Cols)})
	if err != nil {
		sendJSON(send, model.TerminalClosedMessage{Type: "terminal_closed", SessionID: msg.SessionID})
		return
	}
	mu.Lock()
	sessions[msg.SessionID] = f
	mu.Unlock()

	defer func() {
		f.Close(); cmd.Wait()
		mu.Lock(); delete(sessions, msg.SessionID); mu.Unlock()
		sendJSON(send, model.TerminalClosedMessage{Type: "terminal_closed", SessionID: msg.SessionID})
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			if err != nil { return }
			sendJSON(send, model.TerminalOutputMessage{Type: "terminal_output", SessionID: msg.SessionID, Data: string(buf[:n])})
		}
	}()

	cmd.Wait()
}
