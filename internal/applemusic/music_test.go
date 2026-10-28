package applemusic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestAppleOrigin(t *testing.T) {
	for raw, want := range map[string]bool{
		"https://music.apple.com/us/browse":     true,
		"http://music.apple.com/":               false,
		"https://music.apple.com.evil.example/": false,
		"https://music.apple.com@evil.example/": false,
		"https://user@music.apple.com/":         false,
		"https://account.apple.com/":            false,
	} {
		if appleOrigin(raw) != want {
			t.Errorf("origin %q", raw)
		}
	}
}

func runBridge(t *testing.T, setup string, request any) []byte {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is needed to test the embedded JavaScript")
	}
	script, err := scriptFor(request)
	if err != nil {
		t.Fatal(err)
	}
	// The process budget includes cold Windows startup; JS timeout assertions are independent.
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--input-type=module", "-")
	const started = "tapefox-fixture-started\n"
	marker, _ := json.Marshal(started)
	cmd.Stdin = strings.NewReader("process.stderr.write(" + string(marker) + ");" + setup + "; console.log(JSON.stringify(await " + script + "));")
	output, err := cmd.CombinedOutput()
	fixtureStarted := bytes.Contains(output, []byte(started))
	output = bytes.Replace(output, []byte(started), nil, 1)
	if err != nil {
		t.Fatalf("bridge: %v (context: %v, fixture_started=%v): %s", err, ctx.Err(), fixtureStarted, output)
	}
	return output
}

func TestBridgeHarnessAcceptsLargeESModule(t *testing.T) {
	setup := `import assert from 'node:assert/strict';
await Promise.resolve();
globalThis.location={origin:'https://not-apple.example'};
const padding='` + strings.Repeat("x", 160<<10) + `';
assert.equal(padding.length, 163840);`
	output := runBridge(t, setup, map[string]string{"op": "snapshot"})
	var got struct{ Error string }
	if err := json.Unmarshal(output, &got); err != nil || got.Error != "origin" {
		t.Fatalf("large ES-module bridge fixture did not complete: %s %v", output, err)
	}
}

func TestBridgeSnapshotDoesNotExportCredentials(t *testing.T) {
	output := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({isAuthorized:true,isPlaying:true,previewOnly:false,
musicUserToken:'DO_NOT_EXPORT',developerToken:'DO_NOT_EXPORT',currentPlaybackTime:12,volume:.5,
nowPlayingItem:{id:'i.string-id',attributes:{name:'歌名',artistName:'Artist',durationInMillis:90000}},queue:{items:[]}})}`, map[string]string{"op": "snapshot"})
	if strings.Contains(string(output), "DO_NOT_EXPORT") {
		t.Fatal("credential leaked")
	}
	var got struct{ Data State }
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Data.Ready || !got.Data.Authorized || got.Data.Track.ID != "i.string-id" || got.Data.Position != 12 {
		t.Fatalf("state: %+v", got.Data)
	}
}

func TestBridgeDetectsPreviewFallbackAndActualDuration(t *testing.T) {
	output := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={PlaybackType:{preview:'preview'},getInstance:()=>({isAuthorized:true,isPlaying:true,previewOnly:false,
currentPlaybackDuration:30,nowPlayingItem:{id:'123',playbackType:'preview',attributes:{durationInMillis:240000}},queue:{items:[]}})}`,
		map[string]string{"op": "snapshot"})
	var got struct{ Data State }
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Data.Preview || got.Data.Full || got.Data.Track.Duration != 30 {
		t.Fatalf("preview fallback misreported: %+v", got.Data)
	}
}

func TestBridgeRequiresExplicitFullPlaybackType(t *testing.T) {
	for _, playbackType := range []string{"0", "1", "2", "3"} {
		output := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={PlaybackType:{none:0,preview:1,unencryptedFull:2,encryptedFull:3},getInstance:()=>({isAuthorized:true,isPlaying:true,
nowPlayingItem:{id:'123',playbackType:`+playbackType+`},queue:{items:[]}})}`, map[string]string{"op": "snapshot"})
		var got struct{ Data State }
		if err := json.Unmarshal(output, &got); err != nil {
			t.Fatal(err)
		}
		if got.Data.Full != (playbackType == "2" || playbackType == "3") {
			t.Fatalf("playbackType=%s full=%t", playbackType, got.Data.Full)
		}
	}
}

func TestBridgePausedRequiresExplicitPlaybackState(t *testing.T) {
	for _, state := range []string{"undefined", "'loading'", "'stopped'", "'paused'"} {
		out := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={PlaybackStates:{paused:'paused'},getInstance:()=>({playbackState:`+state+`,isPlaying:false})}`, map[string]string{"op": "snapshot"})
		var got struct{ Data struct{ Paused bool } }
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}
		if got.Data.Paused != (state == "'paused'") {
			t.Fatalf("state %s: paused=%v", state, got.Data.Paused)
		}
	}
	out := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};globalThis.MusicKit={getInstance:()=>({isPlaying:false})}`, map[string]string{"op": "snapshot"})
	var got struct{ Data struct{ Paused bool } }
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Data.Paused {
		t.Fatal("missing enum fabricated a paused state")
	}
}

func TestBridgeRejectsSeekForAnotherTrack(t *testing.T) {
	out := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({isAuthorized:true,nowPlayingItem:{id:'new'},seekToTime:async()=>{throw Error('should not seek')}})}`, Action{Kind: "seek", ID: "old", Value: 30})
	var got struct{ Error string }
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Error != "stale" {
		t.Fatalf("stale seek reached the player: %s", out)
	}
}

