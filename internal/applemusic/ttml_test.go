package applemusic

import (
	"strings"
	"testing"
	"time"
)

func TestAppleTTMLPreservesRealLineTimesAndMixedText(t *testing.T) {
	xml := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:itunes="urn:fixture" itunes:timing="Word" xml:lang="zh-Hans">
<head><metadata><p>not a lyric</p></metadata></head><body><div begin="10s" end="20s">
<p begin="00:10.2" end="00:12.050"><span begin="10.2s" end="11s">夜行</span> &amp; <span>alpha</span><br/>beta</p>
<p begin="11.5s" dur="1500ms">和声</p></div></body></tt>`
	got, err := parseLyricsTTML(xml)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Synced || got.Language != "zh-Hans" || len(got.Lines) != 2 {
		t.Fatalf("bad metadata: %+v", got)
	}
	if got.Lines[0].Text != "夜行 & alpha beta" || got.Lines[0].Start != 10200*time.Millisecond || got.Lines[0].End != 12050*time.Millisecond {
		t.Fatalf("line not preserved: %+v", got.Lines[0])
	}
	if got.Lines[1].Start != 11500*time.Millisecond || got.Lines[1].End != 13*time.Second {
		t.Fatal("overlapping timed line or explicit duration was lost")
	}
}

func TestAppleTTMLAcceptsWebsiteBareSecondTimes(t *testing.T) {
	// The website's duration parser accepts seconds without a colon or unit suffix.
	xml := `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:itunes="urn:fixture" itunes:timing="Word"><body dur="202.345"><div begin="0" end="202.345">
<p begin="12.345" end="15.678"><span begin="12.345" end="13.001">Synthetic</span> line</p>
<p begin="15.678" dur=".200">Second line</p></div></body></tt>`
	got, err := parseLyricsTTML(xml)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Synced || len(got.Lines) != 2 || got.Lines[0].Start != 12345*time.Millisecond || got.Lines[0].End != 15678*time.Millisecond || got.Lines[1].End != 15878*time.Millisecond {
		t.Fatalf("bare seconds were not preserved: %+v", got)
	}
}

func TestAppleTTMLUntimedTextNeverGetsInventedTimestamps(t *testing.T) {
	for _, xml := range []string{`<tt><body><p>plain text</p></body></tt>`, `<tt><body><p begin="1s">missing end</p></body></tt>`, `<tt xmlns:itunes="urn:fixture" itunes:timing="None"><body><p begin="1s" end="2s">plain</p></body></tt>`} {
		got, err := parseLyricsTTML(xml)
		if err != nil || got.Synced || len(got.Lines) != 1 {
			t.Fatalf("unsynced text misclassified: %+v %v", got, err)
		}
	}
}

func TestAppleTTMLEmptySeparatorsDoNotDisableTimedLyrics(t *testing.T) {
	got, err := parseLyricsTTML(`<tt><body><p/><p begin="0s" end="0s"> </p><p begin="1s" end="2s">fixture</p></body></tt>`)
	if err != nil || !got.Synced || len(got.Lines) != 1 {
		t.Fatalf("empty separator changed the real timeline: %+v %v", got, err)
	}
}

func TestAppleTTMLRejectsUnsafeAndUnsupportedDocuments(t *testing.T) {
	for _, xml := range []string{
		`<tt><body></tt>`, `<html><p>wrong root</p></html>`, `<tt/>`, `<tt><body/></tt><tt><body/></tt>`,
		`<!DOCTYPE tt [<!ENTITY x SYSTEM "file:///etc/passwd">]><tt><body><p>&x;</p></body></tt>`,
		`<tt><body><p begin="2s" end="1s">invalid interval</p></body></tt>`,
		`<tt><body><p begin="0:99" end="2s">invalid clock</p></body></tt>`,
		`<tt><body><p begin="10f" end="20f">unsupported frames</p></body></tt>`,
		`<tt><body><div timeContainer="seq"><p begin="1s" end="2s">relative sequence</p></div></body></tt>`,
		`<tt xmlns:ttp="urn:fixture" ttp:timeBase="clock"><body><p begin="1s" end="2s">wall clock</p></body></tt>`,
		"<tt><body>" + strings.Repeat("<div>", 65) + "<p>too deep</p>" + strings.Repeat("</div>", 65) + "</body></tt>",
		strings.Repeat("x", 2_000_001),
	} {
		if _, err := parseLyricsTTML(xml); err == nil {
			t.Fatalf("accepted invalid document %.80q", xml)
		}
	}
}

func TestAppleTTMLParseErrorsIdentifyTheFieldWithoutLeakingText(t *testing.T) {
	_, err := parseLyricsTTML(`<tt><body><p begin="PRIVATE_VALUE" end="2s">PRIVATE_LYRIC</p></body></tt>`)
	if err == nil || !strings.Contains(err.Error(), "行开始时间") || strings.Contains(err.Error(), "PRIVATE") {
		t.Fatalf("missing safe format diagnostic: %v", err)
	}
}

func TestLyricTimeUnits(t *testing.T) {
	for input, want := range map[string]time.Duration{"0": 0, ".2": 200 * time.Millisecond, "12.345": 12345 * time.Millisecond, "123": 123 * time.Second, ".2s": 200 * time.Millisecond, "00:00.2": 200 * time.Millisecond, "01:02.005": 62005 * time.Millisecond, "01:02:03.250": 3723250 * time.Millisecond, "1200ms": 1200 * time.Millisecond, "1.5m": 90 * time.Second, ".5h": 30 * time.Minute} {
		got, err := parseLyricTime(input)
		if err != nil || got != want {
			t.Fatalf("%s: %v, %v; want %v", input, got, err, want)
		}
	}
	for _, bad := range []string{"", ".", "NaN", "Inf", "-1", "1e5", "1.2.3", "86401", "NaNs", "-1s", "1e5s", "0:00:00:12", "1:60:00", "99999999999999999999999h", "1.2.3s"} {
		if _, err := parseLyricTime(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
