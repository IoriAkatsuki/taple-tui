package applemusic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBridgeCatalogSearch(t *testing.T) {
	for _, kind := range []string{"songs", "albums", "artists"} {
		t.Run(kind, func(t *testing.T) {
			query := `"; throw new Error('injected'); // 歌曲`
			output := runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({storefrontId:'us',isAuthorized:false,
get musicUserToken(){throw Error('DO_NOT_EXPORT')},get developerToken(){throw Error('DO_NOT_EXPORT')},
api:{music:async(path,args)=>{
  assert.equal(path,'/v1/catalog/us/search');
  assert.equal(args.types,'`+kind+`');
  assert.equal(args.limit,25);
  return {data:{results:{[args.types]:{data:[{type:args.types,id:'opaque.001',
    attributes:{name:args.term,artistName:'Artist',albumName:'Album',durationInMillis:90000}}],
    next:'/v1/catalog/us/search?offset=25'}}}};
}}})}`, map[string]string{"op": "catalog", "kind": kind, "query": query})
			var got struct {
				Data  CatalogPage
				Error string
			}
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			if got.Error != "" || len(got.Data.Items) != 1 || got.Data.Next != "/v1/catalog/us/search?offset=25" {
				t.Fatalf("catalog search: %s", output)
			}
			item := got.Data.Items[0]
			if item.Type != kind || item.ID != "opaque.001" || item.Name != query || item.Artist != "Artist" || item.Album != "Album" || item.Duration != 90 {
				t.Fatalf("catalog item: %+v", item)
			}
		})
	}
}

func TestBridgeCatalogRelationshipsPreserveOpaqueIDsAndResourceTypes(t *testing.T) {
	for _, tc := range []struct {
		kind, path, resource string
	}{
		{CatalogAlbumTracks, "/albums/", "music-videos"},
		{CatalogArtistAlbums, "/artists/", "albums"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			output := runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({storefrontId:'jp',api:{music:async(path,args)=>{
  assert.equal(path,'/v1/catalog/jp`+tc.path+`opaque.001%2Fa%3F%23`+map[string]string{CatalogAlbumTracks: "/tracks", CatalogArtistAlbums: "/albums"}[tc.kind]+`');
  assert.deepEqual(args,{limit:25});
  return {data:{data:[{type:'`+tc.resource+`',id:'0001',attributes:{name:'Resource'}}]}};
}}})}`, map[string]string{"op": "catalog", "kind": tc.kind, "id": "opaque.001/a?#"})
			var got struct {
				Data  CatalogPage
				Error string
			}
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			if got.Error != "" || len(got.Data.Items) != 1 || got.Data.Items[0].Type != tc.resource || got.Data.Items[0].ID != "0001" || got.Data.Next != "" {
				t.Fatalf("relationship resource: %s", output)
			}
		})
	}
}

func TestBridgeCatalogFollowsOnlyReturnedPage(t *testing.T) {
	for _, tc := range []struct {
		kind, path string
	}{
		{CatalogSongs, "/search"},
		{CatalogAlbums, "/search"},
		{CatalogArtists, "/search"},
		{CatalogAlbumTracks, "/albums/opaque.001/tracks"},
		{CatalogArtistAlbums, "/artists/opaque.001/albums"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			next := "/v1/catalog/us" + tc.path + "?offset=25"
			output := runBridge(t, `import assert from 'node:assert/strict';
globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({storefrontId:'us',api:{music:async(path,args)=>{
  assert.equal(path,'`+next+`');
  assert.deepEqual(args,`+map[bool]string{true: `{term:'q',types:'` + tc.kind + `',limit:25}`, false: `{limit:25}`}[tc.path == "/search"]+`);
  const page={data:[{type:'songs',id:'second-page',attributes:{name:'Page Two'}}]};
  return {data:`+map[bool]string{true: `{results:{['` + tc.kind + `']:page}}`, false: `page`}[tc.path == "/search"]+`};
}}})}`, map[string]string{"op": "catalog", "kind": tc.kind, "query": "q", "id": "opaque.001", "next": next})
			var got struct {
				Data  CatalogPage
				Error string
			}
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			if got.Error != "" || len(got.Data.Items) != 1 || got.Data.Items[0].ID != "second-page" || got.Data.Next != "" {
				t.Fatalf("catalog pagination: %s", output)
			}
		})
	}
}

func TestBridgeCatalogEmptyPagesAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name, kind, response string
		wantError            bool
	}{
		{"empty-search", CatalogSongs, `return {data:{results:{songs:{data:[]}}}}`, false},
		{"missing-search-kind", CatalogArtists, `return {data:{results:{}}}`, false},
		{"empty-relationship", CatalogAlbumTracks, `return {data:{data:[]}}`, false},
		{"api-error", CatalogAlbums, `return {errors:[{detail:'DO_NOT_EXPORT'}]}`, true},
		{"nested-api-error", CatalogArtistAlbums, `return {data:{errors:[{detail:'DO_NOT_EXPORT'}]}}`, true},
		{"exception", CatalogSongs, `throw Error('DO_NOT_EXPORT')`, true},
		{"missing-response", CatalogSongs, `return undefined`, true},
		{"missing-results", CatalogSongs, `return {data:{}}`, true},
		{"null-search-kind", CatalogSongs, `return {data:{results:{songs:null}}}`, true},
		{"missing-relationship-data", CatalogAlbumTracks, `return {data:{}}`, true},
		{"malformed-items", CatalogArtists, `return {data:{results:{artists:{data:{}}}}}`, true},
		{"malformed-next", CatalogSongs, `return {data:{results:{songs:{data:[],next:42}}}}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({storefrontId:'us',api:{music:async()=>{`+tc.response+`}}})}`,
				map[string]string{"op": "catalog", "kind": tc.kind, "query": "q", "id": "opaque.001"})
			var got struct {
				Data  CatalogPage
				Error string
			}
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			if (got.Error != "") != tc.wantError || strings.Contains(string(output), "DO_NOT_EXPORT") {
				t.Fatalf("response error=%v: %s", tc.wantError, output)
			}
			if !tc.wantError && (got.Data.Items == nil || len(got.Data.Items) != 0 || got.Data.Next != "") {
				t.Fatalf("empty page: %s", output)
			}
		})
	}
}

func TestBridgeCatalogFollowsMatchingSearchParameters(t *testing.T) {
	query := `雪 & + "? #`
	next := "/v1/catalog/us/search?" + url.Values{"term": {query}, "types": {CatalogSongs}, "offset": {"25"}}.Encode()
	for _, page := range []string{"", next} {
		request := map[string]string{"op": "catalog", "kind": CatalogSongs, "query": query, "next": page}
		encoded, _ := json.Marshal(request)
		output := runBridge(t, `import assert from 'node:assert/strict';
const expected=`+string(encoded)+`;
globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({storefrontId:'us',api:{music:async(path,args)=>{
  assert.equal(path,expected.next || '/v1/catalog/us/search');
  assert.deepEqual(args,{term:expected.query,types:'songs',limit:25});
  return {data:{results:{songs:{data:[]}}}};
}}})}`, request)
		if string(output) != "{\"data\":{\"items\":[],\"next\":\"\"}}\n" {
			t.Fatalf("search parameter continuation: %s", output)
		}
	}
}

