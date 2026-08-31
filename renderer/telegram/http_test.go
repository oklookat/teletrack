// written by Claude
package telegram

import (
	"context"
	"net"
	"testing"
	"time"
)

type fakeConn struct{ net.Conn }

func TestRace_IPv6Blackhole_FallsBackToIPv4(t *testing.T) {
	// IPv6 never returns until ctx is cancelled (simulates infinite hang).
	dialV6 := func(ctx context.Context) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	// IPv4 succeeds quickly.
	dialV4 := func(ctx context.Context) (net.Conn, error) {
		time.Sleep(20 * time.Millisecond)
		return &fakeConn{}, nil
	}

	start := time.Now()
	conn, network, err := raceDial(context.Background(), dialV6, dialV4)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if network != "ipv4" {
		t.Fatalf("expected ipv4 to win, got %s", network)
	}
	if conn == nil {
		t.Fatalf("expected non-nil conn")
	}
	// Should win well before attemptTimeout (500ms), close to fallbackDelay+20ms.
	if elapsed > 250*time.Millisecond {
		t.Fatalf("fallback took too long: %v", elapsed)
	}
	t.Logf("won via %s in %v", network, elapsed)
}

func TestRace_IPv6ImmediateReset_FallsBackFast(t *testing.T) {
	// IPv6 fails immediately (simulates connection reset).
	dialV6 := func(ctx context.Context) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Err: net.ErrClosed}
	}
	dialV4 := func(ctx context.Context) (net.Conn, error) {
		time.Sleep(10 * time.Millisecond)
		return &fakeConn{}, nil
	}

	start := time.Now()
	_, network, err := raceDial(context.Background(), dialV6, dialV4)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if network != "ipv4" {
		t.Fatalf("expected ipv4 to win, got %s", network)
	}
	// Immediate reset should trigger instant fallback, not wait fallbackDelay.
	if elapsed > 100*time.Millisecond {
		t.Fatalf("expected fast fallback on immediate reset, took %v", elapsed)
	}
	t.Logf("won via %s in %v", network, elapsed)
}

func TestRace_IPv6Wins(t *testing.T) {
	dialV6 := func(ctx context.Context) (net.Conn, error) {
		time.Sleep(5 * time.Millisecond)
		return &fakeConn{}, nil
	}
	dialV4 := func(ctx context.Context) (net.Conn, error) {
		time.Sleep(200 * time.Millisecond)
		return &fakeConn{}, nil
	}

	_, network, err := raceDial(context.Background(), dialV6, dialV4)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if network != "ipv6" {
		t.Fatalf("expected ipv6 to win, got %s", network)
	}
}

func TestRace_BothFail(t *testing.T) {
	dialV6 := func(ctx context.Context) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Err: net.ErrClosed}
	}
	dialV4 := func(ctx context.Context) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Err: net.ErrClosed}
	}

	_, _, err := raceDial(context.Background(), dialV6, dialV4)
	if err == nil {
		t.Fatalf("expected error when both fail")
	}
}

// Standalone reimplementation of just the race/state-machine logic against
// fake dial functions, so we can simulate blackhole/reset/success without
// real sockets. Mirrors the logic in dial.go's DialContext closure.
func raceDial(ctx context.Context, dialV6, dialV4 func(context.Context) (net.Conn, error)) (net.Conn, string, error) {
	const (
		attemptTimeout = 500 * time.Millisecond
		fallbackDelay  = 100 * time.Millisecond
	)

	type result struct {
		network string
		conn    net.Conn
		err     error
	}
	results := make(chan result, 2)
	raceCtx, cancelRace := context.WithCancel(ctx)
	defer cancelRace()

	attempt := func(label string, fn func(context.Context) (net.Conn, error)) {
		dialCtx, cancel := context.WithTimeout(raceCtx, attemptTimeout)
		defer cancel()
		conn, err := fn(dialCtx)
		results <- result{network: label, conn: conn, err: err}
	}

	go attempt("ipv6", dialV6)
	v4Timer := time.NewTimer(fallbackDelay)
	defer v4Timer.Stop()

	var v4Started, v6Done, v4Done bool
	var v6Err, v4Err error

	startV4 := func() {
		if v4Started {
			return
		}
		v4Started = true
		v4Timer.Stop()
		go attempt("ipv4", dialV4)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-v4Timer.C:
			startV4()
		case res := <-results:
			if res.err == nil {
				cancelRace()
				return res.conn, res.network, nil
			}
			switch res.network {
			case "ipv6":
				v6Done, v6Err = true, res.err
				startV4()
			case "ipv4":
				v4Done, v4Err = true, res.err
			}
			if v4Started && v6Done && v4Done {
				_ = v6Err
				_ = v4Err
				return nil, "", context.DeadlineExceeded
			}
		}
	}
}