func TestBridgeGuardsOriginAndEncodesSearch(t *testing.T) {
	output := runBridge(t, `globalThis.location={origin:'https://evil.example'}`, map[string]string{"op": "snapshot"})
	if !strings.Contains(string(output), `"error":"origin"`) {
		t.Fatalf("unguarded: %s", output)
	}
	query := `"; throw new Error('injected'); //`
	output = runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({storefrontId:'us',api:{music:async(path,args)=>({data:{results:{songs:{data:[{id:'123',type:'songs',attributes:{name:args.term}}]}}}})}})}`, map[string]string{"op": "catalog", "kind": "songs", "query": query})
	var got struct{ Data CatalogPage }
	if err := json.Unmarshal(output, &got); err != nil || len(got.Data.Items) != 1 || got.Data.Items[0].Name != query {
		t.Fatalf("search: %s, %v", output, err)
	}
}

func TestBridgeReportsLoginAndPlaybackErrors(t *testing.T) {
	for _, setup := range []string{`({isAuthorized:false})`, `({isAuthorized:true,play:async()=>{throw new Error('private details')}})`} {
		output := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};globalThis.MusicKit={getInstance:()=>`+setup+`}`, map[string]string{"op": "toggle"})
		if !strings.Contains(string(output), `"error":`) || strings.Contains(string(output), "private details") {
			t.Fatalf("error: %s", output)
		}
	}
}

func TestControlRejectsInvalidArgumentsBeforeBrowserIO(t *testing.T) {
	for _, a := range []Action{{Kind: "eval"}, {Kind: "play"}, {Kind: "seek", Value: -1}, {Kind: "volume", Value: 2}} {
		if err := (&Browser{}).Do(context.Background(), a); err == nil {
			t.Errorf("accepted %+v", a)
		}
	}
}

func TestControlKeepsZeroSeekVolumeAndQueueIndex(t *testing.T) {
	for _, kind := range []string{"seek", "volume", "jump"} {
		data, err := json.Marshal(Action{Kind: kind, ID: "song", Value: 0})
		if err != nil {
			t.Fatal(err)
		}
		var request map[string]any
		if err := json.Unmarshal(data, &request); err != nil {
			t.Fatal(err)
		}
		if value, ok := request["value"]; !ok || value != float64(0) {
			t.Fatalf("zero %s argument disappeared: %s", kind, data)
		}
	}
}

func TestPlaybackWaitsForLoginHandover(t *testing.T) {
	in, writer := io.Pipe()
	b := &Browser{pipe: newPipe(in, nopWriteCloser{io.Discard}), background: false}
	defer b.pipe.close()
	defer writer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := b.Do(ctx, Action{Kind: "toggle"}); err == nil || !strings.Contains(err.Error(), "登录") {
		t.Fatalf("login transition should block transport before RPC: %v", err)
	}
}

