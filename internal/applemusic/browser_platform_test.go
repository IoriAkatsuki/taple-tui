package applemusic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestChromeArgumentsKeepPrivateHeadlessRuntime(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "isolated profile")
	for _, background := range []bool{false, true} {
		args := chromeArgs(profile, background)
		for _, required := range []string{"--remote-debugging-pipe", "--user-data-dir=" + profile, "https://music.apple.com/"} {
			if !slices.Contains(args, required) {
				t.Fatalf("missing argument %q", required)
			}
		}
		for _, arg := range args {
			if strings.HasPrefix(arg, "--remote-debugging-port") || arg == "--no-sandbox" || arg == "--mute-audio" || arg == "--disable-gpu" {
				t.Fatalf("runtime must retain private pipes, sandbox, audio and GPU: %q", arg)
			}
		}
		if slices.Contains(args, "--headless=new") != background {
			t.Fatal("playback must be headless; only explicit login can be visible")
		}
		if slices.Contains(args, "--new-window") == background || slices.Contains(args, "--start-minimized") || slices.Contains(args, "--ozone-platform=headless") {
			t.Fatal("background playback must not create a minimized/virtual desktop window")
		}
	}
}

func TestBundledRuntimeLayout(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "install with spaces", "taple")
	for _, goos := range []string{"linux", "windows", "darwin"} {
		for _, candidate := range bundledBrowserCandidates(executable, goos) {
			root := filepath.Join(filepath.Dir(executable), "runtime", "chromium")
			if !filepath.IsAbs(candidate) || !strings.HasPrefix(candidate, root+string(filepath.Separator)) {
				t.Fatalf("%s candidate escaped the installation runtime: %q", goos, candidate)
			}
		}
	}
}

func TestRuntimeSelectionAndExplicitFailure(t *testing.T) {
	// The current test binary is executable on all supported platforms.
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing-runtime")
	for _, override := range []string{"", binary} {
		got, err := resolveBrowser(override, []string{missing, binary})
		if err != nil || got != binary {
			t.Fatalf("runtime resolution: %q %v", got, err)
		}
	}
	if _, err := resolveBrowser(missing, []string{binary}); err == nil {
		t.Fatal("bad override silently fell back to another browser")
	}
	if _, err := resolveBrowser("", []string{missing}); err == nil {
		t.Fatal("missing runtime was accepted")
	}
	preferred := filepath.Join(t.TempDir(), "bundled-runtime.exe")
	if err := os.WriteFile(preferred, nil, 0700); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveBrowser("", []string{preferred, binary}); err != nil || got != preferred {
		t.Fatalf("bundled runtime was not preferred: %q %v", got, err)
	}
	if got, err := resolveBrowser(binary, []string{preferred}); err != nil || got != binary {
		t.Fatalf("explicit runtime was not preferred: %q %v", got, err)
	}
}

func TestFindChromeUsesTapleBrowserOverride(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAPLE_BROWSER", binary)
	t.Setenv("TAPEFOX_BROWSER", filepath.Join(t.TempDir(), "missing-legacy-runtime"))
	got, err := findChrome()
	if err != nil || got != binary {
		t.Fatalf("TAPLE_BROWSER was not selected: got=%q error=%v", got, err)
	}
}

func TestFindChromeIgnoresLegacyBrowserOverride(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAPLE_BROWSER", "")
	t.Setenv("TAPEFOX_BROWSER", binary)
	t.Setenv("PATH", t.TempDir())
	// Discovery may fail or find a native installation, but never the legacy override.
	if got, err := findChrome(); got == binary {
		t.Fatalf("legacy browser override was still read: got=%q error=%v", got, err)
	}
}

func TestFindChromeRejectsInvalidTapleBrowserOverride(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAPLE_BROWSER", filepath.Join(t.TempDir(), "missing-runtime"))
	t.Setenv("TAPEFOX_BROWSER", binary)
	if got, err := findChrome(); err == nil || got != "" || !strings.Contains(err.Error(), "TAPLE_BROWSER") {
		t.Fatalf("invalid TAPLE_BROWSER silently fell back: got=%q error=%v", got, err)
	}
}

