package main

import (
	"bufio"
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
	logFunc   func(string)
	termFunc  func(string)
	appFunc   func(string)
	serverCmd *exec.Cmd
}

func NewManager(log, term, app func(string)) *Manager {
	return &Manager{logFunc: log, termFunc: term, appFunc: app}
}

func (m *Manager) log(msg string) {
	if m.logFunc != nil {
		m.logFunc(msg)
	}
}

func (m *Manager) Stop() {
	if m.serverCmd != nil && m.serverCmd.Process != nil {
		m.log("> Stopping server...")
		_ = m.serverCmd.Process.Kill()
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
				m.appFunc("http://localhost:1920")
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

func (m *Manager) isServerRunning() bool {
	// Simple check: can we connect to port 1920?
	_, err := http.Get("http://localhost:1920/api/version")
	return err == nil
}

func (m *Manager) isServerReady() bool {
	resp, err := http.Get("http://localhost:1920/api/version")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