func TestChromeStartsWithoutVisibleWindow(t *testing.T) {
	if os.Getenv("TAPEFOX_CHROME_HIDDEN") != "1" {
		t.Skip("opt-in unified Chromium headless check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	binary, err := findChrome()
	if err != nil {
		t.Fatal(err)
	}
	b, err := launchBrowser(ctx, binary, t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if !b.background || !slices.Contains(b.cmd.Args, "--headless=new") || slices.Contains(b.cmd.Args, "--start-minimized") {
		t.Fatal("startup must use unified headless, not minimized desktop windows")
	}
	t.Log("Chromium uses unified headless; DRM playback is tested separately")
}

// This opt-in check uses the user's dedicated, already authorized profile.
// It changes only the web player's queue/transport, never library contents.
func TestAppleMusicBrowserIntegration(t *testing.T) {
	if os.Getenv("TAPEFOX_APPLEMUSIC_LIVE") != "1" {
		t.Skip("requires user-assisted login and permission to exercise playback")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	b, err := Open(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	wait := func(label string, check func(State) bool) State {
		t.Helper()
		deadline := time.Now().Add(35 * time.Second)
		var last State
		var lastErr error
		for time.Now().Before(deadline) && ctx.Err() == nil {
			last, lastErr = b.Snapshot(ctx)
			if lastErr == nil && check(last) {
				return last
			}
			time.Sleep(500 * time.Millisecond)
		}
		t.Fatalf("%s: ready=%t authorized=%t playing=%t preview=%t full=%t position=%.1f duration=%.1f err=%v", label, last.Ready, last.Authorized, last.Playing, last.Preview, last.Full, last.Position, last.Track.Duration, lastErr)
		return last
	}
	initial := wait("authorized browser", func(s State) bool { return s.Ready && s.Authorized })
	defer func() { _ = b.Do(context.Background(), Action{Kind: "volume", Value: initial.Volume}) }()
	page, err := b.Catalog(ctx, CatalogRequest{Kind: CatalogSongs, Query: "Daft Punk"})
	tracks := page.Items
	if err != nil || len(tracks) < 2 {
		t.Fatalf("catalog search: count=%d err=%v", len(tracks), err)
	}
	t.Logf("catalog search returned %d tracks", len(tracks))
	if err := b.Do(ctx, Action{Kind: "play", ID: tracks[0].ID}); err != nil {
		t.Logf("play acknowledgement: %v", err)
	}
	s := wait("full track advancing", func(s State) bool {
		return s.Track.ID == tracks[0].ID && s.Playing && s.Full && !s.Preview && s.Position > 2 && s.Track.Duration > 60
	})
	t.Logf("authorized=%t playing=%t preview=%t full=%t background=%t duration=%.1fs position=%.1fs", s.Authorized, s.Playing, s.Preview, s.Full, b.background, s.Track.Duration, s.Position)
	if err := b.Do(ctx, Action{Kind: "toggle"}); err != nil {
		t.Logf("pause acknowledgement: %v", err)
	}
	wait("pause", func(s State) bool { return !s.Playing })
	if err := b.Do(ctx, Action{Kind: "seek", ID: tracks[0].ID, Value: 45}); err != nil {
		t.Fatal(err)
	}
	wait("seek", func(s State) bool { return s.Position >= 44 && s.Position <= 47 })
	if err := b.Do(ctx, Action{Kind: "volume", Value: .35}); err != nil {
		t.Fatal(err)
	}
	wait("volume", func(s State) bool { return s.Volume >= .34 && s.Volume <= .36 })
	if err := b.Do(ctx, Action{Kind: "toggle"}); err != nil {
		t.Logf("resume acknowledgement: %v", err)
	}
	wait("resume", func(s State) bool { return s.Playing && s.Position > 45 })
	if err := b.Do(ctx, Action{Kind: "enqueue", ID: tracks[1].ID}); err != nil {
		t.Fatal(err)
	}
	wait("enqueue", func(s State) bool { return s.QueueTotal >= 2 })
	if err := b.Do(ctx, Action{Kind: "next"}); err != nil {
		t.Logf("next acknowledgement: %v", err)
	}
	wait("next track", func(s State) bool { return s.Track.ID == tracks[1].ID && s.Playing && s.Full && !s.Preview })
	queueState, err := b.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	jumpIndex := -1
	for i, item := range queueState.Queue {
		if item.ID == tracks[0].ID {
			jumpIndex = i
			break
		}
	}
	if jumpIndex < 0 {
		t.Fatalf("original song not found in reported queue: %+v", queueState.Queue)
	}
	t.Logf("queue click target index=%d", jumpIndex)
	if err := b.Do(ctx, Action{Kind: "jump", ID: tracks[0].ID, Value: float64(jumpIndex)}); err != nil {
		t.Logf("jump acknowledgement: %v", err)
	}
	wait("queue jump", func(s State) bool { return s.Track.ID == tracks[0].ID && s.Playing && s.Full })
	t.Log("pause, seek, volume, resume, enqueue and next track verified against the real browser")
}

func TestAppleMusicLoginHandover(t *testing.T) {
	if os.Getenv("TAPEFOX_APPLEMUSIC_LOGIN") != "1" {
		t.Skip("user-assisted login handover")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	b, err := Open(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	shown := false
	for ctx.Err() == nil {
		s, err := b.Snapshot(ctx)
		if err == nil && s.Ready && !s.Authorized && !shown {
			if err = b.Show(ctx); err != nil {
				t.Fatal(err)
			}
			shown = true
			t.Log("LOGIN WINDOW OPEN: please complete Apple login; no credentials are read by the test")
		}
		if err == nil && s.Ready && s.Authorized && b.background {
			t.Log("authorization restored in windowless playback mode")
			b.Close()
			reopened, err := Open(ctx, "")
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			for deadline := time.Now().Add(35 * time.Second); time.Now().Before(deadline); {
				state, err := reopened.Snapshot(ctx)
				if err == nil && state.Authorized && reopened.background {
					t.Log("persistent login survived another windowless restart")
					return
				}
				time.Sleep(500 * time.Millisecond)
			}
			t.Fatal("login did not survive windowless restart")
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("login handover timed out")
}
