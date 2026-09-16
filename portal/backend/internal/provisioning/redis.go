// Package provisioning implements the user provisioning pipeline (Sprint 1.4):
// an admin creates a user in Authentik via the portal, the portal persists a
// provisioning job per service (mail/Nextcloud/Odoo) and enqueues it on Redis,
// and a worker process executes the jobs through service-specific connectors.
//
// redis.go is a minimal RESP-2 client covering exactly what the queue needs
// (AUTH, PING, LPUSH, BRPOP). It exists to avoid adding a Redis driver to
// go.mod for a single-list queue; it is NOT a general-purpose Redis client.
package provisioning

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"time"
)

// redisConn is a single Redis connection. It is NOT safe for concurrent use;
// Queue serializes access.
type redisConn struct {
	conn net.Conn
	r    *bufio.Reader
}

// dialRedis opens a connection, authenticates when password is non-empty and
// verifies the server answers PING.
func dialRedis(addr, password string) (*redisConn, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial redis %s: %w", addr, err)
	}
	c := &redisConn{conn: conn, r: bufio.NewReader(conn)}
	if password != "" {
		if _, err := c.do("AUTH", password); err != nil {
			conn.Close()
			return nil, fmt.Errorf("redis auth: %w", err)
		}
	}
	if _, err := c.do("PING"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return c, nil
}

func (c *redisConn) Close() { _ = c.conn.Close() }

// do sends a command and returns the reply. Bulk strings come back as string
// (or nil for $-1), integers as int64, errors as error, arrays as []interface{}.
func (c *redisConn) do(args ...string) (interface{}, error) {
	if err := c.writeCommand(args); err != nil {
		return nil, err
	}
	return c.readReply()
}

func (c *redisConn) writeCommand(args []string) error {
	out := "*" + strconv.Itoa(len(args)) + "\r\n"
	for _, a := range args {
		out += "$" + strconv.Itoa(len(a)) + "\r\n" + a + "\r\n"
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.conn.Write([]byte(out))
	return err
}

func (c *redisConn) readReply() (interface{}, error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	line, err := c.readLine()
	if err != nil {
		return nil, err
	}
	if len(line) < 1 {
		return nil, fmt.Errorf("redis: empty reply")
	}
	switch line[0] {
	case '+':
		return line[1:], nil
	case '-':
		return nil, fmt.Errorf("redis: %s", line[1:])
	case ':':
		n, err := strconv.ParseInt(line[1:], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("redis: bad integer %q", line)
		}
		return n, nil
	case '$':
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return nil, fmt.Errorf("redis: bad bulk length %q", line)
		}
		if n < 0 {
			return nil, nil // RESP null bulk ($-1), e.g. BRPOP timeout
		}
		buf := make([]byte, n+2) // payload + CRLF
		if _, err := readFull(c.r, buf); err != nil {
			return nil, err
		}
		return string(buf[:n]), nil
	case '*':
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return nil, fmt.Errorf("redis: bad array length %q", line)
		}
		if n < 0 {
			return nil, nil
		}
		arr := make([]interface{}, 0, n)
		for i := 0; i < n; i++ {
			v, err := c.readReply()
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("redis: unknown reply type %q", line)
	}
}

func (c *redisConn) readLine() (string, error) {
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return trimCRLF(line), nil
	}
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func trimCRLF(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
