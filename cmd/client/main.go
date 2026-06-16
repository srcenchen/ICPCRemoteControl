package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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
	writeBufSize     = 128
)

//go:embed checkin_page.html
var checkinPageFS embed.FS

var (
	cmdTimeout    time.Duration
	retryInterval time.Duration
	pingInterval  time.Duration
)

// checkinBridge provides a communication bridge between the HTTP server and
// the TCP connection for check-in request/response correlation.
var (
	checkinMu      sync.Mutex
	checkinWaiters = make(map[string]chan model.CheckinResponseMessage)
)

// clientState holds mutable state that the HTTP handler needs to access.
type clientState struct {
	mu               sync.Mutex
	send             chan<- []byte
	assignedID       int
	hostname         string
	macAddr          string
	ipAddr           string
	checkinStatus    int    // 0=未签到, 1=已签到, 2=已签退
	studentName      string
	studentNum       string
	checkinTime      string
	checkoutTime     string
	welcomeText      string // from server config
	warningText      string
	postCheckinMsg   string
	postCheckoutCmd  string
	postCheckoutMsg  string
}

var state = &clientState{}

func main() {
	if os.Geteuid() != 0 {
		log.Fatal("client must run as root")
	}
	serverFlag := flag.String("server", "", "override server address (ip:port)")
	cmdTimeoutFlag := flag.Int("cmd-timeout", 60, "command execution timeout in seconds")
	retryFlag := flag.Int("retry", 5, "reconnect retry interval in seconds")
	pingFlag := flag.Int("ping", 15, "heartbeat ping interval in seconds")
	flag.Parse()

	cmdTimeout = time.Duration(*cmdTimeoutFlag) * time.Second
	retryInterval = time.Duration(*retryFlag) * time.Second
	pingInterval = time.Duration(*pingFlag) * time.Second

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[client] starting")

	// Start the check-in HTTP server on port 8090.
	go startCheckinServer()

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

// ---- Check-in HTTP Server ----

func startCheckinServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", serveCheckinPage)
	mux.HandleFunc("GET /api/info", handleCheckinInfo)
	mux.HandleFunc("POST /api/checkin", handleCheckinSubmit)
	mux.HandleFunc("POST /api/checkout", handleCheckoutSubmit)

	srv := &http.Server{
		Addr:         ":8090",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Println("[checkin-http] listening on :8090")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("[checkin-http] error: %v", err)
	}
}

