package applemusic

import "testing"

func TestBridgeCatalogSharesInflightReadsAndRefreshesPages(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let reads=0,finish,now=0;
Date.now=()=>now;
const mk={isAuthorized:true,storefrontId:'us',api:{music:async(path,args)=>{
reads++;if(reads===1) await new Promise(resolve=>finish=resolve);
return {data:{results:{songs:{data:[{id:String(reads),type:'songs',attributes:{name:args.term}}]}}}};
}}};globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const q={op:'catalog',kind:'songs',query:'fixture'};
const a=bridge(q),b=bridge(q);await new Promise(setImmediate);
assert.equal(reads,1,'duplicate in-flight reads must be shared');finish();
assert.deepEqual(await a,await b);await bridge(q);assert.equal(reads,1);
const next={...q,next:'/v1/catalog/us/search?offset=25'};await bridge(next);assert.equal(reads,2);
await bridge({...q,refresh:true});assert.equal(reads,3);
await bridge(next);assert.equal(reads,4,'refresh must invalidate old pagination too');
now=60001;await bridge(q);assert.equal(reads,5,'expired pages must be read again');
await bridge({...q,query:'different'});assert.equal(reads,6,'different searches must not collide');`, map[string]string{"op": "snapshot"})
}

func TestBridgeCatalogCacheRejectsFailuresAndInvalidatesSessions(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
let reads=0;
const mk={isAuthorized:true,storefrontId:'us',api:{music:async()=>{
reads++;if(reads===1)return {data:{errors:[{status:500}]}};
return {data:{data:[]}};
}}};globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const q={op:'catalog',kind:'library-playlists'};
assert.equal((await bridge(q)).error,'request');assert.deepEqual((await bridge(q)).data.items,[]);
await bridge(q);assert.equal(reads,2,'successful empty pages must be cached');
mk.isAuthorized=false;assert.equal((await bridge(q)).error,'login');
mk.isAuthorized=true;await bridge(q);assert.equal(reads,3);
mk.storefrontId='jp';await bridge(q);assert.equal(reads,4);
globalThis.MusicKit={getInstance:()=>({...mk})};await bridge(q);assert.equal(reads,5);`, map[string]string{"op": "snapshot"})
}

func TestBridgeCatalogCacheIsBoundedAndTimeoutCanBeRetried(t *testing.T) {
	runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const realTimer=setTimeout;globalThis.setTimeout=(fn,ms)=>realTimer(fn,Math.min(ms,20));
let reads=0,aborted=false;
const mk={storefrontId:'us',api:{music:async(path,args,options)=>{
reads++;if(reads===1){options.fetchOptions.signal.addEventListener('abort',()=>aborted=true);return new Promise(()=>{})}
return {data:{results:{songs:{data:[]}}}};
}}};globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const q={op:'catalog',kind:'songs',query:'0'};
assert.equal((await bridge(q)).error,'timeout');assert.equal(aborted,true);
assert.deepEqual((await bridge(q)).data.items,[]);assert.equal(reads,2);
for(let i=1;i<=32;i++)await bridge({...q,query:String(i)});
await bridge(q);assert.equal(reads,35,'page cache must have a fixed upper bound');`, map[string]string{"op": "snapshot"})
}

func TestBridgeCachedCatalogRechecksSessionBeforeDelivery(t *testing.T) {
	for _, change := range []string{"mk.isAuthorized=false", "mk.storefrontId='jp'"} {
		runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
const mk={isAuthorized:true,storefrontId:'us',api:{music:async()=>({data:{data:[{id:'private-fixture',type:'library-playlists'}]}})}};
globalThis.MusicKit={getInstance:()=>mk};const bridge=`+bridgeJS+`;
const q={op:'catalog',kind:'library-playlists'};
await bridge(q);const hit=bridge(q);`+change+`;await bridge({op:'snapshot'});
const result=await hit;assert.ok(['login','stale'].includes(result.error));assert.equal(result.data,undefined);`, map[string]string{"op": "snapshot"})
	}
}
