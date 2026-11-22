package ui

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

func TestAppleMusicSelectedCollectionTrackKeepsQueueContext(t *testing.T) {
	for kind, resource := range map[string]string{
		applemusic.CatalogAlbumTracks: "albums", applemusic.CatalogPlaylistTracks: "playlists",
		applemusic.LibraryAlbumTracks: "library-albums", applemusic.LibraryPlaylistTracks: "library-playlists",
	} {
		t.Run(kind, func(t *testing.T) {
			c := &appleMouseClient{}
			p := NewAppleMusicPage(c)
			defer p.Close()
			p.level = motion.Off
			p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
			p.selected = 1
			p.tracks = []applemusic.Track{{ID: "first", Type: "songs", Playable: true}, {ID: "i.second", Type: "library-songs", CatalogID: "002", Playable: true}}
			p.catalog.request = applemusic.CatalogRequest{Kind: kind, ID: "collection"}
			cmd := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd == nil || p.pendingID != "i.second" || p.pendingCatalogID != "002" {
				t.Fatalf("selected song identity was lost: target=%q catalog=%q", p.pendingID, p.pendingCatalogID)
			}
			p.Update(cmd(), model.NewApp(model.DefaultOptions()))
			want := applemusic.Action{Kind: "play-collection", ID: "collection", ResourceType: resource, StartID: "i.second"}
			if len(c.actions) != 1 || c.actions[0] != want {
				t.Fatalf("selection discarded album/playlist context: %+v", c.actions)
			}
			p.acceptState(applemusic.State{Ready: true, Authorized: true, Playing: true, Full: true, Storefront: "us", Track: applemusic.Track{ID: "002"}}, time.Now())
			if !p.ready() {
				t.Fatal("real selected song could not confirm collection playback")
			}
		})
	}
}

func TestAppleMusicRapidSelectionsCoalesceOnlyUnsentAdjacentTargets(t *testing.T) {
	for _, barrier := range []bool{false, true} {
		c := &appleMouseClient{}
		p := NewAppleMusicPage(c)
		p.level = motion.Off
		first := p.dispatch(applemusic.Action{Kind: "play", ID: "a"})
		p.dispatch(applemusic.Action{Kind: "play", ID: "b"})
		if barrier {
			p.dispatch(applemusic.Action{Kind: "enqueue", ID: "keep"})
		}
		p.dispatch(applemusic.Action{Kind: "play", ID: "c"})
		p.dispatch(applemusic.Action{Kind: "play", ID: "d"})
		for cmd := first; cmd != nil; {
			_, cmd = p.Update(cmd(), model.NewApp(model.DefaultOptions()))
		}
		var ids []string
		for _, action := range c.actions {
			ids = append(ids, action.ID)
		}
		want := []string{"a", "d"}
		if barrier {
			want = []string{"a", "b", "keep", "d"}
		}
		if !reflect.DeepEqual(ids, want) {
			t.Fatalf("barrier=%v: executed %v, want %v", barrier, ids, want)
		}
		p.Close()
	}
}

func TestAppleMusicDirectPlaybackReadsBackWithoutWaitingForPoll(t *testing.T) {
	for _, action := range []applemusic.Action{
		{Kind: "play", ID: "new"},
		{Kind: "play-collection", ID: "album", ResourceType: "albums", StartID: "new"},
		{Kind: "jump", ID: "new", Value: 1},
	} {
		t.Run(action.Kind, func(t *testing.T) {
			p, c := boundaryTestPage(t, 0, 2)
			c.change = func(s *applemusic.State, _ applemusic.Action) { s.Track.ID, s.QueueIndex = "new", 1 }
			cmd := p.dispatch(action)
			oldPoll := p.poll()()
			p.Update(cmd(), model.NewApp(model.DefaultOptions()))
			if !p.ready() || p.state.Track.ID != "new" || c.reads != 2 {
				t.Fatalf("playback waited for another poll or redundant pre-read: ready=%v id=%s reads=%d", p.ready(), p.state.Track.ID, c.reads)
			}
			p.Update(oldPoll, model.NewApp(model.DefaultOptions()))
			if p.state.Track.ID != "new" {
				t.Fatal("pre-completion poll overwrote the read-back")
			}
		})
	}
}

func TestAppleMusicDirectPlaybackReadbackFailureDoesNotReplay(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	c.failRead = 1
	c.change = func(s *applemusic.State, _ applemusic.Action) { s.Track.ID = "new" }
	p.Update(p.dispatch(applemusic.Action{Kind: "play", ID: "new"})(), model.NewApp(model.DefaultOptions()))
	if !p.pending || len(c.actions) != 1 {
		t.Fatal("failed read-back invented readiness or replayed the request")
	}
	p.Update(p.poll()(), model.NewApp(model.DefaultOptions()))
	if !p.ready() || len(c.actions) != 1 {
		t.Fatal("fresh poll failed to recover without resubmitting playback")
	}
}

type appleSlowReadbackClient struct {
	appleMouseClient
	budget time.Duration
}

