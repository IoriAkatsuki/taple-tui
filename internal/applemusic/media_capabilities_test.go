package applemusic

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Requires an explicitly selected, closed, user-authorized session. This only
// reads snapshot/capabilities; it never calls Do, Show, logout, or profile cleanup.
func TestExistingSessionReadOnlyMediaDiagnosis(t *testing.T) {
	if os.Getenv("TAPEFOX_APPLEMUSIC_DIAGNOSE") != "1" {
		t.Skip("requires explicit consent for read-only session diagnosis")
	}
	profile := os.Getenv("TAPEFOX_DIAGNOSTIC_PROFILE")
	info, err := os.Stat(profile)
	if err != nil || !filepath.IsAbs(profile) || !info.IsDir() {
		t.Fatal("an existing absolute diagnostic profile is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	b, err := Open(ctx, profile)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var state State
	until := time.Now().Add(25 * time.Second)
	for time.Now().Before(until) {
		state, err = b.Snapshot(ctx)
		if err == nil && state.Ready {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
	t.Logf("read-only state: ready=%v authorized=%v playing=%v paused=%v full=%v preview=%v position=%.1f queue=%d error=%v", state.Ready, state.Authorized, state.Playing, state.Paused, state.Full, state.Preview, state.Position, state.QueueTotal, err)
	var caps mediaCapabilities
	if err = b.evaluate(ctx, mediaCapabilityRequest{}, &caps); err != nil {
		t.Fatal(err)
	}
	t.Logf("actual session capabilities: AAC=%v Widevine=%v reason=%s; no queue or transport action sent", caps.AAC, caps.Widevine, caps.Reason)
}

// Read-only, clean-profile capability test on the actual service origin. No
// account is used, no playback is requested, and no page/session data is logged.
func TestAppleMusicOriginMediaCapabilities(t *testing.T) {
	if os.Getenv("TAPEFOX_MEDIA_PREFLIGHT_SMOKE") != "1" {
		t.Skip("opt-in, account-free capability probe on music.apple.com")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	b, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var caps mediaCapabilities
	for {
		err = b.evaluate(ctx, mediaCapabilityRequest{}, &caps)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("could not reach official service origin: %v", err)
		case <-time.After(500 * time.Millisecond):
		}
	}
	t.Logf("music.apple.com headless capability only: AAC=%v Widevine=%v reason=%s (no account or playback)", caps.AAC, caps.Widevine, caps.Reason)
	if os.Getenv("TAPEFOX_REQUIRE_WIDEVINE") == "1" && (!caps.AAC || !caps.Widevine) {
		t.Fatal("AAC/Widevine preflight failed")
	}
}

func TestMediaCapabilitiesCacheAndSafeErrors(t *testing.T) {
	for _, outcome := range []string{"ok", "unsupported", "codec", "eme", "timeout"} {
		t.Run(outcome, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};globalThis.isSecureContext=true;
let calls=0,now=0;Date.now=()=>now;
globalThis.document={createElement:()=>({canPlayType:()=> '`+outcome+`'==='codec'?'':'probably'})};
Object.defineProperty(globalThis,'navigator',{configurable:true,value:{requestMediaKeySystemAccess:async(system,options)=>{
  calls++;assert.equal(system,'com.widevine.alpha');assert.equal(options[0].audioCapabilities[0].contentType,'audio/mp4; codecs="mp4a.40.2"');
  if('`+outcome+`'==='unsupported')throw Object.assign(Error('PRIVATE SESSION'),{name:'NotSupportedError'});
  if('`+outcome+`'==='timeout')return new Promise(()=>{});
  return {};
}}});
if('`+outcome+`'==='eme')delete navigator.requestMediaKeySystemAccess;
if('`+outcome+`'==='timeout')globalThis.setTimeout=(cb)=>{queueMicrotask(cb);return 1};
const probe=`+mediaCapabilitiesJS+`;
const first=await probe({});assert.equal(first.data.widevine,'`+outcome+`'==='ok');
assert.equal(JSON.stringify(first).includes('PRIVATE'),false);
const count=calls;await probe({});assert.equal(calls,count,'reuse completed capability negotiation');
now=60001;await probe({});assert.equal(calls,count*2,'expired results must be renegotiated');
`, mediaCapabilityRequest{})
		})
	}
}

func TestMediaCapabilitiesNeverBlocksPauseOrProbesBeforeLogin(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const mk={isPlaying:true,isAuthorized:true};globalThis.MusicKit={getInstance:()=>mk};
const probe=`+mediaCapabilitiesJS+`;
assert.deepEqual(await probe({skipIfPlaying:true,requireAuthorization:true}),{data:{playing:true}});
mk.isAuthorized=false;assert.deepEqual(await probe({requireAuthorization:true}),{error:'login'});
globalThis.location.origin='https://account.apple.com';assert.deepEqual(await probe({}),{error:'origin'});
`, mediaCapabilityRequest{})
}

func TestPlaybackPreflightPreventsUnsupportedQueueSubmission(t *testing.T) {
	for _, caps := range []mediaCapabilities{{AAC: true, Reason: "widevine"}, {Reason: "codec"}, {AAC: true, Reason: "timeout"}, {AAC: true, Widevine: true}} {
		input, serverOut := io.Pipe()
		serverIn, output := io.Pipe()
		p := newPipe(input, output)
		var actions atomic.Int32
		go func() {
			reader := bufio.NewReader(serverIn)
			for {
				data, err := reader.ReadBytes(0)
				if err != nil {
					return
				}
				var request struct {
					ID     int
					Method string
					Params struct{ Expression string }
				}
				if json.Unmarshal(data[:len(data)-1], &request) != nil {
					return
				}
				var result any
				switch request.Method {
				case "Target.getTargets":
					result = map[string]any{"targetInfos": []map[string]string{{"type": "page", "url": "https://music.apple.com/", "targetId": "page"}}}
				case "Target.attachToTarget":
					result = map[string]string{"sessionId": "session"}
				case "Runtime.evaluate":
					var value any = caps
					if !strings.Contains(request.Params.Expression, "tapefox.mediaCapabilities") {
						actions.Add(1)
						value = true
					}
					result = map[string]any{"result": map[string]any{"value": map[string]any{"data": value}}}
				}
				reply, _ := json.Marshal(map[string]any{"id": request.ID, "result": result})
				if _, err := serverOut.Write(append(reply, 0)); err != nil {
					return
				}
			}
		}()
		b := &Browser{pipe: p, background: true, binary: "fixture"}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := b.Do(ctx, Action{Kind: "play", ID: "fixture"})
		cancel()
		p.close()
		serverIn.Close()
		serverOut.Close()
		if caps.Widevine {
			if err != nil || actions.Load() != 1 {
				t.Fatalf("supported runtime did not submit once: %v actions=%d", err, actions.Load())
			}
		} else if !errors.Is(err, ErrPlaybackUnavailable) || actions.Load() != 0 {
			t.Fatalf("unsupported runtime submitted a queue: %v actions=%d", err, actions.Load())
		}
	}
}

func TestBridgeQueueAcknowledgementAwaitsAudioStart(t *testing.T) {
	for _, outcome := range []string{"playing", "start", "unsupported"} {
		runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let plays=0;
const mk={isAuthorized:true,storefrontId:'us',isPlaying:'`+outcome+`'==='playing',autoplayEnabled:false,
  setQueue:async()=>({items:[]}),play:async()=>{plays++;if('`+outcome+`'==='unsupported')throw Object.assign(Error('PRIVATE'),{name:'NotSupportedError'});mk.isPlaying=true}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const result=await bridge({op:'play',id:'123'});
assert.equal(plays,'`+outcome+`'==='playing'?0:1);
if('`+outcome+`'==='unsupported'){assert.equal(result.error,'media_unsupported');assert.equal(mk.autoplayEnabled,false);assert.equal(JSON.stringify(result).includes('PRIVATE'),false);}
else{assert.equal(result.data,true);assert.equal(mk.isPlaying,true);}
`, map[string]string{"op": "snapshot"})
	}
}

func TestBridgeAudioStartCannotAcknowledgeAChangedSession(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const mk={isAuthorized:true,storefrontId:'us',isPlaying:false,autoplayEnabled:false,
  setQueue:async()=>({items:[]}),play:async()=>{mk.storefrontId='jp'}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
assert.equal((await bridge({op:'play',id:'123'})).error,'stale');
assert.equal(mk.autoplayEnabled,false);
`, map[string]string{"op": "snapshot"})
}

func TestBridgeTimedOutQueueNeverIssuesALatePlay(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const realTimer=setTimeout;globalThis.setTimeout=(fn,ms)=>realTimer(fn,Math.min(ms,10));
let finish,plays=0;
const mk={isAuthorized:true,storefrontId:'us',isPlaying:false,autoplayEnabled:false,
  setQueue:async()=>new Promise(resolve=>finish=resolve),play:async()=>{plays++}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
assert.equal((await bridge({op:'play',id:'123'})).error,'timeout');
finish({items:[]});await new Promise(setImmediate);
assert.equal(plays,0);assert.equal(mk.autoplayEnabled,false);
`, map[string]string{"op": "snapshot"})
}

func TestUnsupportedBridgeErrorRemainsTyped(t *testing.T) {
	if !errors.Is(bridgeError("media_unsupported"), ErrPlaybackUnavailable) {
		t.Fatal("unsupported playback cannot be recognized by the UI")
	}
}
