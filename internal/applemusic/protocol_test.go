package applemusic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestPipeRoutesRepliesAndIgnoresEvents(t *testing.T) {
	input, serverOut := io.Pipe()
	serverIn, output := io.Pipe()
	c := newPipe(input, output)
	defer c.close()
	defer serverIn.Close()
	defer serverOut.Close()
	go func() {
		request, _ := bufio.NewReader(serverIn).ReadBytes(0)
		var msg struct {
			ID int `json:"id"`
		}
		_ = json.Unmarshal(request[:len(request)-1], &msg)
		_, _ = serverOut.Write([]byte("{\"method\":\"Runtime.consoleAPICalled\"}\x00"))
		b, _ := json.Marshal(map[string]any{"id": msg.ID, "result": map[string]int{"answer": 42}})
		_, _ = serverOut.Write(append(b, 0))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var result struct{ Answer int }
	if err := c.call(ctx, "session", "Runtime.evaluate", nil, &result); err != nil || result.Answer != 42 {
		t.Fatalf("answer=%d, err=%v", result.Answer, err)
	}
}

func TestPipeCancellationAndEOF(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "eof"}[closed], func(t *testing.T) {
			input, writer := io.Pipe()
			c := newPipe(input, nopWriteCloser{io.Discard})
			defer c.close()
			defer writer.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			if closed {
				_ = writer.Close()
			}
			if err := c.call(ctx, "", "NeverReplies", nil, nil); err == nil {
				t.Fatal("expected cancellation or closed-pipe error")
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			if len(c.pending) != 0 {
				t.Fatal("pending request leaked")
			}
		})
	}
}

func TestPipeConcurrentRepliesCanArriveOutOfOrder(t *testing.T) {
	input, serverOut := io.Pipe()
	serverIn, output := io.Pipe()
	c := newPipe(input, output)
	defer c.close()
	defer serverIn.Close()
	defer serverOut.Close()
	go func() {
		reader := bufio.NewReader(serverIn)
		var requests []struct {
			ID     int
			Method string
		}
		for range 2 {
			data, _ := reader.ReadBytes(0)
			var request struct {
				ID     int
				Method string
			}
			_ = json.Unmarshal(data[:len(data)-1], &request)
			requests = append(requests, request)
		}
		for i := len(requests) - 1; i >= 0; i-- {
			data, _ := json.Marshal(map[string]any{"id": requests[i].ID, "result": requests[i].Method})
			_, _ = serverOut.Write(append(data, 0))
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	errors := make(chan error, 2)
	for _, method := range []string{"first", "second"} {
		go func() {
			var got string
			err := c.call(ctx, "", method, nil, &got)
			if err == nil && got != method {
				err = fmt.Errorf("%s received %s", method, got)
			}
			errors <- err
		}()
	}
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
}

func TestPipeMalformedResponseAndBlockedWriterAreCancellable(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		input, serverOut := io.Pipe()
		serverIn, output := io.Pipe()
		c := newPipe(input, output)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		if malformed {
			go func() { _, _ = bufio.NewReader(serverIn).ReadBytes(0); _, _ = serverOut.Write([]byte("not json\x00")) }()
		}
		if err := c.call(ctx, "", "test", nil, nil); err == nil {
			t.Fatal("expected a closed transport or cancellation")
		}
		cancel()
		c.close()
		serverIn.Close()
		serverOut.Close()
	}
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
