package applemusic

import "testing"

func TestBridgeCollectionPlaybackReusesBrowsedResourcePages(t *testing.T) {
	for kind, resource := range map[string]string{
		"album-tracks": "albums", "playlist-tracks": "playlists",
		"library-album-tracks": "library-albums", "library-playlist-tracks": "library-playlists",
	} {
		t.Run(kind, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let reads=0,queued=0;
const songs=['101','102'].map(id=>({id,type:'songs',attributes:{hasLyrics:false,playParams:{id,kind:'song'}}}));
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:false,api:{music:async()=>{reads++;return {data:{data:songs}}}},
setQueue:async options=>{assert.deepEqual(options.items,songs,'reuse complete browsed resources, not collection IDs');assert.equal(options.startPlaying,false);queued++;return mk.queue={items:options.items}},
changeToMediaAtIndex:async index=>{mk.nowPlayingItem=mk.queue.items[index]},
play:async()=>{throw Error('changeToMediaAtIndex already starts playback')}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const page=await bridge({op:'catalog',kind:'`+kind+`',id:'collection'});
assert.equal(page.data.items[0].attributes,undefined,'raw resources must stay in the browser');
const result=await bridge({op:'play-collection',resourceType:'`+resource+`',id:'collection',startID:'102'});
assert.equal(result.data,true);assert.equal(reads,1);assert.equal(queued,1);assert.equal(mk.nowPlayingItem.id,'102');`, map[string]string{"op": "snapshot"})
		})
	}
}

func TestBridgeLibraryPlaybackSharesInflightPagesAndStartsLyricsEarly(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let reads=0,lyrics=0,queued=0,finish;
const song=id=>({id,type:'library-songs',attributes:{playParams:{id,kind:'song',isLibrary:true,catalogId:id==='i.1'?'101':'102'}}});
const first=song('i.1'),second=song('i.2'),path='/v1/me/library/playlists/p.list/tracks';
const mk={isAuthorized:true,storefrontId:'us',api:{music:async url=>{
if(url.endsWith('/syllable-lyrics')){lyrics++;return {data:{data:[{attributes:{ttml:'<tt>fixture</tt>'}}]}}}
reads++;if(reads===1)return {data:{data:[first],next:path+'?offset=1'}};
await new Promise(resolve=>finish=resolve);return {data:{data:[second]}};
}},setQueue:async options=>{queued++;assert.deepEqual(options.items,[first,second]);return mk.queue={items:options.items}},
changeToMediaAtIndex:async()=>{},play:async()=>{}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
await bridge({op:'catalog',kind:'library-playlist-tracks',id:'p.list'});
const page=bridge({op:'catalog',kind:'library-playlist-tracks',id:'p.list',next:path+'?offset=1'});
await new Promise(setImmediate);
const playing=bridge({op:'play-collection',resourceType:'library-playlists',id:'p.list',startID:'i.1'});
await new Promise(setImmediate);
assert.equal(reads,2,'playback must join the browser page request');
assert.equal(lyrics,1,'selected song lyrics must not wait for the rest of a playlist');assert.equal(queued,0);
finish();await page;assert.equal((await playing).data,true);assert.equal(queued,1);`, map[string]string{"op": "snapshot"})
}

func TestBridgeSameCollectionSkipsQueueRebuildButHonorsInvalidation(t *testing.T) {
	for _, change := range []string{"none", "queue", "refresh", "storefront", "logout", "expired", "missing"} {
		t.Run(change, func(t *testing.T) {
			runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let queued=0,now=0;Date.now=()=>now;
const songs=['101','102'].map(id=>({id,type:'songs',attributes:{hasLyrics:false,playParams:{id,kind:'song'}}}));
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:false,api:{music:async()=>({data:{data:songs}})},
setQueue:async()=>{queued++;return mk.queue={items:[...songs]}},
changeToMediaAtIndex:async index=>{mk.nowPlayingItem=mk.queue.items[index]},play:async()=>{}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const q={op:'play-collection',resourceType:'albums',id:'album',startID:'101'};
assert.equal((await bridge(q)).data,true);assert.equal(queued,1);
switch('`+change+`'){
case 'queue':mk.queue={items:[...songs]};break;
case 'refresh':await bridge({op:'catalog',kind:'album-tracks',id:'album',refresh:true});break;
case 'storefront':mk.storefrontId='jp';break;
case 'logout':mk.isAuthorized=false;await bridge({op:'snapshot'});mk.isAuthorized=true;break;
case 'expired':now=60001;break;
case 'missing':mk.queue.items.pop();break;
}
assert.equal((await bridge({...q,startID:'102'})).data,true);
assert.equal(queued,'`+change+`'==='none'?1:2,'only a valid same-collection queue may be reused');
assert.equal(mk.nowPlayingItem.id,'102');`, map[string]string{"op": "snapshot"})
		})
	}
}

func TestBridgeIncompleteCatalogPagesDoNotBecomePartialQueues(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const song={id:'101',type:'songs',attributes:{hasLyrics:false,playParams:{id:'101',kind:'song'}}};
const mk={isAuthorized:true,storefrontId:'us',api:{music:async()=>({data:{data:[song],next:'/v1/catalog/us/albums/album/tracks?offset=1'}})},
setQueue:async options=>{assert.equal(options.album,'album');assert.equal(options.items,undefined);return mk.queue={items:[song,{id:'102'}]}},
changeToMediaAtIndex:async index=>assert.equal(index,1),play:async()=>{}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
await bridge({op:'catalog',kind:'album-tracks',id:'album'});
assert.equal((await bridge({op:'play-collection',resourceType:'albums',id:'album',startID:'102'})).data,true);`, map[string]string{"op": "snapshot"})
}

func TestBridgeQueueJumpDoesNotPlayTwice(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let jumps=0;
const mk={isAuthorized:true,storefrontId:'us',queue:{items:[{id:'101',attributes:{hasLyrics:false}}]},
changeToMediaAtIndex:async()=>{jumps++},play:async()=>{throw Error('duplicate playback')}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
assert.equal((await bridge({op:'jump',id:'101',value:0})).data,true);assert.equal(jumps,1);`, map[string]string{"op": "snapshot"})
}

func TestBridgePlaybackTimingSeparatesSDKStagesWithoutExportingResources(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let clock=0;globalThis.performance={now:()=>clock};
const mk={isAuthorized:true,storefrontId:'us',setQueue:async()=>{clock+=120;return mk.queue={items:[{id:'101'}]}},
changeToMediaAtIndex:async()=>{clock+=250}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const result=await bridge({op:'play-collection',resourceType:'albums',id:'private-album',startID:'101'});
assert.equal(result.data,true);assert.ok(result.timing,'missing playback stage timing');assert.equal(result.timing.setQueueMs,120);assert.equal(result.timing.changeMs,250);
assert.equal(result.timing.totalMs,370);assert.equal(result.timing.reusedQueue,false);
assert.ok(!JSON.stringify(result).includes('private-album'));
const again=await bridge({op:'play-collection',resourceType:'albums',id:'private-album',startID:'101'});
assert.equal(again.timing.setQueueMs,0);assert.equal(again.timing.reusedQueue,true);`, map[string]string{"op": "snapshot"})
}

func TestBridgeRefreshDuringPlaybackDoesNotRestoreOldQueueProvenance(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let finish,queued=0,block=true;
const songs=['101','102'].map(id=>({id,type:'songs',attributes:{hasLyrics:false,playParams:{id,kind:'song'}}}));
const mk={isAuthorized:true,storefrontId:'us',api:{music:async()=>({data:{data:songs}})},
setQueue:async()=>{queued++;return mk.queue={items:songs}},changeToMediaAtIndex:async()=>{if(block){block=false;await new Promise(resolve=>finish=resolve)}}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const q={op:'play-collection',resourceType:'albums',id:'album',startID:'101'};
const first=bridge(q);await new Promise(setImmediate);
await bridge({op:'catalog',kind:'album-tracks',id:'album',refresh:true});finish();await first;
assert.equal((await bridge({...q,startID:'102'})).data,true);
assert.equal(queued,2,'a refresh during loading must also invalidate future queue reuse');`, map[string]string{"op": "snapshot"})
}

func TestBridgeSkipLyricsOverlapNativeAudioWithoutPredictingQueueBoundaries(t *testing.T) {
	for _, op := range []string{"next", "previous"} {
		runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let lyrics=0,finish,block=true;
const target={id:'101',type:'songs'};
const skip=async()=>{if(block){block=false;await new Promise(resolve=>finish=resolve)}};
const mk={isAuthorized:true,storefrontId:'us',queue:{nextPlayableItem:target,previousPlayableItem:target},
skipToNextItem:skip,skipToPreviousItem:skip,api:{music:async path=>{
assert.equal(path,'/v1/catalog/us/songs/101/syllable-lyrics');lyrics++;return {data:{data:[{attributes:{ttml:'<tt>fixture</tt>'}}]}};
}}};globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const playing=bridge({op:'`+op+`'});await new Promise(setImmediate);
assert.equal(lyrics,1,'lyrics should run during native audio loading, not after it');finish();assert.equal((await playing).data,true);
mk.queue={};assert.equal((await bridge({op:'`+op+`'})).data,true);
assert.equal(lyrics,1,'unknown targets and queue boundaries must not invent another song');`, map[string]string{"op": "snapshot"})
	}
}

func TestBridgeTimeoutTimingIncludesTheBlockedStage(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let clock=0;globalThis.performance={now:()=>clock};
const realTimer=setTimeout;globalThis.setTimeout=(fn)=>realTimer(()=>{clock=8000;fn()},10);
const mk={isAuthorized:true,storefrontId:'us',setQueue:async()=>new Promise(()=>{})};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const result=await bridge({op:'play',id:'101'});
assert.equal(result.error,'timeout');assert.equal(result.timing.totalMs,8000);
assert.equal(result.timing.setQueueMs,8000,'a timed-out stage must not look free');`, map[string]string{"op": "snapshot"})
}

func TestBridgeCollectionSelectionRejectsSessionChangesDuringAudioLoad(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const mk={isAuthorized:true,storefrontId:'us',autoplayEnabled:false,
setQueue:async()=>mk.queue={items:[{id:'101'}]},changeToMediaAtIndex:async()=>{mk.storefrontId='jp'}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const result=await bridge({op:'play-collection',resourceType:'albums',id:'album',startID:'101'});
assert.equal(result.error,'stale');assert.equal(mk.autoplayEnabled,false);`, map[string]string{"op": "snapshot"})
}