func TestHeadlessOpenDoesNotRequireDesktop(t *testing.T) {
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("TAPLE_BROWSER", filepath.Join(t.TempDir(), "missing-runtime"))
	// Discovery must be reached even on Linux without a desktop. No process starts.
	if _, err := Open(context.Background(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "TAPLE_BROWSER") {
		t.Fatalf("headless startup was gated on desktop availability: %v", err)
	}
}

func TestDisplayRequiredOnlyForExplicitLinuxLogin(t *testing.T) {
	if err := requireLoginDisplay("linux", "", ""); err == nil {
		t.Fatal("graphical login cannot work without a display")
	}
	for _, tc := range [][3]string{{"linux", ":0", ""}, {"linux", "", "wayland-0"}, {"darwin", "", ""}, {"windows", "", ""}} {
		if err := requireLoginDisplay(tc[0], tc[1], tc[2]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestChromeDiscoveryIncludesNativeInstallations(t *testing.T) {
	candidates := chromeCandidates()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", t.TempDir())
		t.Setenv("PROGRAMFILES", t.TempDir())
		t.Setenv("PROGRAMFILES(X86)", t.TempDir())
		candidates = chromeCandidates()
		for _, env := range []string{"LOCALAPPDATA", "PROGRAMFILES", "PROGRAMFILES(X86)"} {
			if !slices.Contains(candidates, filepath.Join(os.Getenv(env), "Google", "Chrome", "Application", "chrome.exe")) {
				t.Fatalf("missing installation root %s", env)
			}
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		for _, root := range []string{"/Applications", filepath.Join(home, "Applications")} {
			if !slices.Contains(candidates, filepath.Join(root, "Google Chrome.app", "Contents", "MacOS", "Google Chrome")) {
				t.Fatal("missing native Chrome app bundle")
			}
		}
	case "linux":
		if !slices.Contains(candidates, "google-chrome-stable") || !slices.Contains(candidates, "chromium") {
			t.Fatal("existing Linux discovery changed")
		}
	}
}

func TestPrepareProfileRejectsLinkedDirectory(t *testing.T) {
	parent := t.TempDir()
	target, link := filepath.Join(parent, "other-browser"), filepath.Join(parent, "chrome")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	makeProfileTestLink(t, target, link)
	if _, err := prepareProfile(link); err == nil {
		t.Fatal("linked profile was accepted")
	}
}

func TestChromeStartupFailureReturnsWithoutKeepingBrowser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// The Go test executable exits on Chrome's unrecognized switches.
	browser, err := launchBrowser(ctx, os.Args[0], filepath.Join(t.TempDir(), "chrome"), true)
	if err == nil || browser != nil || !strings.Contains(err.Error(), "Chrome 未能启动") {
		if browser != nil {
			browser.Close()
		}
		t.Fatal("startup did not exercise detection of an exited browser:", err)
	}
	if ctx.Err() != nil {
		t.Fatal("startup did not notice the child exit")
	}
}

func TestChromeFailedStartClosesParentPipes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux fd accounting; Windows has a native inherited-handle cleanup test")
	}
	parent := t.TempDir()
	// Initialize Go's lazy poll descriptors before comparing pipe fd counts.
	_, _ = launchBrowser(context.Background(), filepath.Join(parent, "missing-chrome"), filepath.Join(parent, "profile"), true)
	before, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, err := launchBrowser(context.Background(), filepath.Join(parent, "missing-chrome"), filepath.Join(parent, "profile"), true); err == nil {
			t.Fatal("missing Chrome unexpectedly started")
		}
	}
	after, err := os.ReadDir("/proc/self/fd")
	if err != nil || len(after) > len(before) {
		t.Fatalf("startup leaked pipe fds: before=%d after=%d error=%v", len(before), len(after), err)
	}
}

