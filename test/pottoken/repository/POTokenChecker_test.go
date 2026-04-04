package repository_test

import (
	"net"
	"testing"
	"time"

	potokenRepo "tg-downloader/src/features/pottoken/data/repository"
	"tg-downloader/src/core/logger"
)

// mockLoggerStrategy captures log calls for assertion.
type mockLoggerStrategy struct {
	infoCalls []string
	warnCalls []string
	debugCalls []string
	errorCalls []string
}

func (m *mockLoggerStrategy) Debug(msg string) { m.debugCalls = append(m.debugCalls, msg) }
func (m *mockLoggerStrategy) Info(msg string)  { m.infoCalls = append(m.infoCalls, msg) }
func (m *mockLoggerStrategy) Warn(msg string)  { m.warnCalls = append(m.warnCalls, msg) }
func (m *mockLoggerStrategy) Error(msg string) { m.errorCalls = append(m.errorCalls, msg) }

// startTestServer starts a TCP listener on a random port and returns its address and a cleanup func.
func startTestServer(t *testing.T) (addr string, cleanup func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	return ln.Addr().String(), func() { ln.Close() }
}

// newMockLogger creates a Logger backed by the given mock strategy.
func newMockLogger(mock *mockLoggerStrategy) *logger.Logger {
	return logger.NewLogger([]logger.ILoggerStrategy{mock})
}

// ---- TCP Reachability Tests ----

// TestTCPReachable_ListeningPort verifies that TcpReachable returns true when a TCP server is listening.
func TestTCPReachable_ListeningPort(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	result := potokenRepo.TcpReachable("http://"+addr, 2*time.Second)
	if !result {
		t.Errorf("expected true for listening port %s, got false", addr)
	}
}

// TestTCPReachable_ClosedPort verifies that TcpReachable returns false for a closed port.
func TestTCPReachable_ClosedPort(t *testing.T) {
	result := potokenRepo.TcpReachable("http://127.0.0.1:1", 1*time.Second)
	if result {
		t.Error("expected false for closed port 1, got true")
	}
}

// TestTCPReachable_MalformedURL verifies that TcpReachable returns false for a malformed URL.
func TestTCPReachable_MalformedURL(t *testing.T) {
	result := potokenRepo.TcpReachable("not-a-url", 1*time.Second)
	if result {
		t.Error("expected false for malformed URL, got true")
	}
}

// TestTCPReachable_EmptyHost verifies that TcpReachable returns false when the URL has no host.
func TestTCPReachable_EmptyHost(t *testing.T) {
	result := potokenRepo.TcpReachable("http://", 1*time.Second)
	if result {
		t.Error("expected false for empty host, got true")
	}
}

// ---- Checker Disabled Tests ----

// TestCheckerDisabledWhenURLEmpty verifies that a checker with nil/empty providerUrl is disabled.
func TestCheckerDisabledWhenURLEmpty(t *testing.T) {
	mock := &mockLoggerStrategy{}
	log := newMockLogger(mock)

	checker := potokenRepo.NewPOTokenCheckerForTest("", 900, log)

	if checker.IsReachable() {
		t.Error("expected IsReachable() == false for empty providerUrl, got true")
	}
	if checker.ProviderURL() != "" {
		t.Errorf("expected ProviderURL() == \"\", got %q", checker.ProviderURL())
	}
}

// ---- Startup Logging Tests ----

// TestCheckerStartupLogOnline verifies that startup check logs Info "online" when server is reachable.
func TestCheckerStartupLogOnline(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	mock := &mockLoggerStrategy{}
	log := newMockLogger(mock)

	checker := potokenRepo.NewPOTokenCheckerForTest("http://"+addr, 900, log)
	checker.CheckNow(true)

	found := false
	for _, msg := range mock.infoCalls {
		if contains(msg, "PO Token server: online") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Info log containing 'PO Token server: online', got infoCalls=%v", mock.infoCalls)
	}
}

// TestCheckerStartupLogUnreachable verifies that startup check logs Warn "unreachable" when server is down.
func TestCheckerStartupLogUnreachable(t *testing.T) {
	mock := &mockLoggerStrategy{}
	log := newMockLogger(mock)

	checker := potokenRepo.NewPOTokenCheckerForTest("http://127.0.0.1:1", 900, log)
	checker.CheckNow(true)

	found := false
	for _, msg := range mock.warnCalls {
		if contains(msg, "PO Token server: unreachable") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Warn log containing 'PO Token server: unreachable', got warnCalls=%v", mock.warnCalls)
	}
}

// ---- State Change Logging Tests ----

// TestStateChangeLogging_DownToUp verifies that Info "back online" is logged when state transitions down->up.
func TestStateChangeLogging_DownToUp(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	mock := &mockLoggerStrategy{}
	log := newMockLogger(mock)

	checker := potokenRepo.NewPOTokenCheckerForTest("http://"+addr, 900, log)
	// Checker starts with reachable=false (zero value). Server is up: down->up transition.
	checker.CheckNow(false)

	found := false
	for _, msg := range mock.infoCalls {
		if contains(msg, "back online") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Info log containing 'back online', got infoCalls=%v", mock.infoCalls)
	}
}

// TestStateChangeLogging_UpToDown verifies that Warn "went unreachable" is logged when state transitions up->down.
func TestStateChangeLogging_UpToDown(t *testing.T) {
	mock := &mockLoggerStrategy{}
	log := newMockLogger(mock)

	checker := potokenRepo.NewPOTokenCheckerForTest("http://127.0.0.1:1", 900, log)
	// Pre-set state to reachable=true so the periodic check sees a down transition.
	checker.SetReachable(true)
	checker.CheckNow(false)

	found := false
	for _, msg := range mock.warnCalls {
		if contains(msg, "went unreachable") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Warn log containing 'went unreachable', got warnCalls=%v", mock.warnCalls)
	}
}

// TestStateChangeLogging_StableNoLog verifies that no log is emitted when state is stable (no transition).
func TestStateChangeLogging_StableNoLog(t *testing.T) {
	mock := &mockLoggerStrategy{}
	log := newMockLogger(mock)

	checker := potokenRepo.NewPOTokenCheckerForTest("http://127.0.0.1:1", 900, log)
	// Both prev and now are false (unreachable) — no state transition.
	checker.CheckNow(false)

	if len(mock.infoCalls) != 0 || len(mock.warnCalls) != 0 {
		t.Errorf("expected no log calls for stable state, got infoCalls=%v warnCalls=%v",
			mock.infoCalls, mock.warnCalls)
	}
}

// contains is a simple string substring check helper.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
