// Package widgetcache is a minimal RESP-2 Redis client for the Sprint 1.5a
// widget summaries (GET/SET with TTL). Like provisioning's queue client it
// exists to avoid a driver dependency; it only covers what the widgets need.
package widgetcache

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Cache is a single-connection Redis client. Callers must serialise use
// (the widget service holds one mutex-protected instance per process).
type Cache struct {
	conn net.Conn
	r    *bufio.Reader
}

// New dials Redis, authenticates when password is set, and verifies PING.
func New(addr, password string) (*Cache, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial redis %s: %w", addr, err)
	}
	c := &Cache{conn: conn, r: bufio.NewReader(conn)}
	if password != "" {
		if _, err := c.Do("AUTH", password); err != nil {
			conn.Close()
			return nil, fmt.Errorf("redis auth: %w", err)
		}
	}
	if _, err := c.Do("PING"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return c, nil
}

// Close releases the connection.
func (c *Cache) Close() { _ = c.conn.Close() }

// Get returns the value for key, or "" with a nil error on miss.
func (c *Cache) Get(key string) (string, error) {
	reply, err := c.Do("GET", key)
	if err != nil {
		return "", err
	}
	s, _ := reply.(string)
	return s, nil
}

// Set stores value under key with the given TTL (0 = no expiry).
func (c *Cache) Set(key, value string, ttl time.Duration) error {
	args := []string{"SET", key, value}
	if ttl > 0 {
		args = append(args, "EX", strconv.Itoa(int(ttl.Seconds())))
	}
	_, err := c.Do(args...)
	return err
}

// Do sends one command and returns the decoded reply: bulk string → string
// (nil → nil), integer → int64, simple string → string, error → error.
func (c *Cache) Do(args ...string) (interface{}, error) {
	var out strings.Builder
	out.WriteString("*" + strconv.Itoa(len(args)) + "\r\n")
	for _, a := range args {
		out.WriteString("$" + strconv.Itoa(len(a)) + "\r\n" + a + "\r\n")
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.conn.Write([]byte(out.String())); err != nil {
		return nil, err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	return c.readReply()
}

func (c *Cache) readLine() ([]byte, error) {
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	return line[:len(line)-2], nil // strip CRLF
}

func (c *Cache) readReply() (interface{}, error) {
	line, err := c.readLine()
	if err != nil {
		return nil, err
	}
	if len(line) < 1 {
		return nil, errors.New("redis: empty reply")
	}
	switch line[0] {
	case '+':
		return string(line[1:]), nil
	case '-':
		return nil, errors.New(string(line[1:]))
	case ':':
		n, err := strconv.ParseInt(string(line[1:]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("redis int: %w", err)
		}
		return n, nil
	case '$':
		n, err := strconv.Atoi(string(line[1:]))
		if err != nil {
			return nil, fmt.Errorf("redis bulk: %w", err)
		}
		if n < 0 {
			return nil, nil // miss
		}
		buf := make([]byte, n+2) // payload + CRLF
		if _, err := ioReadFull(c.r, buf); err != nil {
			return nil, err
		}
		return string(buf[:n]), nil
	default:
		return nil, fmt.Errorf("redis: unexpected reply %q", line)
	}
}

func ioReadFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
