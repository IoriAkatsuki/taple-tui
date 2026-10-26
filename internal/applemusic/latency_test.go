package applemusic

import (
	"fmt"
	"testing"
)

func TestBridgeLyricsOverlapPlaybackAndShareTheRequest(t *testing.T) {
	for _, op := range []string{"play", "jump"} {
		t.Run(op, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let finishAudio, finishLyrics, calls=0;
const audio=new Promise(resolve=>finishAudio=resolve);
const lyric=new Promise(resolve=>finishLyrics=resolve);
const target={id:'123',type:'songs',attributes:{}};
const mk={isAuthorized:true,storefrontId:'us',nowPlayingItem:{id:'old'},queue:{items:[target]},
setQueue:async()=>{await audio;mk.nowPlayingItem=target},changeToMediaAtIndex:async()=>{await audio;mk.nowPlayingItem=target},play:async()=>{},
api:{music:async path=>{calls++;assert.equal(path,'/v1/catalog/us/songs/123/syllable-lyrics');return lyric}}};
globalThis.MusicKit={getInstance:()=>mk};
const bridge=`+bridgeJS+`;
const playing=bridge({op:'`+op+`',id:'123',value:0});
await new Promise(setImmediate);
assert.equal(calls,1,'lyrics must start before the audio load completes');
assert.equal(mk.nowPlayingItem.id,'old','prefetch must not change playback');
mk.nowPlayingItem=target;
const foreground=bridge({op:'lyrics',id:'123',storefront:'us'});
await new Promise(setImmediate);
assert.equal(calls,1,'foreground must join the in-flight request');
finishLyrics({data:{data:[{attributes:{ttml:'<tt><body><p>fixture</p></body></tt>'}}]}});
assert.equal((await foreground).data.status,'available');
finishAudio();assert.equal((await playing).data,true);
assert.equal((await bridge({op:'lyrics',id:'123',storefront:'us'})).data.status,'available');
assert.equal(calls,1,'repeat playback must reuse successful lyrics');`, map[string]string{"op": "snapshot"})
		})
	}
}

func TestBridgeLyricsCacheIsBoundedRefreshableAndSessionScoped(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let calls=0;
const mk={isAuthorized:true,storefrontId:'us',nowPlayingItem:{id:'100',type:'songs'},
api:{music:async()=>{calls++;return {data:{data:[{attributes:{ttml:'<tt><body><p>fixture</p></body></tt>'}}]}}}}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const read=async(id='100',refresh=false)=>{mk.nowPlayingItem={id,type:'songs'};return bridge({op:'lyrics',id,storefront:mk.storefrontId,refresh})};
assert.equal((await read()).data.status,'available');await read();assert.equal(calls,1);
await read('100',true);assert.equal(calls,2,'explicit retry must bypass a completed cache entry');
for(let id=101;id<=108;id++) await read(String(id));
await read();assert.equal(calls,11,'cache must evict old entries instead of growing indefinitely');
mk.isAuthorized=false;assert.equal((await read()).data.status,'login_required');
mk.isAuthorized=true;await read();assert.equal(calls,12,'logout must clear lyrics');
mk.storefrontId='jp';await read();assert.equal(calls,13,'region changes must clear lyrics');
globalThis.MusicKit={getInstance:()=>({...mk})};await read();assert.equal(calls,14,'new SDK instances must not reuse account data');`, map[string]string{"op": "snapshot"})
}

func TestBridgeLyricsFailuresAndTimeoutsDoNotPoisonCache(t *testing.T) {
	for _, failure := range []string{"reject", "timeout"} {
		t.Run(failure, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const realTimeout=setTimeout;globalThis.setTimeout=(fn,ms)=>realTimeout(fn,Math.min(ms,20));
let calls=0,aborted=false;
const mk={isAuthorized:true,storefrontId:'us',nowPlayingItem:{id:'123',type:'songs'},
api:{music:async(path,args,options)=>{calls++;if(calls===1){
if('`+failure+`'==='reject') throw {status:429,message:'PRIVATE'};
options.fetchOptions.signal.addEventListener('abort',()=>aborted=true);return new Promise(()=>{});
}return {data:{data:[{attributes:{ttml:'<tt><body><p>fixture</p></body></tt>'}}]}}}}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const q={op:'lyrics',id:'123',storefront:'us'};
const first=await bridge(q);assert.equal(first.data.status,'`+map[string]string{"reject": "rate_limited", "timeout": "timeout"}[failure]+`');
assert.ok(!JSON.stringify(first).includes('PRIVATE'));
assert.equal((await bridge(q)).data.status,'available');assert.equal(calls,2);
if('`+failure+`'==='timeout') assert.ok(aborted);`, map[string]string{"op": "snapshot"})
		})
	}
}

func TestBridgeLibraryMappingSharesInflightCatalogLyrics(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let finish,calls=0;
const lyrics=new Promise(resolve=>finish=resolve);
const resource={id:'i.local',type:'library-songs',attributes:{playParams:{id:'i.local',kind:'song',isLibrary:true}},relationships:{catalog:{data:[{id:'000123'}]}}};
const mk={isAuthorized:true,storefrontId:'us',nowPlayingItem:{id:'old'},setQueue:async()=>{},api:{music:async(path,args)=>{
if(path==='/v1/me/library/songs/i.local') return {data:{data:[resource]}};
assert.equal(path,'/v1/catalog/us/songs/000123/syllable-lyrics');calls++;return lyrics;
}}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
await bridge({op:'play',id:'i.local',resourceType:'library-songs'});await new Promise(setImmediate);
assert.equal(calls,1);
mk.nowPlayingItem={id:'000123',type:'songs'};
const foreground=bridge({op:'lyrics',id:'000123',storefront:'us'});await new Promise(setImmediate);
assert.equal(calls,1,'resolved library mapping must share the still pending catalog request');
finish({data:{data:[{attributes:{ttml:'<tt><body><p>fixture</p></body></tt>'}}]}});
assert.equal((await foreground).data.status,'available');`, map[string]string{"op": "snapshot"})
}

func TestBridgeLibraryMappingRefreshesCompletedCatalogCache(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let calls=0;
const mk={isAuthorized:true,storefrontId:'us',nowPlayingItem:{id:'123',type:'songs'},api:{music:async path=>{
if(path==='/v1/me/library/songs/i.local') return {data:{data:[{relationships:{catalog:{data:[{id:'123'}]}}}]}};
calls++;return {data:{data:[{attributes:{ttml:'version'+calls}}]}};
}}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
assert.equal((await bridge({op:'lyrics',id:'123',storefront:'us'})).data.ttml,'version1');
mk.nowPlayingItem={id:'i.local',type:'library-songs',attributes:{}};
assert.equal((await bridge({op:'lyrics',id:'i.local',storefront:'us',refresh:true})).data.ttml,'version2','retry after resolving a library ID must bypass completed catalog data');
assert.equal(calls,2);`, map[string]string{"op": "snapshot"})
}

func TestBridgeLyricEvictionAbortsPendingReads(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let calls=0,aborted=0;
const mk={isAuthorized:true,storefrontId:'us',setQueue:async()=>{},api:{music:async(path,args,options)=>{
calls++;options.fetchOptions.signal.addEventListener('abort',()=>aborted++);return new Promise(()=>{});
}}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
for(let id=100;id<109;id++) await bridge({op:'play',id:String(id)});
assert.equal(calls,9);assert.equal(aborted,1,'eviction must abort a superseded read');
mk.isAuthorized=false;await bridge({op:'snapshot'});await new Promise(setImmediate);
assert.equal(aborted,9,'logout must abort the remaining read-only requests');`, map[string]string{"op": "snapshot"})
}

func TestBridgeLyricAliasEvictionKeepsReferencedRequest(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let finish,aborted=false;
const pending=new Promise(resolve=>finish=resolve);
const resource={id:'i.local',type:'library-songs',attributes:{playParams:{id:'i.local',isLibrary:true,catalogId:'123'}}};
const mk={isAuthorized:true,storefrontId:'us',setQueue:async()=>{},api:{music:async(path,args,options)=>{
if(path==='/v1/me/library/songs/i.local') return {data:{data:[resource]}};
if(path==='/v1/catalog/us/songs/123/syllable-lyrics'){
options.fetchOptions.signal.addEventListener('abort',()=>aborted=true);return pending;
}return new Promise(()=>{});
}}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
await bridge({op:'play',id:'i.local',resourceType:'library-songs'});
for(let id=201;id<=206;id++) await bridge({op:'play',id:String(id)});
mk.nowPlayingItem={id:'123',type:'songs'};
const foreground=bridge({op:'lyrics',id:'123',storefront:'us'});await new Promise(setImmediate);
await bridge({op:'play',id:'207'});
assert.equal(aborted,false,'dropping an old alias must not abort a request still referenced by a recent catalog key');
finish({data:{data:[{attributes:{ttml:'<tt><body><p>fixture</p></body></tt>'}}]}});
assert.equal((await foreground).data.status,'available');
mk.isAuthorized=false;await bridge({op:'snapshot'});`, map[string]string{"op": "snapshot"})
}

func TestBridgeLibraryPlaybackReusesListedResource(t *testing.T) {
	for _, op := range []string{"play", "enqueue"} {
		t.Run(op, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let singleReads=0,lyricReads=0,queued=0,now=0;
Date.now=()=>now;
const resource={id:'i.opaque',type:'library-songs',attributes:{playParams:{id:'i.opaque',kind:'song',isLibrary:true,catalogId:'000123'}}};
const mk={isAuthorized:true,storefrontId:'us',nowPlayingItem:{id:'old'},api:{music:async path=>{
if(path.includes('syllable-lyrics')){lyricReads++;return {data:{data:[{attributes:{ttml:'<tt><body><p>fixture</p></body></tt>'}}]}};}
if(path==='/v1/me/library/songs/i.opaque') singleReads++;
else assert.equal(path,'/v1/me/library/songs');
return {data:{data:[resource]}};
}},setQueue:async options=>{assert.equal(options.items[0],resource);queued++},playLater:async options=>{assert.equal(options.items[0],resource);queued++}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
await bridge({op:'catalog',kind:'library-songs'});
const action={op:'`+op+`',id:'i.opaque',resourceType:'library-songs'};
assert.equal((await bridge(action)).data,true);assert.equal(singleReads,0,'play must not re-fetch an already listed resource');
if(action.op==='play'){
mk.nowPlayingItem={id:'000123',type:'songs'};
assert.equal((await bridge({op:'lyrics',id:'000123',storefront:'us'})).data.status,'available');
assert.equal(lyricReads,1,'library and catalog IDs must share the lyric request');
}else assert.equal(lyricReads,0,'enqueue must not prefetch every appended song');
now=300001;assert.equal((await bridge(action)).data,true);assert.equal(singleReads,1,'expired resources must be revalidated');
mk.isAuthorized=false;await bridge({op:'snapshot'});mk.isAuthorized=true;
assert.equal((await bridge(action)).data,true);assert.equal(singleReads,2,'logout must clear resources');
assert.equal(queued,3);`, map[string]string{"op": "snapshot"})
		})
	}
}

