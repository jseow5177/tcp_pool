package tcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Pre-defined error
var (
	ErrConnectionClosed  = errors.New("connection closed")
	ErrConnectionTimeout = errors.New("connection timeout")
	ErrMissingHandler    = errors.New("handler not registered")
	ErrRead              = errors.New("cannot read data from tcp connection")
	ErrWrite             = errors.New("cannot write data into tcp connection")
)

const (
	prefixSize     = 4      // 4 bytes
	maxQueueLength = 10_000 // max 10,000 connection requests in queue
)

// An atomic piece of wrapper used to transmit data
// between the HTTP client and TCP server.
// Command specifies which function to call on the TCP server to handle Data.
type Packet struct {
	Command string
	Data    []byte
}

// TcpConfig is a set of configurations for a TCP connection pool
type TcpConfig struct {
	Host         string
	Port         int
	MaxIdleConns int
	MaxOpenConn  int
}

// tcpConn is a wrapper for a single tcp connection
type tcpConn struct {
	id   string       // A unique id to identify a connection
	pool *TcpConnPool // The TCP connecion pool
	conn net.Conn     // The underlying TCP connection
}

// connRequest wraps a channel to receive a connection
// and a channel to receive an error
type connRequest struct {
	connChan chan *tcpConn
	errChan  chan error
}

// TcpConnPool represents a pool of tcp connections
type TcpConnPool struct {
	host         string
	port         int
	mu           sync.Mutex          // mutex to prevent race conditions in concurrent access
	idleConns    map[string]*tcpConn // holds the idle connections
	numOpen      int                 // counter that tracks open connections
	maxOpenCount int
	maxIdleCount int
	requestChan  chan *connRequest // A queue of connection requests
}

// CreateTcpConnPool() creates a connection pool
// and starts the worker that handles connection request
func CreateTcpConnPool(cfg *TcpConfig) (*TcpConnPool, error) {
	pool := &TcpConnPool{
		host:         cfg.Host,
		port:         cfg.Port,
		idleConns:    make(map[string]*tcpConn),
		requestChan:  make(chan *connRequest, maxQueueLength),
		maxOpenCount: cfg.MaxOpenConn,
		maxIdleCount: cfg.MaxIdleConns,
	}

	go pool.handleConnectionRequest()

	return pool, nil
}

// createTcpBuffer() implements the TCP protocol used in this application
// A stream of TCP data to be sent over has two parts: a prefix and the actual data itself
// The prefix is a fixed length byte that states how much data is being transferred over (including the prefix)
func createTcpBuffer(data []byte) []byte {
	// Create a buffer with size enough to hold a prefix and actual data
	buf := make([]byte, prefixSize+len(data))

	// State the total number of bytes (including prefix) to be transferred over
	binary.BigEndian.PutUint32(buf[:prefixSize], uint32(prefixSize+len(data)))

	// Copy data into the remaining buffer
	copy(buf[prefixSize:], data[:])

	return buf
}

// Read() reads the data from the underlying TCP connection
func (c *tcpConn) Read() ([]byte, error) {
	prefix := make([]byte, prefixSize)

	// Read the prefix, which contains the length of data expected
	_, err := io.ReadFull(c.conn, prefix)
	if err != nil {
		switch {
		case errors.Is(err, io.EOF):
			return nil, ErrConnectionClosed
		default:
			return nil, err
		}
	}

	totalDataLength := binary.BigEndian.Uint32(prefix[:])

	// Buffer to store the actual data
	data := make([]byte, totalDataLength-prefixSize)

	// Read actual data without prefix
	_, err = io.ReadFull(c.conn, data)
	if err != nil {
		switch {
		case errors.Is(err, io.EOF):
			return nil, ErrConnectionClosed
		default:
			return nil, err
		}
	}

	return data, nil
}

// Write() writes data to the underlying TCP connection
func (c *tcpConn) Write(data []byte) (int, error) {
	reqBuf := createTcpBuffer(data)

	n, err := c.conn.Write(reqBuf)
	if err != nil {
		switch {
		case errors.Is(err, io.EOF):
			return 0, ErrConnectionClosed
		default:
			return 0, err
		}
	}

	return n, nil
}

