package applemusic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCookieImportParsesSupportedFormatsWithoutWideningScope(t *testing.T) {
	now := time.Unix(1700000000, 0)
	for _, input := range []string{
		`[{"domain":".music.apple.com","hostOnly":false,"path":"/","name":"synthetic","value":"fixture-only","secure":true,"httpOnly":true,"sameSite":"no_restriction","expirationDate":1800000000}]`,
		"# Netscape HTTP Cookie File\n#HttpOnly_.music.apple.com\tTRUE\t/\tTRUE\t1800000000\tsynthetic\tfixture-only\n",
	} {
		jar, err := parseCookies([]byte(input), now)
		if err != nil || jar.Count() != 1 {
			t.Fatalf("parse failed: %v", err)
		}
		c := jar.cookies[0]
		if c.Domain != ".music.apple.com" || c.URL != "" || !c.Secure || !c.HTTPOnly || c.Expires == nil || *c.Expires != 1800000000 {
			t.Fatal("cookie scope or attributes changed")
		}
		if strings.Contains(fmt.Sprintf("%v %+v %#v", jar, jar, jar), "fixture-only") {
			t.Fatal("jar formatting exposes values")
		}
	}
	jar, err := parseCookies([]byte(`[{"domain":"music.apple.com","hostOnly":true,"path":"/account","name":"host","value":"fixture-only","secure":true,"session":true}]`), now)
	if err != nil {
		t.Fatal(err)
	}
	if c := jar.cookies[0]; c.Domain != "" || c.URL != "https://music.apple.com/account" || c.Expires != nil {
		t.Fatal("host-only or session cookie widened")
	}
}

func TestCookieImportRejectsUnsafeOrAmbiguousInput(t *testing.T) {
	cases := []string{
		`[]`, `not-a-cookie-file`, `[{"domain":"music.apple.com.evil.test","name":"a","value":"secret-fixture"}]`,
		`[{"domain":".apple.com.evil.test","name":"a","value":"secret-fixture"}]`,
		`[{"domain":".icloud.com","name":"a","value":"secret-fixture"}]`,
		`[{"domain":"apple.com","hostOnly":true,"name":"a","value":"secret-fixture"}]`,
		`[{"domain":"music.apple.com","name":"bad;name","value":"secret-fixture"}]`,
		`[{"domain":"music.apple.com","name":"a","value":"secret-fixture\n"}]`,
		`[{"domain":"music.apple.com","name":"a","value":"secret-fixture","sameSite":"None","secure":false}]`,
		`[{"domain":"music.apple.com","name":"a","value":"secret-fixture","sameSite":"invalid"}]`,
		`[{"domain":"music.apple.com","name":"a","value":"secret-fixture","partitionKey":{"topLevelSite":"https://music.apple.com"}}]`,
		`[{"domain":"music.apple.com","name":"a","value":"secret-fixture","url":"https://evil.test/"}]`,
		`[{"domain":"music.apple.com","name":"a","value":"secret-fixture","expires":1800000000,"expirationDate":1900000000}]`,
		`[{"domain":"music.apple.com","name":"a","value":"secret-fixture"},{"domain":"music.apple.com","name":"a","value":"different"}]`,
		"music.apple.com\tMAYBE\t/\tTRUE\t1800000000\ta\tsecret-fixture\n",
		`[{"domain":"music.apple.com","name":"__Host-a","value":"secret-fixture","secure":false}]`,
		`[{"domain":".music.apple.com","name":"__Host-a","value":"secret-fixture","secure":true}]`,
		`[{"domain":"music.apple.com","path":"not-absolute","name":"a","value":"secret-fixture"}]`,
	}
	for i, input := range cases {
		if _, err := parseCookies([]byte(input), time.Unix(1700000000, 0)); err == nil {
			t.Errorf("unsafe case %d accepted", i)
		} else if strings.Contains(err.Error(), "secret-fixture") || strings.Contains(err.Error(), "evil.test") {
			t.Errorf("case %d leaked input", i)
		}
	}
	if _, err := parseCookies([]byte(strings.Repeat("x", maxCookieFileBytes+1)), time.Now()); err == nil {
		t.Fatal("oversized input accepted")
	}
	if _, err := parseCookies([]byte(`[{"domain":"music.apple.com","name":"a","value":"`+strings.Repeat("x", 4097)+`"}]`), time.Now()); err == nil {
		t.Fatal("oversized cookie accepted")
	}
}

func TestCookieImportFiltersExpiredAndBoundsCount(t *testing.T) {
	input := `[{"domain":"music.apple.com","name":"expired","value":"fixture","expires":1600000000},{"domain":".apple.com","hostOnly":false,"name":"valid","value":"fixture","secure":true,"expires":1800000000}]`
	jar, err := parseCookies([]byte(input), time.Unix(1700000000, 0))
	if err != nil || jar.Count() != 1 || jar.Expired() != 1 {
		t.Fatalf("expiration filtering: %v", err)
	}
	var entries []string
	for i := 0; i < 257; i++ {
		entries = append(entries, fmt.Sprintf(`{"domain":"music.apple.com","name":"n%d","value":"fixture"}`, i))
	}
	if _, err := parseCookies([]byte("["+strings.Join(entries, ",")+"]"), time.Now()); err == nil {
		t.Fatal("unbounded cookie count accepted")
	}
	jar, err = parseCookies([]byte("music.apple.com\tFALSE\t/\tTRUE\t0\tempty\t\n"), time.Now())
	if err != nil || jar.Count() != 1 || jar.cookies[0].Expires != nil {
		t.Fatalf("empty session value rejected: %v", err)
	}
}

func TestCookieFileRequiresPrivateRegularInput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "synthetic-cookies.json")
	data := []byte(`[{"domain":"music.apple.com","name":"synthetic","value":"fixture-only","secure":true}]`)
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	secureCookieTestFile(t, file)
	jar, err := ReadCookieFile(file)
	if err != nil || jar.Count() != 1 {
		t.Fatalf("private cookie file: %v", err)
	}
	if _, err = ReadCookieFile(filepath.Dir(file)); err == nil {
		t.Fatal("directory accepted")
	}
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(data) {
		t.Fatal("source was changed")
	}
}

func TestCookieJSONZeroExpiryIsExpiredNotResurrected(t *testing.T) {
	jar, err := parseCookies([]byte(`[{"domain":"music.apple.com","name":"expired-at-epoch","value":"fixture","expires":0},{"domain":"music.apple.com","name":"session","value":"fixture","expires":-1}]`), time.Now())
	if err != nil || jar.Count() != 1 || jar.Expired() != 1 {
		t.Fatalf("JSON epoch expiry was converted to a live session: accepted=%d expired=%d err=%v", jar.Count(), jar.Expired(), err)
	}
}

func TestCookieImportRejectsControlBytesInPath(t *testing.T) {
	if _, err := parseCookies([]byte(`[{"domain":"music.apple.com","name":"synthetic","value":"fixture","path":"/bad\tpath"}]`), time.Now()); err == nil {
		t.Fatal("control byte in path accepted")
	}
}

func TestCookieImportRejectsWindowsNetworkAndDevicePaths(t *testing.T) {
	for _, path := range []string{`\\server\share\cookies.json`, `\\?\UNC\server\share\cookies.json`, `\\.\pipe\cookies`, `//server/share/cookies.json`} {
		if localCookiePath(path) {
			t.Fatal("network/device cookie path accepted")
		}
	}
	for _, path := range []string{`C:\private\cookies.json`, `cookies.json`} {
		if !localCookiePath(path) {
			t.Fatal("ordinary local cookie path rejected")
		}
	}
}
