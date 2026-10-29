package applemusic

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCookieImportUsesPrivatePipeAndDoesNotReplaceAuthorizedSession(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		authorized, failWrite, failReload bool
		want                              []string
	}{
		{"accepted", false, false, false, []string{"Storage.setCookies", "Page.reload"}},
		{"authorized", true, false, false, nil},
		{"failed write", false, true, false, []string{"Storage.setCookies"}},
		{"failed reload", false, false, true, []string{"Storage.setCookies", "Page.reload"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, serverOut := io.Pipe()
			serverIn, output := io.Pipe()
			p := newPipe(input, output)
			defer p.close()
			defer serverIn.Close()
			defer serverOut.Close()
			var mu sync.Mutex
			var methods []string
			go func() {
				reader := bufio.NewReader(serverIn)
				for {
					data, err := reader.ReadBytes(0)
					if err != nil {
						return
					}
					var req struct {
						ID     int
						Method string
						Params json.RawMessage
					}
					if json.Unmarshal(data[:len(data)-1], &req) != nil {
						return
					}
					var result any = map[string]any{}
					switch req.Method {
					case "Target.getTargets":
						result = map[string]any{"targetInfos": []map[string]string{{"type": "page", "url": "https://music.apple.com/", "targetId": "page"}}}
					case "Target.attachToTarget":
						result = map[string]string{"sessionId": "session"}
					case "Runtime.evaluate":
						result = map[string]any{"result": map[string]any{"value": map[string]any{"data": State{Ready: true, Authorized: tc.authorized}}}}
					default:
						mu.Lock()
						methods = append(methods, req.Method)
						mu.Unlock()
					}
					reply := map[string]any{"id": req.ID, "result": result}
					if (tc.failWrite && req.Method == "Storage.setCookies") || (tc.failReload && req.Method == "Page.reload") {
						delete(reply, "result")
						reply["error"] = map[string]any{"code": -32602, "message": "DO NOT PRINT fixture-only"}
					}
					body, _ := json.Marshal(reply)
					if _, err := serverOut.Write(append(body, 0)); err != nil {
						return
					}
				}
			}()
			jar, err := parseCookies([]byte(`[{"domain":"music.apple.com","name":"synthetic","value":"fixture-only","secure":true}]`), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			b := &Browser{pipe: p, background: true}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err = b.ImportCookies(ctx, jar)
			if (err != nil) != (tc.authorized || tc.failWrite || tc.failReload) {
				t.Fatalf("unexpected result: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "fixture-only") {
				t.Fatal("protocol failure leaked cookie data")
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(methods, tc.want) {
				t.Fatalf("unexpected mutation sequence: %v", methods)
			}
		})
	}
}

func TestCookieImportRejectsEmptyJarWithoutBrowserIO(t *testing.T) {
	if err := (&Browser{}).ImportCookies(context.Background(), CookieJar{}); err == nil {
		t.Fatal("empty jar accepted")
	}
}

func TestCookieImportSyntheticBrowserSmoke(t *testing.T) {
	if os.Getenv("TAPLE_COOKIE_SMOKE") != "1" {
		t.Skip("opt-in clean-profile synthetic cookie test; no user account")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	b, err := Open(ctx, filepath.Join(t.TempDir(), "chrome"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	jar, err := parseCookies([]byte(`[{"domain":"music.apple.com","hostOnly":true,"name":"taple-synthetic-host","value":"fixture-only","secure":true,"httpOnly":true,"sameSite":"Strict"},{"domain":".music.apple.com","hostOnly":false,"name":"taple-synthetic-domain","value":"fixture-only","secure":true,"sameSite":"Lax","expires":2000000000}]`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.ImportCookies(ctx, jar); err != nil {
		t.Fatal(err)
	}
	var received struct {
		Cookies []struct {
			Name, Domain, SameSite    string
			Secure, HTTPOnly, Session bool
		}
	}
	if err = b.pipe.call(ctx, "", "Storage.getCookies", nil, &received); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, c := range received.Cookies {
		switch c.Name {
		case "taple-synthetic-host":
			if c.Domain != "music.apple.com" || !c.HTTPOnly || !c.Secure || !c.Session || c.SameSite != "Strict" {
				t.Fatal("host-only/session/security attributes changed")
			}
			found++
		case "taple-synthetic-domain":
			if c.Domain != ".music.apple.com" || !c.Secure || c.Session || c.SameSite != "Lax" {
				t.Fatal("domain/persistent attributes changed")
			}
			found++
		}
	}
	if found != 2 {
		t.Fatal("synthetic cookies were not both accepted")
	}
	t.Log("Synthetic cookies accepted with preserved scope; no account authorization or playback was tested")
}
