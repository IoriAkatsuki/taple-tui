package applemusic

import (
	"context"
	"strings"
	"testing"
)

func TestCollectionStartValidationPrecedesBrowserIO(t *testing.T) {
	for _, action := range []Action{
		{Kind: "play", ID: "123", StartID: "456"},
		{Kind: "play-collection", ID: "album", ResourceType: "albums", StartID: "bad\x00id"},
		{Kind: "play-collection", ID: "album", ResourceType: "albums", StartID: strings.Repeat("x", 257)},
	} {
		if err := (&Browser{}).Do(context.Background(), action); err == nil {
			t.Fatal("invalid starting track reached browser IO")
		}
	}
}

func TestBridgeCatalogPlaybackReusesResourcesAndEnablesAutoplay(t *testing.T) {
	for _, op := range []string{"play", "enqueue"} {
		t.Run(op, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let reads=0,queued=0;
const song={id:'000123',type:'songs',attributes:{name:'fixture',hasLyrics:false,playParams:{id:'000123',kind:'song'}}};
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:false,api:{music:async()=>{reads++;return {data:{results:{songs:{data:[song]}}}}}},
setQueue:async options=>{assert.deepEqual(options,{items:[song],startPlaying:true});assert.equal(mk.autoplayEnabled,false);queued++},
playLater:async options=>{assert.deepEqual(options,{items:[song]});assert.equal(mk.autoplayEnabled,false);queued++}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
await bridge({op:'catalog',kind:'songs',query:'fixture'});assert.equal(mk.autoplayEnabled,false);
assert.equal((await bridge({op:'`+op+`',resourceType:'songs',id:'000123'})).data,true);
assert.equal(reads,1);assert.equal(queued,1);assert.equal(mk.autoplayEnabled,'`+op+`'==='play');`, map[string]string{"op": "snapshot"})
		})
	}
}

func TestBridgeCollectionStartsAtSelectedTrackWithoutPlayingTheFirst(t *testing.T) {
	for _, resource := range []string{"albums", "playlists"} {
		t.Run(resource, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let queued=0,selected=-1,played=-1;
const queue={items:[{id:'first'},{id:'000123'}]};
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:false,setQueue:async options=>{
assert.equal(options.`+map[string]string{"albums": "album", "playlists": "playlist"}[resource]+`,'collection');
assert.equal(options.startPlaying,false);
assert.equal(mk.autoplayEnabled,false);queued++;mk.queue=queue;return queue;
},changeToMediaAtIndex:async index=>{selected=index;played=index},play:async()=>{throw Error('already playing')}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
assert.equal((await bridge({op:'play-collection',resourceType:'`+resource+`',id:'collection',startID:'000123'})).data,true);
assert.equal(queued,1);assert.equal(played,1);assert.equal(mk.autoplayEnabled,true);`, map[string]string{"op": "snapshot"})
		})
	}
}

func TestBridgeLibraryCollectionRetainsWholeQueueAndValidatesStart(t *testing.T) {
	for _, start := range []string{"i.second", "i.missing"} {
		t.Run(start, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let reads=0,queued=0,selected=-1,played=-1;
const song=id=>({id,type:'library-songs',attributes:{hasLyrics:false,playParams:{id,kind:'song',isLibrary:true}}});
const first=song('i.first'),second=song('i.second');
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:false,api:{music:async path=>{
reads++;return {data:{data:[reads===1?first:second],next:reads===1?'/v1/me/library/playlists/p.collection/tracks?offset=1':undefined}};
}},setQueue:async options=>{assert.equal(reads,2);assert.deepEqual(options.items,[first,second]);assert.equal(options.startPlaying,false);queued++;mk.queue={items:options.items};return mk.queue},
changeToMediaAtIndex:async index=>{selected=index;played=index},play:async()=>{throw Error('already playing')}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const result=await bridge({op:'play-collection',id:'p.collection',resourceType:'library-playlists',startID:'`+start+`'});
if('`+start+`'==='i.missing'){assert.equal(result.error,'stale');assert.equal(queued,0);}
else{assert.equal(result.data,true);assert.equal(queued,1);assert.equal(played,1);assert.equal(mk.autoplayEnabled,true);}`, map[string]string{"op": "snapshot"})
		})
	}
}

func TestBridgeAutoplayReflectsActualSDKCapability(t *testing.T) {
	for _, supported := range []bool{false, true} {
		property := ""
		if supported {
			property = "autoplayEnabled:false,"
		}
		output := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
const mk={isAuthorized:true,`+property+`setQueue:async()=>{}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
await bridge({op:'play',id:'123'});`, map[string]string{"op": "snapshot"})
		want := `"autoplayAvailable":false,"autoplay":false`
		if supported {
			want = `"autoplayAvailable":true,"autoplay":true`
		}
		if !strings.Contains(string(output), want) {
			t.Fatalf("autoplay capability missing: %s", output)
		}
	}
}

