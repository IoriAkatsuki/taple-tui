async (request) => {
  if (location.origin !== 'https://music.apple.com') return {error: 'origin'};
  const mk = globalThis.MusicKit?.getInstance();
  const cacheKey = Symbol.for('taple.appleMusicCache');
  let cache = globalThis[cacheKey];
  if (!mk || cache?.sdk !== mk || cache.storefront !== mk.storefrontId || cache.authorized !== !!mk.isAuthorized) {
    for (const entry of cache?.lyrics.values() || []) entry.abort.abort();
    for (const entry of cache?.pages.values() || []) entry.abort.abort();
    cache = {sdk:mk, storefront:mk?.storefrontId, authorized:!!mk?.isAuthorized, songs:new Map(), lyrics:new Map(), pages:new Map(), playGeneration:0};
    globalThis[cacheKey] = cache;
  }
  if (!mk) return request.op === 'snapshot' ? {data: {ready: false}} : {error: 'loading'};
  const number = (n) => Number.isFinite(n) ? n : 0;
  const validID = id => typeof id === 'string' && id.length > 0 && id.length <= 256 && !/[\u0000-\u0020\u007f-\u009f]/.test(id) && id !== '.' && id !== '..';
  const sameSession = () => globalThis[cacheKey] === cache && mk.storefrontId === cache.storefront && !!mk.isAuthorized === cache.authorized;
  const sessionStatus = () => !mk.isAuthorized ? 'login_required' :
    !sameSession() ? 'stale' : '';
  const collectionKinds = {albums:'album-tracks', playlists:'playlist-tracks', 'library-albums':'library-album-tracks', 'library-playlists':'library-playlist-tracks'};
  const readCatalog = (scope, key, refresh, read) => {
    if (refresh) {
      if (cache.collection?.scope === scope) cache.collection = null;
      for (const [id, entry] of cache.pages) {
        if (entry.scope === scope && (entry.done || id !== key)) {
          cache.pages.delete(id);
          entry.abort.abort();
        }
      }
    }
    let entry = cache.pages.get(key);
    if (entry?.done && Date.now() >= entry.expires) { cache.pages.delete(key); entry = null; }
    if (entry) {
      cache.pages.delete(key);
      cache.pages.set(key, entry);
      return entry.promise;
    }
    const abort = new AbortController();
    entry = {scope, abort, done:false};
    cache.pages.set(key, entry);
    while (cache.pages.size > 32) {
      const oldest = cache.pages.keys().next().value;
      cache.pages.get(oldest).abort.abort();
      cache.pages.delete(oldest);
    }
    let timer;
    const interrupted = new Promise(resolve => {
      abort.signal.addEventListener('abort', () => resolve({error:'stale'}), {once:true});
      timer = setTimeout(() => { resolve({error:'timeout'}); abort.abort(); }, 8000);
    });
    entry.promise = Promise.race([
      Promise.resolve().then(() => read(abort.signal)).catch(() => ({error:'request'})), interrupted
    ]).then(result => {
      clearTimeout(timer);
      entry.done = true;
      entry.expires = Date.now() + 60000;
      if (!sameSession() && !result.error) result = {error:'stale'};
      if ((result.error || abort.signal.aborted) && cache.pages.get(key) === entry) cache.pages.delete(key);
      return result;
    });
    return entry.promise;
  };
  const rememberSong = item => {
    if (sessionStatus() || !['songs', 'library-songs'].includes(item?.type) || !validID(item.id)) return;
    const key = item.type + ':' + item.id;
    cache.songs.delete(key);
    if (!item.attributes?.playParams) return;
    cache.songs.set(key, {item, at:Date.now()});
    while (cache.songs.size > 100) cache.songs.delete(cache.songs.keys().next().value);
  };
  const songResource = (id, type) => {
    const key = type + ':' + id;
    const entry = cache.songs.get(key);
    if (entry && Date.now() - entry.at < 300000) return entry.item;
    cache.songs.delete(key);
  };
  const enableAutoplay = () => {
    if (typeof mk.autoplayEnabled === 'boolean') mk.autoplayEnabled = true;
  };
  const collectionQueue = (scope, startIDs) => {
    const entry = cache.collection;
    if (scope && entry?.scope === scope && entry.queue === mk.queue && Date.now() < entry.expires &&
        entry.queue.items.some(item => startIDs.includes(String(item.id)))) return entry.queue;
  };
  const started = performance.now();
  const timing = {resourcesMs:0, setQueueMs:0, changeMs:0, playMs:0, reusedQueue:false};
  let activeStage = '', stageStarted = 0;
  const measure = async (stage, work) => {
    activeStage = stage;
    stageStarted = performance.now();
    try { return await work(); }
    finally { timing[stage] += performance.now() - stageStarted; activeStage = ''; }
  };
  const measured = result => {
    const now = performance.now(), sample = {...timing, totalMs:now - started};
    if (activeStage) sample[activeStage] += now - stageStarted;
    return {...result, timing:sample};
  };
  let expired = false;
  const playQueue = async (options, startIDs = [], scope = '') => {
    const generation = ++cache.playGeneration;
    let queue = collectionQueue(scope, startIDs);
    timing.reusedQueue = !!queue;
    const collection = scope ? {scope, queue:null, expires:0} : null;
    cache.collection = collection;
    // The SDK otherwise awaits recommendation requests before starting audio.
    if (!queue && typeof mk.autoplayEnabled === 'boolean') mk.autoplayEnabled = false;
    if (startIDs.length) {
      queue ||= await measure('setQueueMs', () => mk.setQueue({...options, startPlaying:false}));
      if (expired || !sameSession() || generation !== cache.playGeneration || !queue || mk.queue !== queue) return {error:'stale'};
      const index = queue.items.findIndex(item => startIDs.includes(String(item.id)));
      if (index < 0) return {error:'stale'};
      await measure('changeMs', () => mk.changeToMediaAtIndex(index));
      if (expired || !sameSession() || generation !== cache.playGeneration) return {error:'stale'};
    } else {
      queue = await measure('setQueueMs', () => mk.setQueue(options));
    }
    if (expired || !sameSession() || generation !== cache.playGeneration) return {error:'stale'};
    // Queue changes may resolve before audio starts. Await the play promise if
    // the SDK still reports stopped/paused, surfacing codec and DRM failures.
    if (mk.isPlaying === false) {
      if (typeof mk.play !== 'function') return {error:'loading'};
      await measure('playMs', () => mk.play());
    }
    if (expired || !sameSession() || generation !== cache.playGeneration) return {error:'stale'};
    if (collection && cache.collection === collection && queue && mk.queue === queue) {
      collection.queue = queue;
      collection.expires = Date.now() + 60000;
    }
    enableAutoplay();
    return {data:true};
  };
  const forgetLyrics = entry => {
    for (const [key, value] of cache.lyrics) if (value === entry) cache.lyrics.delete(key);
  };
  const rememberLyrics = (key, entry) => {
    cache.lyrics.delete(key);
    cache.lyrics.set(key, entry);
    while (cache.lyrics.size > 8) {
      const oldest = cache.lyrics.keys().next().value;
      const removed = cache.lyrics.get(oldest);
      cache.lyrics.delete(oldest);
      if (![...cache.lyrics.values()].includes(removed)) removed.abort.abort();
    }
  };
  // Only explicit playback preloads; foreground reads share the same bounded promise.
  const loadLyrics = async (item, id, refresh = false) => {
    const session = sessionStatus();
    if (session) return {status:session};
    if (!validID(id) || !validID(cache.storefront)) return {status:'invalid_request'};
    if (item?.type && !['song', 'songs', 'library-songs'].includes(item.type)) return {status:'unsupported_media'};
    if (item?.attributes?.hasLyrics === false) return {status:'no_lyrics'};
    const api = mk.api?.v3 ?? mk.api;
    if (typeof api?.music !== 'function') return {status:'sdk_unavailable'};
    const params = item?.attributes?.playParams ?? item?.playParams ?? songResource(id, 'library-songs')?.attributes?.playParams ?? {};
    let catalogID = params.catalogId == null ? '' : String(params.catalogId);
    const library = item?.type === 'library-songs' || params.isLibrary === true || id.startsWith('i.');
    if (!catalogID && !library && /^\d+$/.test(id)) catalogID = id;
    const key = catalogID || id;
    let entry = cache.lyrics.get(key);
    if (entry && refresh && entry.done) { forgetLyrics(entry); entry = null; }
    if (entry) {
      rememberLyrics(key, entry);
      if (id !== key) rememberLyrics(id, entry);
      const source = entry.done ? 'hit' : 'shared';
      return {...await entry.promise, cache:source};
    }
    const abort = new AbortController();
    entry = {abort, done:false};
    rememberLyrics(key, entry);
    if (id !== key) rememberLyrics(id, entry);
    const failure = code => ({status:({401:'unauthorized',403:'access_denied',404:'unavailable',429:'rate_limited'})[code] || 'request_error'});
    const check = result => {
      const session = sessionStatus();
      if (session || abort.signal.aborted) return {status:session || 'stale'};
      const code = Number(result?.status || result?.data?.errors?.[0]?.status || result?.errors?.[0]?.status);
      return code >= 400 || result?.data?.errors || result?.errors ? failure(code) : null;
    };
    const read = async () => {
      try {
        if (!catalogID && library) {
          const mapped = await api.music('/v1/me/library/songs/' + encodeURIComponent(id), {include:'catalog'}, {fetchOptions:{signal:abort.signal}});
          const error = check(mapped);
          if (error) return error;
          const resource = mapped?.data?.data?.[0];
          rememberSong(resource);
          catalogID = String(resource?.relationships?.catalog?.data?.[0]?.id || '');
        }
        if (!validID(catalogID)) return {status:'no_catalog_mapping'};
        if (catalogID !== key) {
          let shared = cache.lyrics.get(catalogID);
          if (shared && refresh && shared.done) { forgetLyrics(shared); shared = null; }
          if (shared && shared !== entry) {
            rememberLyrics(id, shared);
            return await shared.promise;
          }
          rememberLyrics(catalogID, entry);
        }
        const result = await api.music('/v1/catalog/' + encodeURIComponent(cache.storefront) + '/songs/' + encodeURIComponent(catalogID) + '/syllable-lyrics', {}, {fetchOptions:{signal:abort.signal}});
        const error = check(result);
        if (error) return error;
        const attributes = result?.data?.data?.[0]?.attributes;
        const ttml = typeof attributes?.ttml === 'string' && attributes.ttml.trim() ? attributes.ttml : attributes?.ttmlLocalizations;
        if (typeof ttml !== 'string' || !ttml.trim()) return {status:'unavailable'};
        if (ttml.length > 2_000_000) return {status:'parse_error'};
        return {status:'available', catalogID, ttml};
      } catch (error) {
        return {status:sessionStatus() || failure(Number(error?.status || error?.response?.status)).status};
      }
    };
    let timer;
    const interrupted = new Promise(resolve => {
      abort.signal.addEventListener('abort', () => resolve({status:'stale'}), {once:true});
      timer = setTimeout(() => { resolve({status:'timeout'}); abort.abort(); }, 8000);
    });
    entry.promise = Promise.race([read(), interrupted]).then(data => {
      clearTimeout(timer);
      entry.done = true;
      if (data.status !== 'available' || sessionStatus() || abort.signal.aborted) forgetLyrics(entry);
      return data;
    });
    return {...await entry.promise, cache:'miss'};
  };
  const prefetchLyrics = (item, id) => { void loadLyrics(item, id).catch(() => {}); };
  const validPage = (next, path) => {
    if (typeof next !== 'string' || !next || /[\u0000-\u001f\u007f-\u009f\\#]/.test(next) || /%(?![\da-f]{2})/i.test(next)) return false;
    const url = new URL(next, 'https://api.music.apple.com');
    return url.origin === 'https://api.music.apple.com' && !url.username && !url.password && url.pathname === path && next.split('?')[0] === path;
  };
  const track = (item) => {
    const a = item?.attributes || {};
    return {id: String(item?.id || ''), type: String(item?.type || ''), playable: !!a.playParams, catalogId: String(a.playParams?.catalogId || ''),
      name: String(a.name || a.title?.stringForDisplay || ''), artist: String(a.artistName || a.curatorName || ''),
      album: String(a.albumName || ''), duration: number(a.durationInMillis) / 1000};
  };
  if (request.op === 'snapshot') {
    const items = Array.from(mk.queue?.items || []);
    const current = track(mk.nowPlayingItem);
    if (number(mk.currentPlaybackDuration) > 0) current.duration = mk.currentPlaybackDuration;
    const playbackType = mk.nowPlayingItem?.playbackType;
    const preview = !!mk.previewOnly || (MusicKit.PlaybackType?.preview !== undefined && playbackType === MusicKit.PlaybackType.preview);
    const full = !preview && ['encryptedFull', 'unencryptedFull'].some(kind =>
      MusicKit.PlaybackType?.[kind] !== undefined && playbackType === MusicKit.PlaybackType[kind]);
    const paused = MusicKit.PlaybackStates?.paused !== undefined && mk.playbackState === MusicKit.PlaybackStates.paused;
    return {data: {ready: true, authorized: !!mk.isAuthorized, playing: !!mk.isPlaying, paused,
      autoplayAvailable: typeof mk.autoplayEnabled === 'boolean', autoplay: mk.autoplayEnabled === true,
      preview, full, position: number(mk.currentPlaybackTime), volume: number(mk.volume),
      storefront: String(mk.storefrontId || ''), track: current, queue: items.slice(0, 50).map(track),
      queueIndex: number(mk.nowPlayingItemIndex), queueTotal: items.length}};
  }
  let timer;
  const collectionAbort = request.op === 'play-collection' || (['play','enqueue'].includes(request.op) && request.resourceType === 'library-songs') ? new AbortController() : null;
  try {
    const operation = async () => {
      if (request.op === 'lyrics') {
        const reply = (status, fields = {}) => ({data: {status, trackID: request.id, storefront: request.storefront, ...fields}});
        if (!mk.isAuthorized) return reply('login_required');
        if (!validID(request.id) || !validID(request.storefront)) return reply('invalid_request');
        const item = mk.nowPlayingItem;
        const stale = () => String(mk.nowPlayingItem?.id || '') !== request.id || mk.storefrontId !== request.storefront;
        if (stale()) return reply('stale');
        const result = await loadLyrics(item, request.id, request.refresh === true);
        if (!mk.isAuthorized) return reply('login_required');
        if (stale() || sessionStatus()) return reply('stale');
        return reply(result.status, result);
      }
      if (request.op === 'catalog') {
        if (!mk.storefrontId) return {error: 'loading'};
        const storefront = mk.storefrontId;
        const searching = ['songs', 'albums', 'artists'].includes(request.kind);
        const catalog = '/v1/catalog/' + encodeURIComponent(storefront);
        const routes = {
          'album-tracks':[catalog+'/albums/','/tracks'], 'artist-albums':[catalog+'/artists/','/albums'],
          'playlist-tracks':[catalog+'/playlists/','/tracks'],
          'library-playlist-tracks':['/v1/me/library/playlists/','/tracks'],
          'library-album-tracks':['/v1/me/library/albums/','/tracks'],
          'recommendation-contents':['/v1/me/recommendations/','/contents']
        };
        const roots = {'library-playlists':'/v1/me/library/playlists','library-albums':'/v1/me/library/albums',
          'library-songs':'/v1/me/library/songs','recommendations':'/v1/me/recommendations','personal-station':catalog+'/stations'};
        const relationship = routes[request.kind];
        const personal = request.kind.startsWith('library-') || ['recommendations','recommendation-contents','personal-station'].includes(request.kind);
        if (!searching && !relationship && !Object.hasOwn(roots,request.kind)) return {error: 'request'};
        if (personal && !mk.isAuthorized) return {error:'login'};
        const controls = /[\u0000-\u001f\u007f-\u009f]/;
        if (searching) {
          if (typeof request.query !== 'string' || !request.query.trim() || new TextEncoder().encode(request.query).length > 512) return {error: 'request'};
        } else if (relationship && (typeof request.id !== 'string' || !request.id || new TextEncoder().encode(request.id).length > 256 || controls.test(request.id))) {
          return {error: 'request'};
        }
        const path = searching ? catalog+'/search' : relationship ? relationship[0]+encodeURIComponent(request.id)+relationship[1] : roots[request.kind];
        const origin = 'https://api.music.apple.com';
        if (new URL(path, origin).pathname !== path) return {error: 'request'};
        const validNext = (next) => {
          if (!validPage(next,path)) return false;
          const url = new URL(next, origin);
          if (request.kind === 'personal-station' && !url.searchParams.getAll('filter[identity]').every(v=>v==='personal')) return false;
          return !searching || [['term', request.query], ['types', request.kind]].every(([key, value]) =>
            url.searchParams.getAll(key).every(item => item === value));
        };
        if (request.next !== undefined && request.next !== '' && !validNext(request.next)) return {error: 'request'};
        const args = searching ? {term: request.query, types: request.kind, limit: 25} :
          request.kind === 'personal-station' ? {'filter[identity]':'personal'} : {limit: request.kind === 'recommendations' ? 10 : 25};
        const scope = JSON.stringify([request.kind, request.query || '', request.id || '']);
        const key = JSON.stringify([scope, request.next || '']);
        const result = await readCatalog(scope, key, request.refresh === true, async signal => {
          const result = await mk.api.music(request.next || path, args, {fetchOptions:{signal}});
          if (personal && !mk.isAuthorized) return {error:'login'};
          if (!sameSession() || signal.aborted) return {error:'stale'};
          if (Number(result?.status) >= 400 || result?.errors || result?.data?.errors || !result?.data) return {error: 'request'};
          if (searching && (!result.data.results || typeof result.data.results !== 'object' || Array.isArray(result.data.results))) return {error: 'request'};
          const page = searching ? result.data.results[request.kind] : result.data;
          if (page === undefined && searching) return {data: {items: [], next: ''}};
          if (!Array.isArray(page?.data) || (page.next !== undefined && page.next !== '' && !validNext(page.next))) return {error: 'request'};
          for (const item of page.data) rememberSong(item);
          return {data: {items: page.data, next: page.next || ''}};
        });
        if (personal && !mk.isAuthorized) return {error:'login'};
        if (!sameSession()) return {error:'stale'};
        return result.error ? result : {data: {items: result.data.items.map(track), next: result.data.next}};
      }
      if (!mk.isAuthorized) return {error: 'login'};
      const sourceStorefront = mk.storefrontId;
      const readLibrary = async (path, args = {}) => {
        const result = await measure('resourcesMs', () => mk.api.music(path, args, {fetchOptions:{signal:collectionAbort.signal}}));
        if (collectionAbort.signal.aborted || !mk.isAuthorized || sourceStorefront !== mk.storefrontId || Number(result?.status) >= 400 || result?.errors || result?.data?.errors || !Array.isArray(result?.data?.data)) throw Error('library read failed');
        return result.data;
      };
      if (['play','enqueue'].includes(request.op) && request.resourceType === 'library-songs') {
        const path = '/v1/me/library/songs/' + encodeURIComponent(request.id);
        if (new URL(path,'https://api.music.apple.com').pathname !== path) return {error:'request'};
        let item = songResource(request.id, 'library-songs');
        if (!item) {
          const page = await readLibrary(path);
          item = page.data.find(item=>String(item.id)===request.id && item.type==='library-songs');
          rememberSong(item);
        }
        if (!item?.attributes?.playParams) return {error:'unplayable'};
        if (request.op === 'play') {
          prefetchLyrics(item, request.id);
          return playQueue({items:[item],startPlaying:true});
        }
        else await mk.playLater({items:[item]});
        return {data:true};
      }
      if (request.op === 'play-collection') {
        if (request.startID !== undefined && request.startID !== '' && !validID(request.startID)) return {error:'request'};
        const kind = collectionKinds[request.resourceType];
        if (!kind) return {error:'request'};
        const scope = JSON.stringify([kind, '', request.id]);
        const library = request.resourceType.startsWith('library-');
        let selected = (library && songResource(request.startID, 'library-songs')) || songResource(request.startID, 'songs');
        let startIDs = request.startID ? [request.startID, String(selected?.attributes?.playParams?.catalogId || '')].filter(validID) : [];
        if (request.startID && (selected || !library)) prefetchLyrics(selected, request.startID);
        if (collectionQueue(scope, startIDs)) return playQueue(null, startIDs, scope);
        const root = library ? '/v1/me/library/' : '/v1/catalog/' + encodeURIComponent(mk.storefrontId) + '/';
        const path = root + request.resourceType.replace('library-', '') + '/' + encodeURIComponent(request.id) + '/tracks';
        if (new URL(path,'https://api.music.apple.com').pathname !== path) return {error:'request'};
        const items = [], seen = new Set();
        let next = path;
        while (next) {
          if (!validPage(next,path) || seen.has(next)) return {error:'request'};
          seen.add(next);
          const key = JSON.stringify([scope, next === path ? '' : next]);
          const cached = cache.pages.get(key);
          // Catalog collections with incomplete cached pages stay on the SDK's native loader.
          if (!library && (!cached || (cached.done && Date.now() >= cached.expires))) {
            return playQueue({[request.resourceType === 'albums' ? 'album' : 'playlist']:request.id,startPlaying:true}, startIDs, scope);
          }
          const page = await measure('resourcesMs', () => readCatalog(scope, key, false, async signal => {
            const result = await mk.api.music(next, {limit:100}, {fetchOptions:{signal}});
            const page = result?.data;
            if (signal.aborted || !sameSession()) return {error:'stale'};
            if (Number(result?.status) >= 400 || result?.errors || page?.errors || !Array.isArray(page?.data) ||
                (page.next !== undefined && page.next !== '' && !validPage(page.next,path))) return {error:'request'};
            for (const item of page.data) rememberSong(item);
            return {data: {items:page.data, next:page.next || ''}};
          }));
          if (page.error) return page;
          if (expired || !sameSession() || collectionAbort.signal.aborted) return {error:'stale'};
          items.push(...page.data.items.filter(item=>['songs','library-songs'].includes(item.type) && item.attributes?.playParams));
          if (request.startID && !selected) {
            selected = page.data.items.find(item => String(item.id) === request.startID);
            if (selected) prefetchLyrics(selected, request.startID);
          }
          next = page.data.next;
        }
        if (!items.length) return {error:'unplayable'};
        if (request.startID) {
          const item = items.find(item => String(item.id) === request.startID);
          if (!item) return {error:'stale'};
          startIDs = [request.startID, String(item.attributes.playParams.catalogId || '')].filter(validID);
        }
        return playQueue({items,startPlaying:true}, startIDs, scope);
      }
      switch (request.op) {
        case 'play-station':
          if (request.resourceType !== 'stations') return {error:'request'};
          return playQueue({station:request.id,startPlaying:true});
        case 'play': {
          const item = songResource(request.id, 'songs');
          prefetchLyrics(item, request.id);
          return playQueue(item ? {items:[item], startPlaying:true} : {song:request.id, startPlaying:true});
        }
        case 'enqueue': {
          const item = songResource(request.id, 'songs');
          await mk.playLater(item ? {items:[item]} : {song:request.id}); break;
        }
        case 'toggle':
          if (mk.isPlaying) await mk.pause();
          else { await measure('playMs', () => mk.play()); if (!expired && sameSession()) enableAutoplay(); }
          break;
        case 'next':
        case 'previous': {
          const repeatOne = MusicKit.PlayerRepeatMode?.one !== undefined && mk.repeatMode === MusicKit.PlayerRepeatMode.one;
          const item = repeatOne ? mk.nowPlayingItem : request.op === 'next' ? mk.queue?.nextPlayableItem : mk.queue?.previousPlayableItem;
          if (item?.id) prefetchLyrics(item, String(item.id));
          await measure('changeMs', () => request.op === 'next' ? mk.skipToNextItem() : mk.skipToPreviousItem());
          break;
        }
        case 'seek':
          if (!request.id || String(mk.nowPlayingItem?.id || '') !== request.id) return {error: 'stale'};
          await mk.seekToTime(request.value);
          break;
        case 'volume': mk.volume = request.value; break;
        case 'jump':
          if (!Number.isInteger(request.value) || request.value < 0 || request.value >= (mk.queue?.items?.length || 0)) return {error: 'request'};
          if (String(mk.queue.items[request.value].id) !== request.id) return {error: 'request'};
          prefetchLyrics(mk.queue.items[request.value], request.id);
          await measure('changeMs', () => mk.changeToMediaAtIndex(request.value));
          if (expired || !sameSession()) return {error:'stale'};
          if (!expired && sameSession()) enableAutoplay();
          break;
        default: return {error: 'request'};
      }
      return {data: true};
    };
    if (request.op === 'lyrics' || request.op === 'catalog') return await operation();
    const result = await Promise.race([operation(), new Promise(resolve => {
      timer = setTimeout(() => {
        expired = true;
        collectionAbort?.abort();
        resolve({error:'timeout'});
      }, 8000);
    })]);
    return measured(result);
  } catch (error) {
    // Do not export website exception details: they may contain session data.
    return ['catalog','lyrics'].includes(request.op) ? {error:'request'} : measured({error:error?.name === 'NotSupportedError' ? 'media_unsupported' : 'request'});
  } finally {
    clearTimeout(timer);
  }
}