// This test never opens Apple Music or an existing browser profile.
func TestChromePrivatePipeSmoke(t *testing.T) {
	if os.Getenv("TAPLE_CHROME_SMOKE") != "1" {
		t.Skip("set TAPLE_CHROME_SMOKE=1 for a disposable, account-free native Chrome smoke test")
	}
	binary, err := findChrome()
	if err != nil {
		t.Fatal(err)
	}
	logChromeSmokeBinary(t, binary)
	profile := filepath.Join(t.TempDir(), "chrome")
	identity, err := prepareProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		info, err := prepareProfile(profile)
		if err != nil || !os.SameFile(identity, info) {
			t.Fatal("profile identity changed across restart:", err)
		}
		marker := filepath.Join(profile, "taple-synthetic-session")
		if err := os.WriteFile(marker, []byte("synthetic; no credentials"), 0600); err != nil {
			t.Fatal(err)
		}
		chromePrivatePipeSmokeRound(t, binary, profile)
		if _, err := os.Stat(marker); err != nil {
			t.Fatal("ordinary browser close removed session data:", err)
		}
		if err := clearOwnedProfile(profile, identity); err != nil {
			t.Fatal("closed Chrome retained profile locks or changed directory identity:", err)
		}
		entries, err := os.ReadDir(profile)
		if err != nil || len(entries) != 0 {
			t.Fatal("synthetic profile was not cleared:", err)
		}
		assertProfilePrivate(t, profile)
	}
	t.Log("Headless Chromium private pipe, own-process exit, private profile cleanup and restart verified; no account or playback was tested")
}

func chromeSmokeArgs(profile string, mediaProbe bool) []string {
	args := chromeArgs(profile, true)
	args[len(args)-1] = "about:blank"
	if !mediaProbe {
		args = append(args, "--disable-background-networking", "--disable-component-update")
	}
	// On Windows/macOS, even a bundled CDM is registered via the component
	// updater. Disabling it would turn the media probe into a false negative.
	return args
}

func TestMediaSmokeKeepsComponentRegistrationEnabled(t *testing.T) {
	for _, mediaProbe := range []bool{false, true} {
		args := chromeSmokeArgs(t.TempDir(), mediaProbe)
		for _, flag := range []string{"--disable-background-networking", "--disable-component-update"} {
			if slices.Contains(args, flag) == mediaProbe {
				t.Fatalf("mediaProbe=%v: incorrect %s setting", mediaProbe, flag)
			}
		}
		if !slices.Contains(args, "--headless=new") || !slices.Contains(args, "about:blank") || slices.Contains(args, "https://music.apple.com/") {
			t.Fatal("smoke test must stay headless and account-free")
		}
	}
}

func chromePrivatePipeSmokeRound(t *testing.T, binary, profile string) {
	t.Helper()
	childRead, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer childRead.Close()
	defer write.Close()
	read, childWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer childWrite.Close()
	// A native CI VM may spend tens of seconds on first process startup.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	mediaProbe := os.Getenv("TAPLE_BROWSER_CAPABILITIES") == "1" || os.Getenv("TAPLE_REQUIRE_WIDEVINE") == "1"
	args := chromeSmokeArgs(profile, mediaProbe)
	cmd := exec.CommandContext(ctx, binary, args...)
	stderr := &chromeSmokeOutput{}
	cmd.Stderr = stderr
	cmd.WaitDelay = 2 * time.Second
	rpc := &chromeSmokeRPC{t: t, stage: "startChrome"}
	t.Logf("Chrome start: binary=%q mediaProbe=%t sandbox=enabled profile=disposable", binary, mediaProbe)
	if err := startChrome(cmd, childRead, childWrite); err != nil {
		t.Fatalf("Chrome start: %v; stderr=%q", err, stderr.text(profile))
	}
	childRead.Close()
	childWrite.Close()
	exited := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(exited) }()
	defer func() {
		observedExit := false
		var killErr error
		select {
		case <-exited:
			observedExit = true
		default:
			killErr = cmd.Process.Kill()
			<-exited
		}
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		t.Logf("Chrome process: pid=%d last_stage=%s exit_observed_before_cleanup=%t exit_code=%d wait=%v cleanup_kill=%v context=%v", cmd.Process.Pid, rpc.stage, observedExit, exitCode, waitErr, killErr, ctx.Err())
		t.Logf("Chrome stderr (tail, <=8192 bytes, paths/URLs/sensitive lines redacted): %q", stderr.text(profile))
	}()
	pipe := newPipe(read, write)
	defer pipe.close()
	rpc.pipe = pipe
	var version struct{ Product, UserAgent string }
	if err := rpc.call(ctx, "", "Browser.getVersion", nil, &version); err != nil {
		t.Fatal(err)
	}
	if version.Product == "" {
		t.Fatal("Chrome did not report a version over the private pipe")
	}
	t.Logf("Chrome CDP version: product=%q", version.Product)
	if !strings.Contains(version.UserAgent, "HeadlessChrome/") {
		t.Fatalf("runtime did not report headless mode: %s", version.Product)
	}
	if mediaProbe {
		probeBrowserCapabilities(t, ctx, rpc)
	}
	// Browser.close can close the pipe before its acknowledgement is delivered.
	_ = rpc.call(ctx, "", "Browser.close", nil, nil)
	select {
	case <-exited:
	case <-ctx.Done():
		t.Fatal("own Chrome did not exit after Browser.close")
	}
}

