package provisioning

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// queueKey is the Redis list holding pending provisioning jobs. Jobs are
// pushed with LPUSH and consumed with BRPOP (FIFO).
const queueKey = "provisioning:jobs"

// JobPayload is the message body enqueued on Redis. JobID links the message
// back to its provisioning_jobs row; UID is the Authentik uid (sha256 hex)
// used as the Nextcloud userid and inside the Odoo auto-created username.
// UID/Name/UserEmail are re-resolved from the Authentik API when empty, so
// recovery-swept jobs do not have to carry them.
type JobPayload struct {
	JobID     string `json:"job_id"`
	UserID    int    `json:"user_id"`
	UID       string `json:"uid,omitempty"`
	Name      string `json:"name,omitempty"`
	UserEmail string `json:"user_email,omitempty"`
	Action    string `json:"action"` // provision (deprovision ships in 1.4b)
	Service   string `json:"service"`
}

// Queue is a durable-ish enqueue/pop client over a single Redis connection.
// Reconnects happen lazily on the next operation after a failure.
type Queue struct {
	addr     string
	password string

	mu   sync.Mutex
	conn *redisConn
}

// NewQueue returns a Queue bound to addr (host:port). Connection is lazy.
func NewQueue(addr, password string) *Queue {
	return &Queue{addr: addr, password: password}
}

// client returns a healthy connection, redialing when needed.
func (q *Queue) client() (*redisConn, error) {
	if q.conn != nil {
		if _, err := q.conn.do("PING"); err == nil {
			return q.conn, nil
		}
		q.conn.Close()
		q.conn = nil
	}
	c, err := dialRedis(q.addr, q.password)
	if err != nil {
		return nil, err
	}
	q.conn = c
	return c, nil
}

// Enqueue pushes a job payload onto the queue (LPUSH, FIFO with BRPOP).
func (q *Queue) Enqueue(ctx context.Context, p JobPayload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal job payload: %w", err)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	c, err := q.client()
	if err != nil {
		return err
	}
	if _, err := c.do("LPUSH", queueKey, string(body)); err != nil {
		return fmt.Errorf("redis lpush: %w", err)
	}
	return nil
}

// Pop blocks up to timeout waiting for the next job (BRPOP). Returns
// (nil, nil) on timeout. Cancelling ctx closes the underlying connection,
// which unblocks a pending BRPOP — used for graceful shutdown.
func (q *Queue) Pop(ctx context.Context, timeout time.Duration) (*JobPayload, error) {
	q.mu.Lock()
	c, err := q.client()
	if err != nil {
		q.mu.Unlock()
		return nil, err
	}
	// Unblock BRPOP when ctx is cancelled (shutdown).
	done := make(chan struct{})
	stop := ctx.Done()
	go func() {
		select {
		case <-stop:
			c.Close()
		case <-done:
		}
	}()
	_ = c.conn.SetDeadline(time.Now().Add(timeout + 10*time.Second))
	reply, err := c.do("BRPOP", queueKey, fmt.Sprintf("%d", int(timeout.Seconds())))
	close(done)
	if err != nil {
		// Connection-level failure (including shutdown close): drop conn.
		c.Close()
		q.conn = nil
		if ctx.Err() != nil {
			q.mu.Unlock()
			return nil, ctx.Err()
		}
		q.mu.Unlock()
		return nil, err
	}
	q.mu.Unlock()

	if reply == nil {
		return nil, nil
	}
	arr, ok := reply.([]interface{})
	if !ok || len(arr) != 2 {
		return nil, fmt.Errorf("redis brpop: unexpected reply shape")
	}
	body, ok := arr[1].(string)
	if !ok {
		return nil, fmt.Errorf("redis brpop: payload is not a string")
	}
	var p JobPayload
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		return nil, fmt.Errorf("decode job payload: %w", err)
	}
	return &p, nil
}

// Close releases the connection, if any.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.conn != nil {
		q.conn.Close()
		q.conn = nil
	}
}
