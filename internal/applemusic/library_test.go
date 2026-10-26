package applemusic

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestBridgeLibraryAndRecommendationsRoutes(t *testing.T) {
	for _, tc := range []struct{ kind, path, resource string }{
		{"library-playlists", "/v1/me/library/playlists", "library-playlists"},
		{"library-albums", "/v1/me/library/albums", "library-albums"},
		{"library-songs", "/v1/me/library/songs", "library-songs"},
		{"library-playlist-tracks", "/v1/me/library/playlists/opaque.001/tracks", "library-songs"},
		{"library-album-tracks", "/v1/me/library/albums/opaque.001/tracks", "library-songs"},
		{"playlist-tracks", "/v1/catalog/us/playlists/opaque.001/tracks", "songs"},
		{"recommendations", "/v1/me/recommendations", "personal-recommendation"},
		{"recommendation-contents", "/v1/me/recommendations/opaque.001/contents", "stations"},
		{"personal-station", "/v1/catalog/us/stations", "stations"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			out := runBridge(t, `import assert from 'node:assert/strict';globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({isAuthorized:true,storefrontId:'us',
get musicUserToken(){throw Error('must not read credentials')},api:{music:async(path,args)=>{
assert.equal(path,'`+tc.path+`');
if('`+tc.kind+`'==='personal-station') assert.equal(args['filter[identity]'],'personal');
return {data:{data:[{id:'opaque.001',type:'`+tc.resource+`',attributes:{name:'Resource',playParams:{id:'opaque.001',kind:'song',isLibrary:true}}}]}};
}}})}`, map[string]string{"op": "catalog", "kind": tc.kind, "id": "opaque.001"})
			var got struct {
				Data  CatalogPage
				Error string
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			if got.Error != "" || len(got.Data.Items) != 1 || got.Data.Items[0].Type != tc.resource || got.Data.Items[0].ID != "opaque.001" {
				t.Fatalf("library request: %s", out)
			}
		})
	}
}

func TestLibraryPaginationValidationBeforeIO(t *testing.T) {
	for _, kind := range []string{LibraryPlaylists, LibraryAlbums, LibrarySongs, Recommendations, PersonalStation} {
		if _, err := (&Browser{stopped: true}).Catalog(context.Background(), CatalogRequest{Kind: kind}); err == nil || err.Error() != "播放器已关闭" {
			t.Fatalf("valid library kind rejected: %s %v", kind, err)
		}
	}
	for _, next := range []string{"/v1/me/library/songs?offset=25", "/v1/catalog/us/playlists?offset=25", "https://api.music.apple.com/v1/me/library/playlists?offset=25", "/v1/me/library/playlists/../songs?offset=25"} {
		if _, err := (&Browser{}).Catalog(context.Background(), CatalogRequest{Kind: LibraryPlaylists, Next: next}); err == nil {
			t.Fatalf("unsafe library cursor accepted: %s", next)
		}
	}
	out := runBridge(t, `import assert from 'node:assert/strict';globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({isAuthorized:true,storefrontId:'us',api:{music:async path=>{assert.equal(path,'/v1/me/library/playlists?offset=25');return {data:{data:[]}}}}})}`,
		map[string]string{"op": "catalog", "kind": LibraryPlaylists, "next": "/v1/me/library/playlists?offset=25"})
	var got struct {
		Data  CatalogPage
		Error string
	}
	json.Unmarshal(out, &got)
	if got.Error != "" || len(got.Data.Items) != 0 {
		t.Fatalf("valid library next failed: %s", out)
	}
}

func TestLibraryPlaylistFailureNeverQueuesPartialResults(t *testing.T) {
	for _, body := range []string{
		`return {data:{data:[song],next:'/v1/me/library/songs?offset=2'}}`,
		`return {data:{data:[song],next:0}}`,
		`return {data:{data:[song],next:'/v1/me/library/playlists/p.opaque/tracks'}}`,
		`await new Promise(r=>originalTimer(r,25));return {data:{data:[song]}}`,
	} {
		setup := `const originalTimer=setTimeout;globalThis.setTimeout=(fn,ms)=>originalTimer(fn,Math.min(ms,5));
globalThis.location={origin:'https://music.apple.com'};let played=0;
const song={id:'i.one',type:'library-songs',attributes:{playParams:{id:'i.one',kind:'song',isLibrary:true}}};
globalThis.MusicKit={getInstance:()=>({isAuthorized:true,storefrontId:'us',api:{music:async()=>{` + body + `}},setQueue:async()=>{played++}})};
const bridge=` + bridgeJS + `;
const result=await bridge({op:'play-collection',id:'p.opaque',resourceType:'library-playlists'});
await new Promise(r=>originalTimer(r,35));
if(!result.error || played!==0) throw Error('partial or late playlist was queued');
globalThis.MusicKit={getInstance:()=>undefined};`
		runBridge(t, setup, map[string]string{"op": "snapshot"})
	}
}