func serveCheckinPage(w http.ResponseWriter, r *http.Request) {
	data, err := checkinPageFS.ReadFile("checkin_page.html")
	if err != nil {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// handleCheckinInfo returns the current device info for the check-in page.
func handleCheckinInfo(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	assignedID := state.assignedID
	hostname := state.hostname
	macAddr := state.macAddr
	ipAddr := state.ipAddr
	checkinStatus := state.checkinStatus
	studentName := state.studentName
	studentNum := state.studentNum
	checkinTime := state.checkinTime
	checkoutTime := state.checkoutTime
	welcomeText := state.welcomeText
	warningText := state.warningText
	postCheckinMsg := state.postCheckinMsg
	postCheckoutMsg := state.postCheckoutMsg
	state.mu.Unlock()

	if assignedID == 0 {
		writeClientJSON(w, http.StatusOK, map[string]interface{}{
			"code":             -1,
			"message":          "设备尚未注册到服务器，请等待连接建立",
			"assigned_id":      0,
			"hostname":         hostname,
			"mac_address":      macAddr,
			"ip_address":       ipAddr,
			"checkin_status":   0,
			"welcome_text":     welcomeText,
			"warning_text":     warningText,
			"post_checkin_msg": postCheckinMsg,
			"post_checkout_msg": postCheckoutMsg,
		})
		return
	}

	writeClientJSON(w, http.StatusOK, map[string]interface{}{
		"code":             checkinStatus,
		"assigned_id":      assignedID,
		"hostname":         hostname,
		"mac_address":      macAddr,
		"ip_address":       ipAddr,
		"checkin_status":   checkinStatus,
		"student_name":     studentName,
		"student_num":      studentNum,
		"checkin_time":     checkinTime,
		"checkout_time":    checkoutTime,
		"welcome_text":     welcomeText,
		"warning_text":     warningText,
		"post_checkin_msg": postCheckinMsg,
		"post_checkout_msg": postCheckoutMsg,
	})
}

// handleCheckinSubmit receives the check-in form POST and forwards it to the server via TCP.
func handleCheckinSubmit(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	assignedID := state.assignedID
	sendCh := state.send
	state.mu.Unlock()

	if assignedID == 0 {
		writeClientJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "message": "设备尚未注册到服务器",
		})
		return
	}

	var body struct {
		StudentName string `json:"student_name"`
		StudentNum  string `json:"student_num"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeClientJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "message": "无效的请求数据",
		})
		return
	}
	if body.StudentName == "" || body.StudentNum == "" {
		writeClientJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "message": "请填写姓名和学号",
		})
		return
	}

	corrID := fmt.Sprintf("checkin_%d", time.Now().UnixNano())
	respCh := make(chan model.CheckinResponseMessage, 1)
	checkinMu.Lock()
	checkinWaiters[corrID] = respCh
	checkinMu.Unlock()

	sendJSONSafe(sendCh, model.CheckinMessage{
		Type: "checkin", CorrelationID: corrID,
		StudentName: body.StudentName, StudentNum: body.StudentNum,
	})

	var success bool
	var msg string
	select {
	case resp := <-respCh:
		success = resp.Success
		if success {
			msg = resp.PostCheckinMsg
		} else {
			msg = resp.Message
		}
	case <-time.After(10 * time.Second):
		checkinMu.Lock()
		delete(checkinWaiters, corrID)
		checkinMu.Unlock()
		writeClientJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "message": "签到请求超时，请重试",
		})
		return
	}

	if success {
		state.mu.Lock()
		state.checkinStatus = 1
		state.studentName = body.StudentName
		state.studentNum = body.StudentNum
		state.checkinTime = time.Now().Format(time.RFC3339)
		state.mu.Unlock()
		if msg == "" {
			msg = "签到成功"
		}
	}

	writeClientJSON(w, http.StatusOK, map[string]interface{}{
		"success": success, "message": msg,
	})
}

// handleCheckoutSubmit handles the client-side checkout request.
func handleCheckoutSubmit(w http.ResponseWriter, r *http.Request) {
	state.mu.Lock()
	assignedID := state.assignedID
	sendCh := state.send
	state.mu.Unlock()

	if assignedID == 0 {
		writeClientJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "message": "设备尚未注册到服务器",
		})
		return
	}

	corrID := fmt.Sprintf("checkout_%d", time.Now().UnixNano())
	respCh := make(chan model.CheckinResponseMessage, 1)
	checkinMu.Lock()
	checkinWaiters[corrID] = respCh
	checkinMu.Unlock()

	sendJSONSafe(sendCh, model.CheckinMessage{
		Type: "checkout", CorrelationID: corrID,
	})

	var success bool
	var cmd, msg string
	select {
	case resp := <-respCh:
		success = resp.Success
		cmd = resp.PostCheckoutCmd
		msg = resp.PostCheckoutMsg
	case <-time.After(10 * time.Second):
		checkinMu.Lock()
		delete(checkinWaiters, corrID)
		checkinMu.Unlock()
		writeClientJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "message": "签退请求超时，请重试",
		})
		return
	}

	if success {
		state.mu.Lock()
		state.checkinStatus = 2
		state.checkoutTime = time.Now().Format(time.RFC3339)
		state.mu.Unlock()

		if cmd != "" {
			log.Printf("[checkin] executing post-checkout command: %s", cmd)
			go exec.Command("sh", "-c", cmd).Run()
		}
		if msg == "" {
			msg = "签退成功"
		}
	}

	writeClientJSON(w, http.StatusOK, map[string]interface{}{
		"success": success, "message": msg,
	})
}

func sendJSONSafe(ch chan<- []byte, v interface{}) {
	data, _ := json.Marshal(v)
	data = append(data, '\n')
	select {
	case ch <- data:
	default:
		log.Printf("[checkin-http] send buffer full, dropping message of type %T", v)
	}
}

func writeClientJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ---- Server Discovery ----

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
	if idStr == "" {
		return nil
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil
	}
	return &id
}

func writeStoredID(id int) {
	os.MkdirAll(filepath.Dir(idFilePath), 0755)
	os.WriteFile(idFilePath, []byte(strconv.Itoa(id)), 0644)
}

func getMacAddress() string {
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 {
			continue
		}
		if len(i.HardwareAddr) > 0 {
			return i.HardwareAddr.String()
		}
	}
	return ""
}

// getLocalIP returns the preferred non-loopback IPv4 address.
func getLocalIP() string {
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// ---- TCP Connection ----

func connectAndServe(serverAddr string, storedID *int) error {
	conn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	// Single write channel serializes all writes to conn.
	send := make(chan []byte, writeBufSize)
	writeDone := make(chan struct{})
	defer close(send) // ensure write goroutine exits on any return path

	// Register the send channel for HTTP handler access.
	state.mu.Lock()
	state.send = send
	state.mu.Unlock()
	defer func() {
		state.mu.Lock()
		state.send = nil
		state.mu.Unlock()
	}()

	go func() {
		defer close(writeDone)
		for msg := range send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := conn.Write(msg); err != nil {
				return
			}
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
	if err != nil {
		return fmt.Errorf("register_response: %w", err)
	}
	var regResp model.RegisterResponse
	if err := json.Unmarshal([]byte(line), &regResp); err != nil {
		return fmt.Errorf("unmarshal register_response: %w", err)
	}
	log.Printf("[client] assigned ID: %d, prefix: %s", regResp.AssignedID, regResp.HostnamePrefix)

	writeStoredID(regResp.AssignedID)
	renameHostname(regResp.AssignedID, regResp.HostnamePrefix)

	// Update state for HTTP handlers.
	state.mu.Lock()
	state.assignedID = regResp.AssignedID
	state.hostname = hostname
	state.macAddr = getMacAddress()
	state.ipAddr = getLocalIP()
	state.mu.Unlock()

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
			case <-done:
				return
			case <-ticker.C:
				sendJSON(send, model.PingMessage{Type: "ping"})
			}
		}
	}()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			<-writeDone
			if err == io.EOF {
				return fmt.Errorf("server closed connection")
			}
			return fmt.Errorf("read: %w", err)
		}

		var base struct{ Type string }
		if err := json.Unmarshal([]byte(line), &base); err != nil {
			log.Printf("[client] unmarshal message type: %v", err)
			continue
		}

		switch base.Type {
		case "execute":
			var msg model.ExecuteMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[client] unmarshal execute: %v", err)
				continue
			}
			go runCommandStreaming(send, &msg, &cmdMu, runningCmds)

		case "cancel":
			var msg model.CancelMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[client] unmarshal cancel: %v", err)
				continue
			}
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
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[client] unmarshal terminal_open: %v", err)
				continue
			}
			go startTerminal(send, &msg, &termMu, termSessions)

		case "terminal_input":
			var msg model.TerminalInputMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[client] unmarshal terminal_input: %v", err)
				continue
			}
			termMu.Lock()
			if f, ok := termSessions[msg.SessionID]; ok {
				f.Write([]byte(msg.Data))
			}
			termMu.Unlock()

		case "terminal_resize":
			var msg model.TerminalResizeMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[client] unmarshal terminal_resize: %v", err)
				continue
			}
			termMu.Lock()
			if f, ok := termSessions[msg.SessionID]; ok {
				pty.Setsize(f, &pty.Winsize{Rows: uint16(msg.Rows), Cols: uint16(msg.Cols)})
			}
			termMu.Unlock()

		case "terminal_close":
			var msg model.TerminalCloseMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				log.Printf("[client] unmarshal terminal_close: %v", err)
				continue
			}
			termMu.Lock()
			if f, ok := termSessions[msg.SessionID]; ok {
				f.Close()
				delete(termSessions, msg.SessionID)
			}
			termMu.Unlock()

		case "checkin_config":
			var cfg model.CheckinConfigMessage
			if err := json.Unmarshal([]byte(line), &cfg); err != nil {
				log.Printf("[client] unmarshal checkin_config: %v", err)
				continue
			}
			state.mu.Lock()
			state.welcomeText = cfg.WelcomeText
			state.warningText = cfg.WarningText
			state.postCheckinMsg = cfg.PostCheckinMsg
			state.postCheckoutCmd = cfg.PostCheckoutCmd
			state.postCheckoutMsg = cfg.PostCheckoutMsg
			state.mu.Unlock()
			log.Printf("[client] received checkin config from server")

		case "checkin_response":
			var resp model.CheckinResponseMessage
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				log.Printf("[client] unmarshal checkin_response: %v", err)
				continue
			}
			checkinMu.Lock()
			if ch, ok := checkinWaiters[resp.CorrelationID]; ok {
				ch <- resp
				delete(checkinWaiters, resp.CorrelationID)
			}
			checkinMu.Unlock()

		case "pong":
		}
	}
}

func sendJSON(ch chan<- []byte, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[client] json marshal error (%T): %v", v, err)
		return
	}
	data = append(data, '\n')
	select {
	case ch <- data:
	default:
		log.Printf("[client] send buffer full, dropping message of type %T", v)
	}
}

func renameHostname(id int, prefix string) {
	name := fmt.Sprintf("%s-%d", prefix, id)
	exec.Command("hostnamectl", "set-hostname", name).Run()
	exec.Command("hostname", name).Run()
}

func collectSystemInfo(id int) (*model.SystemInfoMessage, error) {
	out, err := exec.Command("fastfetch", "--format", "json").Output()
	if err != nil {
		return nil, err
	}
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
		f.Close()
		cmd.Wait()
		mu.Lock()
		delete(sessions, msg.SessionID)
		mu.Unlock()
		sendJSON(send, model.TerminalClosedMessage{Type: "terminal_closed", SessionID: msg.SessionID})
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			if err != nil {
				return
			}
			sendJSON(send, model.TerminalOutputMessage{Type: "terminal_output", SessionID: msg.SessionID, Data: string(buf[:n])})
		}
	}()

	cmd.Wait()
}
