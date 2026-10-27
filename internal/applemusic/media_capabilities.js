async (request) => {
  if (location.origin !== 'https://music.apple.com') return {error:'origin'};
  if (request.requireAuthorization) {
    const mk = globalThis.MusicKit?.getInstance();
    if (!mk) return {error:'loading'};
    if (!mk.isAuthorized) return {error:'login'};
  }
  if (request.skipIfPlaying && globalThis.MusicKit?.getInstance()?.isPlaying) return {data:{playing:true}};
  const key = Symbol.for('tapefox.mediaCapabilities');
  let cached = globalThis[key];
  if (!cached || Date.now() >= cached.expires) {
    cached = {expires:Date.now() + 5000};
    globalThis[key] = cached;
    cached.promise = (async () => {
      const result = {aac:false, widevine:false, reason:''};
      result.aac = !!document.createElement('audio').canPlayType('audio/mp4; codecs="mp4a.40.2"');
      if (!result.aac) { result.reason='codec'; return result; }
      if (!globalThis.isSecureContext || !globalThis.navigator?.requestMediaKeySystemAccess) {
        result.reason='eme'; return result;
      }
      let timer;
      try {
        await Promise.race([
          navigator.requestMediaKeySystemAccess('com.widevine.alpha', [{
            initDataTypes:['cenc'], audioCapabilities:[{contentType:'audio/mp4; codecs="mp4a.40.2"'}],
            distinctiveIdentifier:'optional', persistentState:'optional', sessionTypes:['temporary']
          }]),
          new Promise((_, reject) => { timer=setTimeout(() => reject({name:'Timeout'}), 3000); })
        ]);
        result.widevine=true;
        cached.expires=Date.now() + 60000;
      } catch (error) {
        // Export a fixed code, never an exception message or session data.
        result.reason=error?.name === 'Timeout' ? 'timeout' : error?.name === 'NotAllowedError' ? 'blocked' : 'widevine';
      } finally { clearTimeout(timer); }
      return result;
    })();
  }
  return {data:await cached.promise};
}
