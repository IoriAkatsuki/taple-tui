package applemusic

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Exercise the real private-pipe transport without a browser or user profile.
func deadlineTestBrowser(t *testing.T, handle func(string, <-chan struct{}) (any, bool)) *Browser {
	t.Helper()
	serverIn, appOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	appIn, serverOut, err := os.Pipe()
	if err != nil {
		serverIn.Close()
		appOut.Close()
		t.Fatal(err)
	}
	c := newPipe(appIn, appOut)
	b := &Browser{pipe: c, background: true, target: "page", session: "session", exited: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		reader := bufio.NewReader(serverIn)
		for {
			raw, err := reader.ReadBytes(0)
			if err != nil {
				return
			}
			var request struct {
				ID     int
				Method string
			}
			if err := json.Unmarshal(raw[:len(raw)-1], &request); err != nil {
				t.Error(err)
				return
			}
			result, respond := handle(request.Method, c.done)
			if !respond {
				continue
			}
			raw, err = json.Marshal(map[string]any{"id": request.ID, "result": result})
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := serverOut.Write(append(raw, 0)); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		c.close()
		serverIn.Close()
		serverOut.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("private-pipe test peer did not stop")
		}
	})
	return b
}

func deadlineTestReply(method string) any {
	switch method {
	case "Target.getTargets":
		return map[string]any{"targetInfos": []any{map[string]string{"targetId": "page", "type": "page", "url": "https://music.apple.com/us/browse"}}}
	case "Target.attachToTarget":
		return map[string]string{"sessionId": "session"}
	case "Runtime.evaluate":
		return map[string]any{"result": map[string]any{"value": map[string]any{"data": map[string]any{"ready": true, "authorized": true}}}}
	}
	return map[string]any{}
}

func TestSnapshotDeadlineWhileSessionDiscoveryIsBusy(t *testing.T) {
	for _, method := range []string{"Target.getTargets", "Target.attachToTarget"} {
		for _, cancelEarly := range []bool{false, true} {
			name := method + "/deadline"
			if cancelEarly {
				name = method + "/canceled"
			}
			t.Run(name, func(t *testing.T) {
				seen, release := make(chan struct{}), make(chan struct{})
				releaseHolder := sync.OnceFunc(func() { close(release) })
				held := false
				var discoveries atomic.Int32
				b := deadlineTestBrowser(t, func(request string, done <-chan struct{}) (any, bool) {
					if request == "Target.getTargets" {
						discoveries.Add(1)
					}
					if request == method && !held {
						held = true
						close(seen)
						select {
						case <-release:
						case <-done:
							return nil, false
						}
					}
					return deadlineTestReply(request), true
				})
				t.Cleanup(releaseHolder)
				if method == "Target.attachToTarget" {
					b.target, b.session = "", ""
				}
				holderCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				holderDone := make(chan error, 1)
				go func() { _, err := b.Snapshot(holderCtx); holderDone <- err }()
				select {
				case <-seen:
				case <-time.After(time.Second):
					t.Fatal("session discovery did not start")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
				defer cancel()
				result := make(chan error, 1)
				started := time.Now()
				go func() { _, err := b.Snapshot(ctx); result <- err }()
				want := context.DeadlineExceeded
				if cancelEarly {
					cancel()
					want = context.Canceled
				}
				select {
				case err := <-result:
					t.Logf("Snapshot returned before discovery release after %s: %v", time.Since(started), err)
					if !errors.Is(err, want) {
						t.Errorf("got %v, want %v", err, want)
					}
				case <-time.After(600 * time.Millisecond):
					t.Error("Snapshot ignored cancellation while waiting for session discovery")
					releaseHolder()
					select {
					case <-result:
					case <-time.After(time.Second):
						t.Fatal("canceled Snapshot did not return after discovery release")
					}
				}
				releaseHolder()
				if err := <-holderDone; err != nil {
					t.Fatalf("holder failed: %v", err)
				}
				if s, err := b.Snapshot(holderCtx); err != nil || !s.Ready {
					t.Fatalf("canceled waiter leaked session ownership: ready=%v err=%v", s.Ready, err)
				}
				if got := discoveries.Load(); got != 2 {
					t.Errorf("expired waiter sent a discovery request: got %d, want 2", got)
				}
			})
		}
	}
}

func TestSnapshotSlowRuntimeDoesNotHoldSessionOwnership(t *testing.T) {
	seen := make(chan struct{})
	held := false
	b := deadlineTestBrowser(t, func(method string, _ <-chan struct{}) (any, bool) {
		if method == "Runtime.evaluate" && !held {
			held = true
			close(seen)
			return nil, false
		}
		return deadlineTestReply(method), true
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := b.Snapshot(ctx); done <- err }()
	defer func() { cancel(); <-done }()
	select {
	case <-seen:
	case <-time.After(time.Second):
		t.Fatal("runtime evaluation did not start")
	}
	readCtx, stop := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer stop()
	if s, err := b.Snapshot(readCtx); err != nil || !s.Ready {
		t.Fatalf("independent Snapshot blocked on runtime evaluation: ready=%v err=%v", s.Ready, err)
	}
}