func TestTypedPlaybackArgumentsStopBeforeIO(t *testing.T) {
	for _, a := range []Action{{Kind: "play", ID: "i.x", ResourceType: "library-playlists"}, {Kind: "play-station", ID: "x", ResourceType: "songs"}, {Kind: "play-collection", ID: "x", ResourceType: "personal-recommendation"}} {
		if err := (&Browser{}).Do(context.Background(), a); err == nil {
			t.Fatalf("invalid typed action accepted: %s", fmt.Sprint(a))
		}
	}
}

func TestBridgeRecommendationTitleAndLibraryGuards(t *testing.T) {
	for _, tc := range []struct{ setup, want string }{
		{`mk.isAuthorized=false`, "login"},
		{`mk.api.music=async()=>({data:{data:[],next:'/v1/me/library/songs?offset=25'}})`, "request"},
		{`mk.api.music=async()=>{mk.isAuthorized=false;return {data:{data:[]}}}`, "login"},
	} {
		out := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
const mk={isAuthorized:true,storefrontId:'us',api:{music:async()=>{throw Error('must not be called')}}};
globalThis.MusicKit={getInstance:()=>mk};`+tc.setup, map[string]string{"op": "catalog", "kind": "library-playlists"})
		var got struct{ Error string }
		json.Unmarshal(out, &got)
		if got.Error != tc.want {
			t.Fatalf("guard: %s", out)
		}
	}
	out := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({isAuthorized:true,storefrontId:'us',api:{music:async()=>({data:{data:[{id:'rec',type:'personal-recommendation',attributes:{title:{stringForDisplay:'Made for You'}}}]}})}})}`,
		map[string]string{"op": "catalog", "kind": "recommendations"})
	var got struct{ Data CatalogPage }
	json.Unmarshal(out, &got)
	if len(got.Data.Items) != 1 || got.Data.Items[0].Name != "Made for You" {
		t.Fatalf("recommendation title: %s", out)
	}
}

func TestBridgePlaysLibraryResourcesWithoutTreatingIDsAsCatalogSongs(t *testing.T) {
	for _, op := range []string{"play", "enqueue"} {
		out := runBridge(t, `import assert from 'node:assert/strict';globalThis.location={origin:'https://music.apple.com'};
const resource={id:'i.opaque',type:'library-songs',attributes:{playParams:{id:'i.opaque',kind:'song',isLibrary:true}}};
const check=async options=>{assert.deepEqual(options.items,[resource]);assert.equal(options.song,undefined);};
globalThis.MusicKit={getInstance:()=>({isAuthorized:true,storefrontId:'us',api:{music:async(path)=>{assert.equal(path,'/v1/me/library/songs/i.opaque');return {data:{data:[resource]}}}},setQueue:check,playLater:check})}`,
			Action{Kind: op, ID: "i.opaque", ResourceType: "library-songs"})
		var got struct{ Data bool }
		if json.Unmarshal(out, &got) != nil || !got.Data {
			t.Fatalf("library playback: %s", out)
		}
	}
}

func TestBridgeLibraryPlaylistLoadsPagesBeforeChangingQueue(t *testing.T) {
	out := runBridge(t, `import assert from 'node:assert/strict';globalThis.location={origin:'https://music.apple.com'};
let calls=0;const song=id=>({id,type:'library-songs',attributes:{playParams:{id,kind:'song',isLibrary:true}}});
globalThis.MusicKit={getInstance:()=>({isAuthorized:true,storefrontId:'us',api:{music:async(path)=>{
calls++;assert.equal(path,calls===1?'/v1/me/library/playlists/p.opaque/tracks':'/v1/me/library/playlists/p.opaque/tracks?offset=1');
return {data:{data:[song('i.'+calls)],next:calls===1?'/v1/me/library/playlists/p.opaque/tracks?offset=1':undefined}};
}},setQueue:async options=>{assert.equal(calls,2);assert.deepEqual(options.items.map(i=>i.id),['i.1','i.2']);assert.equal(options.startPlaying,true)}})}`,
		Action{Kind: "play-collection", ID: "p.opaque", ResourceType: "library-playlists"})
	var got struct{ Data bool }
	if json.Unmarshal(out, &got) != nil || !got.Data {
		t.Fatalf("playlist playback: %s", out)
	}
	for _, tc := range []struct{ kind, resource, key string }{{"play-station", "stations", "station"}, {"play-collection", "playlists", "playlist"}, {"play-collection", "albums", "album"}} {
		out := runBridge(t, `import assert from 'node:assert/strict';globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({isAuthorized:true,setQueue:async options=>assert.deepEqual(options,{`+tc.key+`:'opaque',startPlaying:true})})}`,
			Action{Kind: tc.kind, ID: "opaque", ResourceType: tc.resource})
		var got struct{ Data bool }
		if json.Unmarshal(out, &got) != nil || !got.Data {
			t.Fatalf("typed playback: %s", out)
		}
	}
}