func TestBridgeCatalogRejectsForeignOrMalformedPages(t *testing.T) {
	for _, next := range []string{
		"https://evil.example/v1/catalog/us/search",
		"https://api.music.apple.com/v1/catalog/us/search",
		"//evil.example/v1/catalog/us/search",
		"/v1/catalog/jp/search?offset=25",
		"/v1/catalog/us/albums/other/tracks?offset=25",
		"/v1/me/library/songs",
		"/v1/catalog/us/../us/search?offset=25",
		"/v1/catalog/us/%2e%2e/us/search?offset=25",
		"/v1/catalog/us/search#fragment",
		"/v1/catalog/us/search#",
		"/v1/catalog/us/search\\?offset=25",
		"/v1/catalog/us/search?offset=25\n",
		"/v1/catalog/us/search?types=artists&term=q",
		"/v1/catalog/us/search?types=songs&term=other",
		"/v1/catalog/us/search?types=songs&types=artists",
	} {
		for _, returned := range []bool{false, true} {
			t.Run(fmt.Sprintf("%t/%s", returned, next), func(t *testing.T) {
				encodedNext, _ := json.Marshal(next)
				body := `console.log('unexpected-api-call'); return {data:{results:{songs:{data:[]}}}};`
				request := map[string]string{"op": "catalog", "kind": CatalogSongs, "query": "q", "next": next}
				if returned {
					delete(request, "next")
					body = `return {data:{results:{songs:{data:[],next:` + string(encodedNext) + `}}}};`
				}
				output := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({storefrontId:'us',api:{music:async()=>{`+body+`}}})}`, request)
				var got struct{ Error string }
				if err := json.Unmarshal(output, &got); err != nil || got.Error != "request" {
					t.Fatalf("unsafe catalog page: %s, %v", output, err)
				}
			})
		}
	}
	for _, tc := range []struct {
		kind, next string
	}{
		{CatalogAlbumTracks, "/v1/catalog/us/albums/other/tracks"},
		{CatalogAlbumTracks, "/v1/catalog/us/albums/opaque.001/relationships/tracks"},
		{CatalogArtistAlbums, "/v1/catalog/us/artists/other/albums"},
	} {
		output := runBridge(t, `globalThis.location={origin:'https://music.apple.com'};
globalThis.MusicKit={getInstance:()=>({storefrontId:'us',api:{music:async()=>{console.log('unexpected-api-call')}}})}`,
			map[string]string{"op": "catalog", "kind": tc.kind, "id": "opaque.001", "next": tc.next})
		if string(output) != "{\"error\":\"request\"}\n" {
			t.Fatalf("cross-relationship page: %s", output)
		}
	}
}

func TestBridgeCatalogRetainsOriginAndLoadingGuards(t *testing.T) {
	for _, tc := range []struct{ setup, want string }{
		{`globalThis.location={origin:'https://evil.example'}`, "origin"},
		{`globalThis.location={origin:'https://music.apple.com'}`, "loading"},
		{`globalThis.location={origin:'https://music.apple.com'};globalThis.MusicKit={getInstance:()=>({})}`, "loading"},
	} {
		output := runBridge(t, tc.setup, map[string]string{"op": "catalog", "kind": CatalogSongs, "query": "q"})
		if string(output) != fmt.Sprintf("{\"error\":%q}\n", tc.want) {
			t.Fatalf("catalog guard: %s", output)
		}
	}
}

func TestCatalogRejectsInvalidArgumentsBeforeBrowserIO(t *testing.T) {
	requests := []CatalogRequest{
		{}, {Kind: "play"}, {Kind: CatalogSongs}, {Kind: CatalogAlbums, Query: " \n\t "},
		{Kind: CatalogArtists, Query: strings.Repeat("界", 171)},
		{Kind: CatalogAlbumTracks}, {Kind: CatalogArtistAlbums, ID: strings.Repeat("a", 257)},
	}
	for _, control := range []string{"\x00", "\n", "\r", "\t", "\x1b", "\x7f", "\u0085"} {
		requests = append(requests, CatalogRequest{Kind: CatalogAlbumTracks, ID: "id" + control})
	}
	for _, next := range []string{
		"https://api.music.apple.com/v1/catalog/us/search",
		"//api.music.apple.com/v1/catalog/us/search",
		"/v1/me/library/songs", "/v1/catalog", "/v1/catalogue/us/search",
		"/v1/catalog/../me/library/songs", "/v1/catalog/%2e%2e/me/library/songs",
		"/v1/catalog/us/search#", "/v1/catalog/us/search#fragment",
		"/v1/catalog/us/search\\?offset=25", "/v1/catalog/us/search?offset=25\n",
		"/v1/catalog/us/search?offset=%invalid",
	} {
		requests = append(requests, CatalogRequest{Kind: CatalogSongs, Query: "q", Next: next})
	}
	for _, request := range requests {
		if _, err := (&Browser{}).Catalog(context.Background(), request); err == nil {
			t.Errorf("accepted %+v", request)
		}
	}
}

func TestCatalogAcceptsSupportedKindsAndStopsBeforeClosedBrowserIO(t *testing.T) {
	for _, request := range []CatalogRequest{
		{Kind: CatalogSongs, Query: strings.Repeat("a", 512)},
		{Kind: CatalogAlbums, Query: " Album "},
		{Kind: CatalogArtists, Query: "Artist", Next: "/v1/catalog/us/search?offset=25"},
		{Kind: CatalogAlbumTracks, ID: "opaque.001/a?#"},
		{Kind: CatalogArtistAlbums, ID: strings.Repeat("a", 256)},
	} {
		if _, err := (&Browser{stopped: true}).Catalog(context.Background(), request); err == nil || err.Error() != "播放器已关闭" {
			t.Errorf("supported request rejected or closed browser reached: %+v, %v", request, err)
		}
	}
}

func TestCatalogBrowserTransportUsesEncodedReadOnlyRequest(t *testing.T) {
	input, serverOut := io.Pipe()
	serverIn, output := io.Pipe()
	b := &Browser{pipe: newPipe(input, output), background: true}
	defer b.pipe.close()
	defer serverIn.Close()
	defer serverOut.Close()
	query := `"; throw Error('injected'); //`
	want := CatalogPage{Items: []CatalogItem{{Type: CatalogSongs, ID: "0001", Name: query}}, Next: "/v1/catalog/us/search?offset=25"}
	checked := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(serverIn)
		for {
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
			switch request.Method {
			case "Target.getTargets":
				result = map[string]any{"targetInfos": []any{map[string]string{"targetId": "page", "type": "page", "url": "https://music.apple.com/us/browse"}}}
			case "Target.attachToTarget":
				result = map[string]string{"sessionId": "session"}
			case "Runtime.evaluate":
				wantScript, _ := scriptFor(struct {
					Op string `json:"op"`
					CatalogRequest
				}{"catalog", CatalogRequest{Kind: CatalogSongs, Query: query}})
				if request.Params.UserGesture || request.Params.Expression != wantScript {
					checked <- fmt.Errorf("incorrect read-only request")
					return
				}
				result = map[string]any{"result": map[string]any{"value": map[string]any{"data": want}}}
			default:
				checked <- fmt.Errorf("unexpected method %q", request.Method)
				return
			}
			response, _ := json.Marshal(map[string]any{"id": request.ID, "result": result})
			if _, err := serverOut.Write(append(response, 0)); err != nil {
				checked <- err
				return
			}
			if request.Method == "Runtime.evaluate" {
				checked <- nil
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := b.Catalog(ctx, CatalogRequest{Kind: CatalogSongs, Query: " " + query + " "})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog transport: %+v, %v", got, err)
	}
	if err := <-checked; err != nil {
		t.Fatal(err)
	}
}
