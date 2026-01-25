package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Manager struct {
	logFunc    func(string)
	termFunc   func(string)
	appFunc    func(string)
	serverCmd  *exec.Cmd
	serverAddr string
}

func NewManager(log, term, app func(string), serverAddr string) *Manager {
	return &Manager{logFunc: log, termFunc: term, appFunc: app, serverAddr: serverAddr}
}

func (m *Manager) log(msg string) {
	if m.logFunc != nil {
		m.logFunc(msg)
	}
}

func (m *Manager) Stop() {
	if m.serverCmd != nil && m.serverCmd.Process != nil {
		fmt.Println("> PhileasGUI closing: Sending shutdown signal to server...")

		// Use 127.0.0.1 to avoid resolution issues
		addr := m.resolveAddr()
		url := fmt.Sprintf("http://%s/api/shutdown", addr)

		// Try Graceful Shutdown via API
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client := &http.Client{
			Timeout: 2 * time.Second,
		}

		req, _ := http.NewRequestWithContext(ctx, "POST", url, http.NoBody)
		resp, err := client.Do(req)

		if err == nil {
			fmt.Println("> Shutdown command sent successfully.")
			resp.Body.Close()
			time.Sleep(500 * time.Millisecond)
		} else {
			fmt.Printf("> API shutdown failed: %v\n", err)
		}
	}
}

func (m *Manager) Start() {
	go func() {
		// 1. Check Prerequisites
		if !m.checkPrerequisites() {
			m.termFunc("install.ps1")
			m.log("> Prerequisites missing. Running installer...")
			if err := m.runInstaller(); err != nil {
				m.log(fmt.Sprintf("> Installer failed: %v", err))
				return
			}
			m.log("> Installation complete.")
		}

		// 2. Check Server
		m.termFunc("phileasgo.exe")
		if !m.isServerRunning() {
			m.log("> Server not running. Starting phileasgo.exe...")
			go m.runServer()
		} else {
			m.log("> Server already active.")
			m.termFunc("server.log")
			go m.tailServerLog()
		}

		// 3. Wait for Readiness
		m.log("> Waiting for server...")
		for i := 0; i < 30; i++ {
			if m.isServerReady() {
				m.log("> Server ready!")
				m.appFunc(fmt.Sprintf("http://%s", m.serverAddr))
				return
			}
			time.Sleep(1 * time.Second)
		}
		m.log("> Error: Server timed out.")
	}()
}

func (m *Manager) checkPrerequisites() bool {
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		return false
	}

	// Check for either .env OR .env.local
	_, envErr := os.Stat(".env")
	_, envLocalErr := os.Stat(".env.local")

	if os.IsNotExist(envErr) && os.IsNotExist(envLocalErr) {
		return false
	}

	return true
}

func (m *Manager) runInstaller() error {
	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", "install.ps1")
	return m.runWithOutput(cmd)
}

func (m *Manager) runServer() {
	// We want to capture output here too
	cmd := exec.Command("./phileasgo.exe")
	m.serverCmd = cmd
	if err := m.runWithOutput(cmd); err != nil {
		m.log(fmt.Sprintf("Server exited with error: %v", err))
	}
}

func (m *Manager) runWithOutput(cmd *exec.Cmd) error {
	// Hide window on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return err
	}

	go m.streamReader(stdout)
	go m.streamReader(stderr)

	return cmd.Wait()
}

func (m *Manager) tailServerLog() {
	// Simple tail implementation
	file, err := os.Open("logs/server.log")
	if err != nil {
		m.log(fmt.Sprintf("Could not open log file: %v", err))
		return
	}
	defer file.Close()

	// Seek to end
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		m.log(fmt.Sprintf("Could not seek log file: %v", err))
		return
	}
	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			break
		}
		m.log(strings.TrimSpace(line))
	}
}

func (m *Manager) streamReader(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		m.log(scanner.Text())
	}
}

func (m *Manager) resolveAddr() string {
	addr := m.serverAddr
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr
	}
	if strings.HasPrefix(addr, "localhost:") {
		return strings.Replace(addr, "localhost:", "127.0.0.1:", 1)
	}
	return addr
}

func (m *Manager) isServerRunning() bool {
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/api/version", m.serverAddr))
	if err == nil {
		resp.Body.Close()
		return true
	}
	return false
}

func (m *Manager) isServerReady() bool {
	client := http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/api/version", m.serverAddr))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