func (c *appleSlowReadbackClient) Snapshot(ctx context.Context) (applemusic.State, error) {
	if deadline, ok := ctx.Deadline(); ok {
		c.budget = time.Until(deadline)
		<-ctx.Done()
		return applemusic.State{}, ctx.Err()
	}
	return c.state, nil
}

func TestAppleMusicSlowReadbackReleasesTheControlQueue(t *testing.T) {
	c := &appleSlowReadbackClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	first := p.dispatch(applemusic.Action{Kind: "play", ID: "song"})
	p.dispatch(applemusic.Action{Kind: "volume", Value: .5})
	_, next := p.Update(first(), model.NewApp(model.DefaultOptions()))
	if c.budget <= 0 || c.budget > 300*time.Millisecond {
		t.Fatalf("read-back has no short RPC deadline: %v", c.budget)
	}
	if next == nil || !p.pending {
		t.Fatal("read-back timeout blocked controls or invented playback success")
	}
	p.Update(next(), model.NewApp(model.DefaultOptions()))
	if len(c.actions) != 2 || c.actions[1].Kind != "volume" {
		t.Fatal("read-back failure replayed the song or lost the queued control")
	}
}

func TestAppleMusicCatalogRefreshBypassesCacheWithoutChangingViewIdentity(t *testing.T) {
	c := &appleLibraryClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{}}}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.drawer = appleLibrary
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	p.acceptCatalog(p.loadLibrary(applemusic.LibraryPlaylists)().(appleCatalogMsg))
	msg := p.refreshCatalog()().(appleCatalogMsg)
	if !c.requests[len(c.requests)-1].Refresh {
		t.Fatal("explicit refresh did not bypass backend cache")
	}
	if msg.request.Refresh || p.library.lastRequest.request.Refresh {
		t.Fatal("refresh flag leaked into view identity and future pagination")
	}
}

func TestAppleMusicAutoplayPanelShowsBackendState(t *testing.T) {
	SetLanguage("en")
	defer SetLanguage("en")
	for _, tc := range []struct {
		available, enabled bool
		want               string
	}{{true, true, "Autoplay: ON"}, {true, false, "Autoplay: enabled on next play"}, {false, false, "Autoplay: unavailable"}} {
		p := NewAppleMusicPage(nil)
		p.level, p.panel = motion.Off, T("panel.title")
		p.state = applemusic.State{Ready: true, AutoplayAvailable: tc.available, Autoplay: tc.enabled}
		frame := ansi.Strip(p.draw(120, 32, time.Unix(100, 0), 2))
		if !strings.Contains(frame, tc.want) {
			t.Fatalf("panel hid native autoplay state %q", tc.want)
		}
		p.Close()
	}
}

type appleSeekWindClient struct {
	state     applemusic.State
	snapshots []applemusic.State
	actions   []applemusic.Action
}

func (c *appleSeekWindClient) Snapshot(context.Context) (applemusic.State, error) {
	if len(c.snapshots) > 0 {
		s := c.snapshots[0]
		c.snapshots = c.snapshots[1:]
		return s, nil
	}
	return c.state, nil
}
func (*appleSeekWindClient) Catalog(context.Context, applemusic.CatalogRequest) (applemusic.CatalogPage, error) {
	return applemusic.CatalogPage{}, nil
}
func (*appleSeekWindClient) Lyrics(context.Context, applemusic.LyricsRequest) (applemusic.Lyrics, error) {
	return applemusic.Lyrics{}, nil
}
func (c *appleSeekWindClient) Do(_ context.Context, a applemusic.Action) error {
	c.actions = append(c.actions, a)
	return nil
}
func (*appleSeekWindClient) Show(context.Context) error { return nil }

func TestAppleMusicConfirmedSeekTriggersWindAndKeys(t *testing.T) {
	base := applemusic.State{
		Ready: true, Authorized: true, Full: true, Playing: true,
		Track:    applemusic.Track{ID: "track-1", Name: "Seek Track", Duration: 120},
		Position: 40,
	}
	afterFF := base
	afterFF.Position = 55
	afterRew := base
	afterRew.Position = 20

	c := &appleSeekWindClient{
		state:     base,
		snapshots: []applemusic.State{base, afterFF, afterFF, afterRew},
	}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Full
	app := model.NewApp(model.DefaultOptions())
	now := time.Now()
	p.acceptState(base, now)

	cmd := p.dispatch(applemusic.Action{Kind: "seek", ID: "track-1", Value: 55})
	msg := cmd()
	p.Update(msg, app)
	st := p.mechanics(time.Now())
	if st.Keys&deck.KeyFF == 0 || !p.reels.Winding(time.Now()) {
		t.Fatalf("confirmed forward seek did not engage KeyFF and tape wind: keys=%v", st.Keys)
	}

	c.state = afterFF
	cmd2 := p.dispatch(applemusic.Action{Kind: "seek", ID: "track-1", Value: 20})
	p.Update(cmd2(), app)
	st2 := p.mechanics(time.Now())
	if st2.Keys&deck.KeyRew == 0 {
		t.Fatalf("confirmed backward seek did not engage KeyRew: keys=%v", st2.Keys)
	}
}
