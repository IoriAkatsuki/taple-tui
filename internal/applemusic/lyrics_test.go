package applemusic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestBridgeLyricsUsesAuthorizedSDKAndKeepsIdentity(t *testing.T) {
	out := runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const mk={isAuthorized:true,storefrontId:'us',nowPlayingItem:{id:'i.local',type:'library-songs',attributes:{}},
get musicUserToken(){throw Error('PRIVATE')},get developerToken(){throw Error('PRIVATE')},
api:{v3:{music:async(path,args)=>{
 if(path==='/v1/me/library/songs/i.local') {assert.equal(args.include,'catalog');return {data:{data:[{relationships:{catalog:{data:[{id:'000123'}]}}}]}};}
 assert.equal(path,'/v1/catalog/us/songs/000123/syllable-lyrics');
 return {data:{data:[{attributes:{ttml:'<tt><body><p begin="1s" end="2s">fixture</p></body></tt>'}}]}};
}}}};globalThis.MusicKit={getInstance:()=>mk};`, map[string]string{"op": "lyrics", "id": "i.local", "storefront": "us"})
	var got struct {
		Data  struct{ Status, TrackID, CatalogID, Storefront, TTML string }
		Error string
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if got.Error != "" || got.Data.Status != "available" || got.Data.TrackID != "i.local" || got.Data.CatalogID != "000123" || got.Data.Storefront != "us" || !strings.Contains(got.Data.TTML, "fixture") || strings.Contains(string(out), "PRIVATE") {
		t.Fatalf("lyrics request failed or lost identity: %s", out)
	}
}

func TestLyricsRequestValidationPrecedesBrowserAccess(t *testing.T) {
	for _, q := range []LyricsRequest{{}, {ID: "123"}, {ID: "../", Storefront: "\n"}, {ID: "bad\x00id", Storefront: "us"}, {ID: "..", Storefront: "us"}, {ID: strings.Repeat("a", 257), Storefront: "us"}} {
		if _, err := (&Browser{}).Lyrics(context.Background(), q); err == nil {
			t.Fatalf("accepted invalid request %+v", q)
		}
	}
}

func TestLyricsTransportParsesDataWithoutUserGesture(t *testing.T) {
	for _, tc := range []struct{ name, track, xml, status string }{
		{"timed", "i.local", `<tt xml:lang="en"><body><p begin="1s" end="2s">fixture</p></body></tt>`, "available"},
		{"bare seconds", "i.local", `<tt><body><p begin="1.000" end="2.000">fixture</p></body></tt>`, "available"},
		{"plain", "i.local", `<tt><body><p>fixture</p></body></tt>`, "unsynced"},
		{"invalid", "i.local", `<!DOCTYPE tt><tt><body/></tt>`, "parse_error"},
		{"stale", "other", `<tt><body><p>fixture</p></body></tt>`, "stale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, serverOut := io.Pipe()
			serverIn, output := io.Pipe()
			b := &Browser{pipe: newPipe(input, output), background: true, target: "page", session: "session"}
			defer b.pipe.close()
			defer serverIn.Close()
			defer serverOut.Close()
			q := LyricsRequest{ID: "i.local", Storefront: "us"}
			checked := make(chan error, 1)
			go func() {
				reader := bufio.NewReader(serverIn)
				for range 2 {
					data, err := reader.ReadBytes(0)
					if err != nil {
						checked <- err
						return
					}
					var request struct {
						ID     int
						Method string
						Params struct {
							Expression  string
							UserGesture bool
						}
					}
					if err := json.Unmarshal(data[:len(data)-1], &request); err != nil {
						checked <- err
						return
					}
					var result any
					if request.Method == "Target.getTargets" {
						result = map[string]any{"targetInfos": []any{map[string]string{"targetId": "page", "type": "page", "url": "https://music.apple.com/us/browse"}}}
					} else if request.Method == "Runtime.evaluate" {
						want, _ := scriptFor(struct {
							Op string `json:"op"`
							LyricsRequest
						}{"lyrics", q})
						if request.Params.UserGesture || request.Params.Expression != want {
							checked <- fmt.Errorf("incorrect read-only lyrics request")
							return
						}
						result = map[string]any{"result": map[string]any{"value": map[string]any{"data": map[string]any{"status": "available", "trackID": tc.track, "catalogID": "000123", "storefront": "us", "ttml": tc.xml}}}}
					} else {
						checked <- fmt.Errorf("unexpected method %s", request.Method)
						return
					}
					response, _ := json.Marshal(map[string]any{"id": request.ID, "result": result})
					if _, err := serverOut.Write(append(response, 0)); err != nil {
						checked <- err
						return
					}
				}
				checked <- nil
			}()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, err := b.Lyrics(ctx, q)
			if err != nil || got.Status != tc.status || got.Source != "apple-web-undocumented" || got.TrackID != "i.local" || got.CatalogID != "000123" {
				t.Fatalf("lyrics result: %+v %v", got, err)
			}
			if tc.status == "available" && (len(got.Lines) != 1 || got.Lines[0].Text != "fixture" || got.Lines[0].Start != time.Second) {
				t.Fatal("transport did not parse real TTML into timed lines")
			}
			if tc.status == "parse_error" {
				report, _ := json.Marshal(got)
				if !strings.Contains(string(report), "DTD") || strings.Contains(string(report), "<!DOCTYPE") {
					t.Fatal("safe parse reason did not reach the lyric result")
				}
			}
			if err := <-checked; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBridgeLyricsFailureStatesAndStaleResponses(t *testing.T) {
	for _, tc := range []struct{ name, setup, want string }{
		{"login", `mk.isAuthorized=false`, "login_required"},
		{"mapping", `mk.nowPlayingItem.id='opaque';request.id='opaque'`, "no_catalog_mapping"},
		{"no lyrics", `mk.nowPlayingItem.attributes.hasLyrics=false`, "no_lyrics"},
		{"video", `mk.nowPlayingItem.type='music-videos'`, "unsupported_media"},
		{"wrong track", `request.id='456'`, "stale"},
		{"wrong region", `request.storefront='jp'`, "stale"},
		{"late track", `mk.api.music=async()=>{mk.nowPlayingItem.id='456';return reply}`, "stale"},
		{"late login", `mk.api.music=async()=>{mk.isAuthorized=false;return reply}`, "login_required"},
		{"late region", `mk.api.music=async()=>{mk.storefrontId='jp';return reply}`, "stale"},
		{"401", `mk.api.music=async()=>{throw {status:401,message:'PRIVATE'}}`, "unauthorized"},
		{"403", `mk.api.music=async()=>({data:{errors:[{status:'403',detail:'PRIVATE'}]}})`, "access_denied"},
		{"404", `mk.api.music=async()=>({data:{errors:[{status:'404'}]}})`, "unavailable"},
		{"429", `mk.api.music=async()=>({errors:[{status:429}]})`, "rate_limited"},
		{"empty", `mk.api.music=async()=>({data:{data:[]}})`, "unavailable"},
		{"timeout", `const original=setTimeout;globalThis.setTimeout=(fn,ms)=>original(fn,Math.min(ms,5));mk.api.music=async()=>new Promise(()=>{})`, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setup := `globalThis.location={origin:'https://music.apple.com'};
const reply={data:{data:[{attributes:{ttml:'SYNTHETIC'}}]}};
const request={id:'123',storefront:'us'};
const mk={isAuthorized:true,storefrontId:'us',nowPlayingItem:{id:'123',type:'songs',attributes:{}},api:{music:async()=>reply}};
globalThis.MusicKit={getInstance:()=>mk};` + tc.setup + `;
// Request mutation is reflected by the fixture's argument below.
globalThis.fixtureRequest=request;`
			// Embed the same production bridge while allowing per-case requested identity.
			setup += fmt.Sprintf(`;const bridge=%s;const result=await bridge({op:'lyrics',...request});globalThis.fixtureResult=result;`, bridgeJS)
			// A throwing assertion makes runBridge fail without printing exception details from the page.
			runBridge(t, setup+`;if(fixtureResult.data?.status!==`+fmt.Sprintf("%q", tc.want)+`||JSON.stringify(fixtureResult).includes('PRIVATE')) throw Error(JSON.stringify(fixtureResult));globalThis.MusicKit={getInstance:()=>undefined}`, map[string]string{"op": "snapshot"})
		})
	}
}