// terminate() closes a connection and removes it from the idle pool
func (p *TcpConnPool) terminate(tcpConn *tcpConn) {
	tcpConn.conn.Close()
	delete(p.idleConns, tcpConn.id)
}

// SendData() sends data to the TCP connection, reads response, and releases the connection
func (p *TcpConnPool) SendData(data []byte) ([]byte, error) {
	// Get a new TCP connection
	tcpConn, err := p.get()
	if err != nil {
		return nil, err
	}

	// Write data into the underlying connection
	_, err = tcpConn.Write(data)
	if err != nil {
		p.terminate(tcpConn)
		return nil, err
	}

	// Read response data from the underlying connection
	resBuf, err := tcpConn.Read()
	if err != nil {
		p.terminate(tcpConn)
		return nil, err
	}

	// Releases the connection back to the pool
	p.release(tcpConn)

	return resBuf, nil
}

// release() attempts to return a used connection back to the pool
// It closes the connection if it can't do so
func (p *TcpConnPool) release(c *tcpConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.maxIdleCount > 0 && p.maxIdleCount > len(p.idleConns) {
		p.idleConns[c.id] = c // put into the pool
	} else {
		c.conn.Close()
		c.pool.numOpen--
	}
}

// get() retrieves a TCP connection
func (p *TcpConnPool) get() (*tcpConn, error) {
	p.mu.Lock()

	// Case 1: Gets a free connection from the pool if any
	numIdle := len(p.idleConns)
	if numIdle > 0 {
		// Loop map to get one conn
		for _, c := range p.idleConns {
			// remove from pool
			delete(p.idleConns, c.id)
			p.mu.Unlock()
			return c, nil
		}
	}

	// Case 2: Queue a connection request
	if p.maxOpenCount > 0 && p.numOpen >= p.maxOpenCount {
		// Create the request
		req := &connRequest{
			connChan: make(chan *tcpConn, 1),
			errChan:  make(chan error, 1),
		}
		// Queue the request
		p.requestChan <- req

		// Waits for either,
		// 1. Request fulfilled, or
		// 2. An error is returned
		select {
		case tcpConn := <-req.connChan:
			return tcpConn, nil
		case err := <-req.errChan:
			return nil, err
		}
	}

	// Case 3: Open a new connection
	p.numOpen++
	p.mu.Unlock()

	newTcpConn, err := p.openNewTcpConnection()
	if err != nil {
		p.mu.Lock()
		p.numOpen--
		p.mu.Unlock()
		return nil, err
	}

	return newTcpConn, nil
}

// openNewTcpConnection() creates a new TCP connection at p.host and p.port
func (p *TcpConnPool) openNewTcpConnection() (*tcpConn, error) {
	addr := fmt.Sprintf("%s:%d", p.host, p.port)

	c, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &tcpConn{
		// Use current time as random id
		id:   fmt.Sprintf("%v", time.Now().UnixNano()),
		conn: c,
		pool: p,
	}, nil
}

// handleConnectionRequest() listens to the request queue
// and attempts to fulfil any incoming requests
func (p *TcpConnPool) handleConnectionRequest() {
	for req := range p.requestChan {
		var (
			requestDone = false
			hasTimeout  = false

			// start a 3-second timeout
			timeoutChan = time.After(3 * time.Second)
		)

		for {
			if hasTimeout || requestDone {
				break
			}
			select {
			// request timeout
			case <-timeoutChan:
				hasTimeout = true
				req.errChan <- ErrConnectionTimeout
			default:
				p.mu.Lock()

				// First, we try to get an idle conn.
				// If fail, we try to open a new conn.
				// If both does not work, we try again in the next loop until timeout.
				numIdle := len(p.idleConns)
				if numIdle > 0 {
					for _, c := range p.idleConns {
						delete(p.idleConns, c.id)
						p.mu.Unlock()
						req.connChan <- c // give conn
						requestDone = true
						break
					}
				} else if p.maxOpenCount > 0 && p.numOpen < p.maxOpenCount {
					p.numOpen++
					p.mu.Unlock()

					c, err := p.openNewTcpConnection()
					// we allow retry instead of return error
					if err != nil {
						p.mu.Lock()
						p.numOpen--
						p.mu.Unlock()
					} else {
						req.connChan <- c // give conn
						requestDone = true
					}
				} else {
					p.mu.Unlock()
				}
			}
		}
	}
}
