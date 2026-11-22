package applemusic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

type reply struct {
	ID     int                 `json:"id"`
	Result json.RawMessage     `json:"result"`
	Error  *struct{ Code int } `json:"error"`
}

type pipeWrite struct {
	data []byte
	done chan error
}

// pipe is CDP's NUL-delimited transport over inherited private pipes/handles.
type pipe struct {
	in      io.ReadCloser
	out     io.WriteCloser
	writes  chan pipeWrite
	done    chan struct{}
	once    sync.Once
	mu      sync.Mutex
	next    int
	pending map[int]chan reply
}

func newPipe(in io.ReadCloser, out io.WriteCloser) *pipe {
	c := &pipe{in: in, out: out, writes: make(chan pipeWrite), done: make(chan struct{}), pending: make(map[int]chan reply)}
	go c.read()
	go func() {
		for {
			select {
			case w := <-c.writes:
				_, err := c.out.Write(w.data)
				w.done <- err
				if err != nil {
					c.close()
					return
				}
			case <-c.done:
				return
			}
		}
	}()
	return c
}

func (c *pipe) read() {
	defer c.close()
	s := bufio.NewScanner(c.in)
	s.Buffer(make([]byte, 4096), 8<<20)
	s.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if n := bytes.IndexByte(data, 0); n >= 0 {
			return n + 1, data[:n], nil
		}
		if atEOF && len(data) != 0 {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return 0, nil, nil
	})
	for s.Scan() {
		var r reply
		if json.Unmarshal(s.Bytes(), &r) != nil {
			return
		}
		c.mu.Lock()
		ch := c.pending[r.ID]
		c.mu.Unlock()
		if ch != nil {
			select {
			case ch <- r:
			default:
			}
		}
	}
}

func (c *pipe) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.in.Close()
		_ = c.out.Close()
	})
}

var (
	replyChanPool = sync.Pool{New: func() any { return make(chan reply, 1) }}
	errChanPool   = sync.Pool{New: func() any { return make(chan error, 1) }}
	writeBufPool  = sync.Pool{New: func() any { return new(bytes.Buffer) }}
)

func (c *pipe) call(ctx context.Context, session, method string, params, result any) error {
	c.mu.Lock()
	c.next++
	id := c.next
	ch := replyChanPool.Get().(chan reply)
	select {
	case <-ch:
	default:
	}
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	buf := writeBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	enc := json.NewEncoder(buf)
	if err := enc.Encode(struct {
		ID        int    `json:"id"`
		Method    string `json:"method"`
		Params    any    `json:"params,omitempty"`
		SessionID string `json:"sessionId,omitempty"`
	}{id, method, params, session}); err != nil {
		writeBufPool.Put(buf)
		return err
	}
	raw := buf.Bytes()
	if n := len(raw); n > 0 && raw[n-1] == '\n' {
		raw[n-1] = 0
	} else {
		buf.WriteByte(0)
		raw = buf.Bytes()
	}
	errCh := errChanPool.Get().(chan error)
	select {
	case <-errCh:
	default:
	}
	w := pipeWrite{raw, errCh}
	select {
	case c.writes <- w:
	case <-ctx.Done():
		writeBufPool.Put(buf)
		return ctx.Err()
	case <-c.done:
		writeBufPool.Put(buf)
		return errors.New("Chrome 已关闭，请退出后重新启动 apple-music")
	}
	select {
	case err := <-w.done:
		writeBufPool.Put(buf)
		errChanPool.Put(errCh)
		if err != nil {
			return errors.New("Chrome 控制管道已断开")
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return errors.New("Chrome 控制管道已断开")
	}
	select {
	case r := <-ch:
		replyChanPool.Put(ch)
		if r.Error != nil {
			return fmt.Errorf("Chrome 协议错误 (%d)，请检查浏览器页面", r.Error.Code)
		}
		if result != nil {
			return json.Unmarshal(r.Result, result)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return errors.New("Chrome 控制管道已断开")
	}
}