func TestBridgeLatePrefetchCannotPopulateAnotherSession(t *testing.T) {
	for _, change := range []string{"mk.storefrontId='jp'", "mk.isAuthorized=false"} {
		t.Run(change, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let finish,calls=0;
const first=new Promise(resolve=>finish=resolve);
const response={data:{data:[{attributes:{ttml:'<tt><body><p>fixture</p></body></tt>'}}]}};
const mk={isAuthorized:true,storefrontId:'us',nowPlayingItem:{id:'123',type:'songs'},setQueue:async()=>{},
api:{music:async()=>{calls++;return calls===1?first:response}}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
await bridge({op:'play',id:'123'});await new Promise(setImmediate);assert.equal(calls,1);
`+change+`;await bridge({op:'snapshot'});finish(response);await new Promise(setImmediate);
mk.isAuthorized=true;mk.storefrontId='us';
assert.equal((await bridge({op:'lyrics',id:'123',storefront:'us'})).data.status,'available');
assert.equal(calls,2,'late prefetch must not enter a replacement session cache');`, map[string]string{"op": "snapshot"})
		})
	}
}

func TestBridgePrefetchFailureNeverPreventsPlayback(t *testing.T) {
	runBridge(t, fmt.Sprintf(`import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let played=0;
const mk={isAuthorized:true,storefrontId:'us',api:{music:async()=>{throw {status:403}}},setQueue:async()=>played++};
globalThis.MusicKit={getInstance:()=>mk};const bridge=%s;
assert.equal((await bridge({op:'play',id:'123'})).data,true);assert.equal(played,1);`, bridgeJS), map[string]string{"op": "snapshot"})
}
