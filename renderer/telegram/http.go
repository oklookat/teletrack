package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

func newTelegramHTTPClient(logger *slog.Logger) *http.Client {
	return &http.Client{
		Transport: telegramTransport(logger),
	}
}

// dialResult carries the outcome of a single dial attempt back to the
// coordinator goroutine.
type dialResult struct {
	network string // "ipv6" or "ipv4", for logging
	conn    net.Conn
	err     error
}

// telegramTransport returns an *http.Transport whose DialContext implements
// a Happy-Eyeballs-like race between IPv6 and IPv4:
//
//   - IPv6 is attempted first (Telegram is sometimes blocked on IPv4 in
//     some regions -- either by TCP RST or by a silent blackhole -- while
//     IPv6 still works).
//   - IPv4 is started shortly after (fallbackDelay), WITHOUT waiting for
//     the IPv6 attempt to finish. This bounds the extra latency we pay
//     when IPv6 itself turns out to be blackholed, instead of waiting out
//     a multi-second dial timeout before even trying IPv4.
//   - Whichever attempt succeeds first wins; the loser is cancelled, and
//     if its connection completes anyway it is closed immediately so we
//     don't leak a socket.
//   - Each individual attempt has its own hard timeout, independent of
//     whatever deadline (or lack of one) the caller's context carries.
//     This matters because a silently blackholed connection would
//     otherwise hang for as long as the parent context allows -- which
//     may be "forever" -- defeating the whole point of the fallback.
func telegramTransport(logger *slog.Logger) *http.Transport {
	dialer := &net.Dialer{}

	const (
		// attemptTimeout bounds a single dial attempt. This is the
		// "budget" for detecting a silent blackhole (infinite connect).
		// A TCP RST (connection reset) typically returns almost
		// immediately and won't hit this timeout at all.
		attemptTimeout = 2 * time.Second

		// fallbackDelay is how long IPv6 gets as a head start before
		// IPv4 is also started in parallel. This preserves "prefer
		// IPv6" while capping the worst-case extra latency to this
		// delay, rather than to the full attemptTimeout.
		fallbackDelay = 300 * time.Millisecond
	)

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,

		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			logSuccess := func(conn net.Conn, networkType string) {
				remoteAddr := conn.RemoteAddr().String()
				host, port, _ := net.SplitHostPort(remoteAddr)
				logger.InfoContext(ctx, "telegram connection established",
					slog.String("network", networkType),
					slog.String("remote_ip", host),
					slog.String("remote_port", port),
					slog.String("target_addr", addr),
				)
			}

			// Buffered so a "losing" goroutine can deliver its result
			// (or exit cleanly) without blocking after we've already
			// returned from DialContext.
			results := make(chan dialResult, 2)

			// raceCtx is cancelled as soon as we have a winner, so the
			// loser's in-flight dial is aborted promptly instead of
			// lingering in the background.
			raceCtx, cancelRace := context.WithCancel(ctx)
			defer cancelRace()

			attempt := func(netType, label string) {
				dialCtx, cancel := context.WithTimeout(raceCtx, attemptTimeout)
				defer cancel()
				conn, err := dialer.DialContext(dialCtx, netType, addr)
				results <- dialResult{network: label, conn: conn, err: err}
			}

			// 1) Start IPv6 immediately.
			go attempt("tcp6", "ipv6")

			v4Timer := time.NewTimer(fallbackDelay)
			defer v4Timer.Stop()

			var (
				v4Started      bool
				v6Done, v4Done bool
				v6Err, v4Err   error
			)

			startV4 := func() {
				if v4Started {
					return
				}
				v4Started = true
				v4Timer.Stop()
				go attempt("tcp4", "ipv4")
			}

			// bothFailed returns the combined error once both attempts
			// have been started AND both have finished unsuccessfully.
			bothFailed := func() error {
				if !v4Started || !v6Done || !v4Done {
					return nil
				}
				return fmt.Errorf("telegram connection failed: ipv6: %v; ipv4: %v", v6Err, v4Err)
			}

			for {
				select {
				case <-ctx.Done():
					// Caller gave up (e.g. request cancelled upstream).
					return nil, ctx.Err()

				case <-v4Timer.C:
					// IPv6 hasn't resolved (success or failure) within
					// fallbackDelay -- start racing IPv4 now instead of
					// waiting out IPv6's full attemptTimeout.
					startV4()

				case res := <-results:
					if res.err == nil {
						// Winner. Cancel the loser. Non-blocking drain so a
						// late connection (if any) is closed without
						// leaking a goroutine when the other attempt was
						// never started or already finished with an error.
						cancelRace()
						go func() {
							select {
							case other := <-results:
								if other.err == nil && other.conn != nil {
									_ = other.conn.Close()
								}
							case <-time.After(attemptTimeout + fallbackDelay):
							}
						}()
						logSuccess(res.conn, res.network)
						return res.conn, nil
					}

					// This attempt failed.
					switch res.network {
					case "ipv6":
						v6Done, v6Err = true, res.err
						// IPv6 failed before the fallback timer fired --
						// no reason to keep waiting on it, start IPv4
						// (if not already running) right away.
						startV4()
					case "ipv4":
						v4Done, v4Err = true, res.err
					}

					if err := bothFailed(); err != nil {
						logger.ErrorContext(ctx, "all connection attempts failed",
							slog.String("target_addr", addr),
							slog.Any("error", err),
						)
						return nil, err
					}
					logger.WarnContext(ctx, "connection attempt failed, still trying alternatives",
						slog.String("network", res.network),
						slog.Any("error", res.err),
						slog.String("target_addr", addr),
					)
				}
			}
		},

		ForceAttemptHTTP2: true,
	}
}