// Local, account-free EME negotiation, NOT a license or playback test.
// Loopback is a potentially trustworthy origin, as required by EME.
func probeBrowserCapabilities(t *testing.T, ctx context.Context, p *chromeSmokeRPC) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><title>Local media capability probe</title>"))
	}))
	defer server.Close()
	var target struct{ TargetID string }
	if err := p.call(ctx, "", "Target.createTarget", map[string]string{"url": server.URL}, &target); err != nil {
		t.Fatal(err)
	}
	var attached struct{ SessionID string }
	if err := p.call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true}, &attached); err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result struct {
			Value struct {
				Secure   bool   `json:"secure"`
				AAC      string `json:"aac"`
				Widevine bool   `json:"widevine"`
				Reason   string `json:"reason"`
			}
		}
		ExceptionDetails any
	}
	const expression = `(async () => {
  if (location.protocol !== 'http:') return {secure:false, aac:'', widevine:false, reason:'loading'};
  const aac = document.createElement('audio').canPlayType('audio/mp4; codecs="mp4a.40.2"');
  const result = {secure:isSecureContext, aac, widevine:false, reason:''};
  if (!navigator.requestMediaKeySystemAccess) { result.reason='EME unavailable'; return result; }
  let timer;
  try {
    await Promise.race([
      navigator.requestMediaKeySystemAccess('com.widevine.alpha', [{
        initDataTypes:['cenc'], audioCapabilities:[{contentType:'audio/mp4; codecs="mp4a.40.2"'}],
        distinctiveIdentifier:'optional', persistentState:'optional', sessionTypes:['temporary']
      }]),
      new Promise((_, reject) => { timer=setTimeout(() => reject({name:'Timeout'}), 5000); })
    ]);
    result.widevine=true;
  } catch (error) { result.reason=String(error.name || 'Unavailable'); }
  finally { clearTimeout(timer); }
  return result;
})()`
	// A fresh target can still be at about:blank, and bundled CDM registration
	// is asynchronous. Retry startup NotSupportedError briefly, not indefinitely.
	registrationDeadline := time.Now().Add(3 * time.Second)
	for {
		if err := p.call(ctx, attached.SessionID, "Runtime.evaluate", map[string]any{"expression": expression, "awaitPromise": true, "returnByValue": true, "timeout": 7000}, &response); err != nil {
			t.Fatal(err)
		}
		if response.ExceptionDetails != nil {
			t.Fatal("local capability probe failed")
		}
		reason := response.Result.Value.Reason
		if reason != "loading" && (reason != "NotSupportedError" || !time.Now().Before(registrationDeadline)) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	caps := response.Result.Value
	if !caps.Secure {
		t.Fatal("local capability probe did not reach a trustworthy origin")
	}
	t.Logf("headless media capability only: AAC=%q Widevine=%t reason=%q (not Apple Music playback)", caps.AAC, caps.Widevine, caps.Reason)
	if os.Getenv("TAPLE_REQUIRE_WIDEVINE") == "1" && (caps.AAC == "" || !caps.Widevine) {
		t.Fatal("selected runtime cannot negotiate AAC + Widevine in headless mode")
	}
}

// Test-only diagnostics: no production profile, credentials or RPC payloads.
const chromeSmokeOutputLimit = 8192

type chromeSmokeOutput struct {
	mu   sync.Mutex
	tail []byte
}