func TestBridgeAutoplayNeverGatesExplicitPlaybackOnRecommendations(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let played=0;
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:true,setQueue:async()=>{
if(mk.autoplayEnabled) throw Error('SDK replaceQueue awaited a failed recommendation request');
played++;
}};globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
for(const id of ['123','456']){assert.equal((await bridge({op:'play',id})).data,true);assert.equal(mk.autoplayEnabled,true);}
assert.equal(played,2);`, map[string]string{"op": "snapshot"})
}

func TestBridgeLatePlaybackCannotEnableAutoplayForANewerRequest(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let finish,enabled=false;const writes=[];
const pending=new Promise(resolve=>finish=resolve);
const mk={isAuthorized:true,storefrontId:'us',get autoplayEnabled(){return enabled},set autoplayEnabled(v){writes.push(v);enabled=v},
setQueue:async options=>{if(options.song==='123') await pending}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const first=bridge({op:'play',id:'123'});await new Promise(setImmediate);
await bridge({op:'play',id:'456'});finish();await first;
assert.deepEqual(writes,[false,false,true]);`, map[string]string{"op": "snapshot"})
}

func TestBridgeTimedOutPlaybackDoesNotScheduleLateAutoplay(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const realTimer=setTimeout;globalThis.setTimeout=(fn,ms)=>realTimer(fn,Math.min(ms,10));
let finish;const pending=new Promise(resolve=>finish=resolve);
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:false,setQueue:async()=>pending};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
assert.equal((await bridge({op:'play',id:'123'})).error,'timeout');finish();await new Promise(setImmediate);
assert.equal(mk.autoplayEnabled,false);`, map[string]string{"op": "snapshot"})
}

func TestBridgeCollectionNeverPlaysAnUnavailableSelectedTrack(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};let played=0;
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:false,setQueue:async options=>{
assert.equal(options.startPlaying,false);mk.queue={items:[{id:'111'},{id:'222'}]};return mk.queue;
},changeToMediaAtIndex:async()=>played++,play:async()=>played++};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const result=await bridge({op:'play-collection',resourceType:'albums',id:'album',startID:'333'});
assert.equal(result.error,'stale');assert.equal(played,0);assert.equal(mk.autoplayEnabled,false);`, map[string]string{"op": "snapshot"})
}

func TestBridgeLibraryStartUsesFilteredSDKQueueAndCatalogMapping(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};let played=0;
const resource=id=>({id,type:'library-songs',attributes:{hasLyrics:false,playParams:{id,kind:'song',isLibrary:true,catalogId:id==='i.selected'?'002':'001'}}});
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:false,api:{music:async()=>({data:{data:[resource('i.first'),resource('i.selected')]}})},
setQueue:async options=>{assert.equal(options.startPlaying,false);mk.queue={items:[{id:'002'}]};return mk.queue},
changeToMediaAtIndex:async index=>{assert.equal(index,0);played++},play:async()=>{throw Error('already playing')}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
assert.equal((await bridge({op:'play-collection',resourceType:'library-albums',id:'album',startID:'i.selected'})).data,true);
assert.equal(played,1);`, map[string]string{"op": "snapshot"})
}