func (o *chromeSmokeOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := len(p)
	if n >= chromeSmokeOutputLimit {
		o.tail = append(o.tail[:0], p[n-chromeSmokeOutputLimit:]...)
	} else {
		if keep := chromeSmokeOutputLimit - n; len(o.tail) > keep {
			copy(o.tail, o.tail[len(o.tail)-keep:])
			o.tail = o.tail[:keep]
		}
		o.tail = append(o.tail, p...)
	}
	return n, nil
}

var chromeSmokeURL = regexp.MustCompile(`(?i)(?:https?|wss?)://[^\s<>"']+`)
var chromeSmokeEmail = regexp.MustCompile(`(?i)[a-z0-9_.+%-]+@[a-z0-9.-]+\.[a-z]{2,}`)
var chromeSmokeSensitive = regexp.MustCompile(`(?i)cookie|authorization|password|passwd|token|secret|bearer|basic\s`)

func (o *chromeSmokeOutput) text(profile string) string {
	o.mu.Lock()
	text := string(o.tail)
	o.mu.Unlock()
	if profile != "" {
		text = strings.ReplaceAll(text, profile, "<profile>")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		text = strings.ReplaceAll(text, home, "<home>")
	}
	text = chromeSmokeURL.ReplaceAllString(text, "<url>")
	text = chromeSmokeEmail.ReplaceAllString(text, "<email>")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if chromeSmokeSensitive.MatchString(line) {
			lines[i] = "<sensitive line redacted>"
		}
	}
	text = strings.Join(lines, "\n")
	// Logs are quoted as well, but remove terminal controls before reporting.
	text = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' || r == 127 {
			return -1
		}
		return r
	}, text)
	if len(text) > chromeSmokeOutputLimit {
		text = text[len(text)-chromeSmokeOutputLimit:]
	}
	return text
}

func logChromeSmokeBinary(t *testing.T, binary string) {
	t.Helper()
	real, err := filepath.EvalSymlinks(binary)
	t.Logf("Chrome selection: binary=%q resolved=%q resolve_error=%v override=%t", binary, real, err, os.Getenv("TAPLE_BROWSER") != "")
	for _, candidate := range chromeCandidates() {
		if path, err := exec.LookPath(candidate); err == nil {
			t.Logf("Chrome candidate: name=%q path=%q", candidate, path)
		}
	}
	if f, err := os.Open(binary); err == nil {
		var prefix [2]byte
		_, readErr := f.Read(prefix[:])
		_ = f.Close()
		t.Logf("Chrome selected binary: script_wrapper=%t header_error=%v", string(prefix[:]) == "#!", readErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output := &chromeSmokeOutput{}
	cmd := exec.CommandContext(ctx, binary, "--version")
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = output, output, time.Second
	err = cmd.Run()
	t.Logf("Chrome executable version: output=%q error=%v context=%v", output.text(""), err, ctx.Err())
}

type chromeSmokeRPC struct {
	t     *testing.T
	pipe  *pipe
	stage string
}

func (c *chromeSmokeRPC) call(ctx context.Context, session, method string, params, result any) error {
	c.t.Helper()
	c.stage = method
	start := time.Now()
	c.t.Log("Chrome RPC begin:", method)
	err := c.pipe.call(ctx, session, method, params, result)
	c.t.Logf("Chrome RPC end: method=%s elapsed=%s error=%v context=%v", method, time.Since(start).Round(time.Millisecond), err, ctx.Err())
	return err
}

func TestChromeSmokeDiagnosticOutput(t *testing.T) {
	var output chromeSmokeOutput
	_, _ = output.Write([]byte(strings.Repeat("x", chromeSmokeOutputLimit*2)))
	_, _ = output.Write([]byte("\nRPC failed /synthetic/profile https://example.invalid/path?private=synthetic user@example.invalid\nauthorization: synthetic-private-value\n\x1bChrome crashed\n"))
	text := output.text("/synthetic/profile")
	if len(text) > chromeSmokeOutputLimit || len(output.tail) > chromeSmokeOutputLimit {
		t.Fatal("diagnostic output exceeded its bound")
	}
	for _, forbidden := range []string{"/synthetic/profile", "example.invalid", "synthetic-private-value", "\x1b"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("diagnostic output retained %q", forbidden)
		}
	}
	for _, expected := range []string{"<profile>", "<url>", "<email>", "Chrome crashed"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("diagnostic output lost %q", expected)
		}
	}
}
